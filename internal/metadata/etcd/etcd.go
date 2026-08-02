package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdConfig configures an etcd backend client.
type EtcdConfig struct {
	Endpoints      []string
	DialTimeout    time.Duration
	RequestTimeout time.Duration
}

type EtcdClient struct {
	client *clientv3.Client
	cfg    EtcdConfig

	mu           sync.Mutex
	activeLocks  map[string]context.CancelFunc
	lastLeaseIDs map[string]clientv3.LeaseID
}

func NewEtcdClient(cfg EtcdConfig) (*EtcdClient, error) {
	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd: at least one endpoint required")
	}
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 10 * time.Second
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("etcd: create client: %w", err)
	}

	return &EtcdClient{
		client:       cli,
		cfg:          cfg,
		activeLocks:  make(map[string]context.CancelFunc),
		lastLeaseIDs: make(map[string]clientv3.LeaseID),
	}, nil
}

func lockKeyFor(volumeName string) string {
	return store.OwnerKeyspace + volumeName + "/lock"
}

// --- store.Backend implementation ---

func (c *EtcdClient) PutObject(ctx context.Context, key string, data []byte) error {
	putCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	_, err := c.client.Put(putCtx, key, string(data))
	return err
}

func (c *EtcdClient) ReadObject(ctx context.Context, key string) ([]byte, error) {
	getCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(getCtx, key)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, store.ErrKeyNotFound
	}
	return resp.Kvs[0].Value, nil
}

func (c *EtcdClient) DeleteObject(ctx context.Context, key string) error {
	delCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	_, err := c.client.Delete(delCtx, key)
	return err
}

func (c *EtcdClient) ListObjects(ctx context.Context, prefix string) ([]s3.Object, error) {
	listCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(listCtx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}

	entries := make([]s3.Object, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		entries = append(entries, s3.Object{Key: &key, ModificationCounter: &kv.ModRevision})
	}
	return entries, nil
}

func (c *EtcdClient) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	delCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	_, err := c.client.Delete(delCtx, prefix, clientv3.WithPrefix())
	return err
}

// --- store.Coordinator implementation ---

// AcquireLock implements store.Coordinator using an etcd transaction/CAS
// with a lease for TTL-based expiration. The lease is automatically kept
// alive via KeepAlive until ReleaseLock is called.
func (c *EtcdClient) AcquireLock(ctx context.Context, volumeName, ownerID string, ttlSeconds int64) (string, error) {
	lockKey := lockKeyFor(volumeName)

	entry := store.OwnerEntry{
		Name: ownerID,
		// The lease is kept alive until ReleaseLock, so the lock does not
		// expire while this process is alive; report it as permanent (0).
		// The granted TTL below still acts as a safety net if keepalive fails.
		ExpiryTime: 0,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal owner entry: %w", err)
	}

	if ttlSeconds <= 0 {
		ttlSeconds = lockTTLPermanent
	}
	leaseResp, gerr := c.client.Grant(ctx, ttlSeconds)
	if gerr != nil {
		return "", fmt.Errorf("grant lease: %w", gerr)
	}
	leaseID := leaseResp.ID

	txn := c.client.Txn(ctx).
		If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0)).
		Then(clientv3.OpPut(lockKey, string(data), clientv3.WithLease(leaseID)))

	tresp, terr := txn.Commit()
	if terr != nil {
		_, _ = c.client.Revoke(ctx, leaseID)
		return "", fmt.Errorf("cas lock: %w", terr)
	}

	if !tresp.Succeeded {
		_, _ = c.client.Revoke(ctx, leaseID)
		return "", store.ErrLockConflict
	}

	// Start automatic keepalive so the lease stays active until ReleaseLock
	kaCtx, kaCancel := context.WithCancel(context.Background())
	kaCh, kaErr := c.client.KeepAlive(kaCtx, leaseID)
	if kaErr != nil {
		kaCancel()
		_, _ = c.client.Revoke(ctx, leaseID)
		return "", fmt.Errorf("keepalive lease: %w", kaErr)
	}

	c.mu.Lock()
	c.activeLocks[lockKey] = kaCancel
	c.lastLeaseIDs[lockKey] = leaseID
	c.mu.Unlock()
	go c.monitorKeepAlive(lockKey, leaseID, kaCh)

	return lockKey, nil
}

// ReleaseLock implements store.Coordinator. It revokes the lease
// (which auto-deletes the lock key) and stops the renewal goroutine.
func (c *EtcdClient) ReleaseLock(ctx context.Context, lockKey string) error {
	c.mu.Lock()
	cancel, hasCancel := c.activeLocks[lockKey]
	leaseID, hasLease := c.lastLeaseIDs[lockKey]
	delete(c.activeLocks, lockKey)
	delete(c.lastLeaseIDs, lockKey)
	c.mu.Unlock()

	if !hasLease {
		// We do not track this lock (e.g. after a process restart or once the
		// keepalive monitor dropped it). If the key still exists, its lease
		// self-expires, so there is nothing safe for us to revoke.
		log.Debugf("release_lock_untracked", "lock=%s", lockKey)
		return nil
	}
	if hasCancel {
		cancel()
	}

	// Delete only if the key is still attached to this exact lease. This
	// prevents a stale owner from deleting a lock acquired after expiry.
	delCtx, delCancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer delCancel()
	_, err := c.client.Txn(delCtx).
		If(clientv3.Compare(clientv3.LeaseValue(lockKey), "=", int64(leaseID))).
		Then(clientv3.OpDelete(lockKey)).
		Commit()
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	// Always revoke our lease. On success it is required to free the key; if
	// the CAS failed the key was deleted concurrently and the lease may be
	// orphaned. Revoking our own lease is always safe.
	if _, rerr := c.client.Revoke(delCtx, leaseID); rerr != nil {
		log.Debugf("release_lock_revoke_failed", "lock=%s error=%v", lockKey, rerr)
	}
	return nil
}

func (c *EtcdClient) monitorKeepAlive(lockKey string, leaseID clientv3.LeaseID, ch <-chan *clientv3.LeaseKeepAliveResponse) {
	for response := range ch {
		if response != nil && response.TTL > 0 {
			continue
		}
		break
	}

	c.mu.Lock()
	if c.lastLeaseIDs[lockKey] == leaseID {
		delete(c.lastLeaseIDs, lockKey)
		delete(c.activeLocks, lockKey)
	}
	c.mu.Unlock()
}

// RenewLock implements store.Coordinator.
func (c *EtcdClient) RenewLock(ctx context.Context, lockKey string) error {
	c.mu.Lock()
	leaseID, ok := c.lastLeaseIDs[lockKey]
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active lease for lock %q", lockKey)
	}
	_, err := c.client.KeepAliveOnce(ctx, leaseID)
	return err
}

// LockIsValid implements store.Coordinator.
func (c *EtcdClient) LockIsValid(ctx context.Context, lockKey string) (bool, error) {
	getCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(getCtx, lockKey)
	if err != nil {
		return false, fmt.Errorf("check lock: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return false, nil
	}

	kv := resp.Kvs[0]
	if kv.Lease > 0 {
		leaseResp, lerr := c.client.TimeToLive(ctx, clientv3.LeaseID(kv.Lease))
		if lerr != nil {
			return false, fmt.Errorf("check lease TTL: %w", lerr)
		}
		if leaseResp.TTL <= 0 {
			return false, nil
		}
	}
	return true, nil
}

// FindLock implements store.Coordinator.
func (c *EtcdClient) FindLock(ctx context.Context, volumeName string) (string, string, int64, int64, error) {
	lockKey := lockKeyFor(volumeName)

	getCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(getCtx, lockKey)
	if err != nil {
		return "", "", 0, 0, err
	}
	if len(resp.Kvs) == 0 {
		return "", "", 0, 0, store.ErrKeyNotFound
	}

	var entry store.OwnerEntry
	if err := json.Unmarshal(resp.Kvs[0].Value, &entry); err != nil {
		return "", "", 0, 0, fmt.Errorf("unmarshal owner entry: %w", err)
	}

	return lockKey, entry.Name, 0, entry.ExpiryTime, nil
}

// ListAllLocks implements store.Coordinator.
func (c *EtcdClient) ListAllLocks(ctx context.Context) (map[string]store.VolumeOwner, error) {
	listCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(listCtx, store.OwnerKeyspace, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	result := make(map[string]store.VolumeOwner)
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		vol := parseVolumeFromLockKey(key)
		if vol == "" {
			continue
		}
		var entry store.OwnerEntry
		if jErr := json.Unmarshal(kv.Value, &entry); jErr != nil {
			continue
		}
		result[vol] = store.VolumeOwner{
			Volume: vol,
			Owner:  entry.Name,
			Expiry: entry.ExpiryTime,
		}
	}
	return result, nil
}

// parseVolumeFromLockKey extracts the volume name from an etcd lock key
// of the form "blt-volume-manager/owners/{volume}/lock".
func parseVolumeFromLockKey(key string) string {
	s := key[len(store.OwnerKeyspace):]
	idx := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			idx = i
		}
	}
	if idx < 0 {
		return ""
	}
	return s[:idx]
}

// NextVersion implements store.Coordinator using an atomic etcd CAS
// transaction. It retries up to 10 times on CAS conflicts.
func (c *EtcdClient) NextVersion(ctx context.Context, volumeName string, major bool) ([]string, error) {
	key := store.VersionKeyspace + volumeName + ".json"

	for i := 0; i < 10; i++ {
		getCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
		resp, gerr := c.client.Get(getCtx, key)
		cancel()
		if gerr != nil {
			return nil, fmt.Errorf("read version: %w", gerr)
		}

		var cur store.VersionCounter
		var modRev int64

		if len(resp.Kvs) > 0 {
			if err := json.Unmarshal(resp.Kvs[0].Value, &cur); err != nil {
				return nil, fmt.Errorf("parse version: %w", err)
			}
			modRev = resp.Kvs[0].ModRevision
		}

		next := cur
		if major {
			next.Major++
			next.Minor = 0
		} else {
			next.Minor++
		}

		nextData, jErr := json.Marshal(next)
		if jErr != nil {
			return nil, fmt.Errorf("marshal version: %w", jErr)
		}

		txnCtx, txnCancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
		txn := c.client.Txn(txnCtx)
		if modRev > 0 {
			txn = txn.If(clientv3.Compare(clientv3.ModRevision(key), "=", modRev))
		} else {
			txn = txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0))
		}
		tresp, terr := txn.Then(clientv3.OpPut(key, string(nextData))).Commit()
		txnCancel()
		if terr != nil {
			return nil, fmt.Errorf("cas version: %w", terr)
		}
		if tresp.Succeeded {
			return []string{
				fmt.Sprintf("v%d", next.Major),
				fmt.Sprintf("v%d.%d", next.Major, next.Minor),
			}, nil
		}
	}

	return nil, fmt.Errorf("version allocation: too many retries")
}

// Close releases all active locks and closes the underlying etcd client.
func (c *EtcdClient) Close() error {
	c.mu.Lock()
	revokeCtx, revokeCancel := context.WithTimeout(context.Background(), c.cfg.RequestTimeout)
	defer revokeCancel()
	for key, cancel := range c.activeLocks {
		cancel()
		if leaseID, ok := c.lastLeaseIDs[key]; ok {
			_, _ = c.client.Revoke(revokeCtx, leaseID)
		}
		delete(c.activeLocks, key)
		delete(c.lastLeaseIDs, key)
	}
	c.mu.Unlock()
	return c.client.Close()
}

// compile-time interface checks
var (
	_ store.Backend     = (*EtcdClient)(nil)
	_ store.Coordinator = (*EtcdClient)(nil)
)

// lockTTLPermanent is the TTL used for permanent locks (~68 years).
const lockTTLPermanent = math.MaxInt32
