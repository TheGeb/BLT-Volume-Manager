package etcd

import (
	"context"
	"encoding/json"
	"errors"
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

// --- store.OwnerLock implementation ---

// AcquireLock implements store.OwnerLock using an etcd transaction/CAS
// with a lease for expiration. expiry is an absolute Unix timestamp; expiry
// <= 0 means a permanent ("create"-mode) lock, represented by a very long
// keepalive-held lease. The lease is automatically kept alive via KeepAlive
// until ReleaseLock is called.
func (c *EtcdClient) AcquireLock(ctx context.Context, volumeName, ownerID string, expiry int64) (string, error) {
	lockKey := lockKeyFor(volumeName)

	ttlSeconds := int64(lockTTLPermanent)
	if expiry > 0 {
		ttlSeconds = expiry - time.Now().Unix()
		if ttlSeconds <= 0 {
			return "", fmt.Errorf("expiry must be in the future")
		}
	}

	// Store a real expiry timestamp so readers (FindLock, ListAllLocks, the
	// migration tool) can derive the lock's remaining lifetime, matching the
	// S3 representation. Permanent locks (the keepalive-held sentinel) keep 0.
	entry := store.OwnerEntry{Name: ownerID}
	if expiry > 0 {
		entry.ExpiryTime = expiry
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal owner entry: %w", err)
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

// ReleaseLock implements store.OwnerLock. It revokes the lease
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

// SetMigratedOwnerLock writes (or refreshes) an owner lock for volume on
// behalf of the metadata migration tool. Unlike AcquireLock it does not start
// keepalive or track the lease: the lock is granted a lease covering the
// remaining time to expiry (reported by the source backend) and self-expires
// once that elapses, which matches the "refreshed expiry" semantics of a
// migrated lock. The operator is expected to restart hosts against this
// backend as part of the cutover. A permanent lock (expiry 0) is written
// without a lease and stays until removed.
//
// It returns ErrLockConflict only if volume already has an active lock held by
// a different owner; the same owner's lock is refreshed in place.
func (c *EtcdClient) SetMigratedOwnerLock(ctx context.Context, volumeName, owner string, expiry int64) error {
	lockKey := lockKeyFor(volumeName)
	entry := store.OwnerEntry{Name: owner, ExpiryTime: expiry, Migrated: true}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal owner entry: %w", err)
	}

	// Grant a lease covering the remaining lifetime so the migrated lock
	// self-expires like the source lock. Permanent locks get no lease.
	var leaseID clientv3.LeaseID
	if expiry > 0 {
		ttl := expiry - time.Now().Unix()
		if ttl <= 0 {
			return store.ErrLeaseExpired
		}
		leaseResp, gerr := c.client.Grant(ctx, ttl)
		if gerr != nil {
			return fmt.Errorf("grant lease: %w", gerr)
		}
		leaseID = leaseResp.ID
	}

	op := func() clientv3.Op {
		if leaseID != 0 {
			return clientv3.OpPut(lockKey, string(data), clientv3.WithLease(leaseID))
		}
		return clientv3.OpPut(lockKey, string(data))
	}

	keepLease := leaseID == 0
	defer func() {
		if !keepLease {
			revokeCtx, cancel := context.WithTimeout(context.Background(), c.cfg.RequestTimeout)
			defer cancel()
			_, _ = c.client.Revoke(revokeCtx, leaseID)
		}
	}()

	putCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	for attempt := 0; attempt < 3; attempt++ {
		// Fast path: the lock does not exist yet.
		tresp, terr := c.client.Txn(putCtx).
			If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0)).
			Then(op()).
			Commit()
		if terr != nil {
			return fmt.Errorf("set owner lock: %w", terr)
		}
		if tresp.Succeeded {
			keepLease = true
			return nil
		}

		// The lock already exists. Only a lock held by the same owner may be
		// refreshed; a different owner's active lock is a hard conflict.
		getResp, gerr := c.client.Get(putCtx, lockKey)
		if gerr != nil {
			return fmt.Errorf("read existing lock: %w", gerr)
		}
		if len(getResp.Kvs) == 0 {
			continue // lock expired between the txn and get; retry
		}
		var cur store.OwnerEntry
		if jerr := json.Unmarshal(getResp.Kvs[0].Value, &cur); jerr != nil {
			return fmt.Errorf("parse existing lock: %w", jerr)
		}
		if cur.Name != owner {
			return store.ErrLockConflict
		}
		rev := getResp.Kvs[0].ModRevision
		uresp, uerr := c.client.Txn(putCtx).
			If(clientv3.Compare(clientv3.ModRevision(lockKey), "=", rev)).
			Then(op()).
			Commit()
		if uerr != nil {
			return fmt.Errorf("refresh owner lock: %w", uerr)
		}
		if uresp.Succeeded {
			keepLease = true
			return nil
		}
	}
	return fmt.Errorf("set owner lock %q: too many concurrent modifications", lockKey)
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

// CheckAndUpdateLock implements store.OwnerLock. It acquires the lock like
// AcquireLock, but on conflict it may refresh or hand over the existing lock:
//
//   - a lock already held by the same owner is refreshed in place (its
//     recorded expiry is re-stamped, the Migrated flag cleared) and the same
//     key is returned - this is how mount-mode renewal re-anchors a lock
//     without creating a takeover window;
//   - a lock written by the metadata migration tool (Migrated - a handoff
//     with no live holder) is released and re-acquired;
//   - a live lock held by another owner is never taken over and returns
//     ErrLockConflict.
func (c *EtcdClient) CheckAndUpdateLock(ctx context.Context, volumeName, owner string, expiry int64) (string, error) {
	lockKey := lockKeyFor(volumeName)
	for attempt := 0; attempt < 3; attempt++ {
		key, err := c.AcquireLock(ctx, volumeName, owner, expiry)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, store.ErrLockConflict) {
			return "", err
		}
		_, curOwner, _, _, migrated, ferr := c.FindLock(ctx, volumeName)
		if ferr != nil {
			if errors.Is(ferr, store.ErrKeyNotFound) {
				continue // lock vanished; retry acquire
			}
			return "", ferr
		}
		if curOwner == owner {
			// Our own lock: re-stamp in place, keeping the key and its
			// (keepalive-held) lease.
			refreshed, rerr := c.refreshOwnLock(ctx, lockKey, expiry)
			if rerr != nil {
				return "", fmt.Errorf("refresh own lock %q: %w", lockKey, rerr)
			}
			if refreshed {
				return lockKey, nil
			}
			continue // key changed under us; retry acquire
		}
		if !migrated {
			return "", store.ErrLockConflict
		}
		if rerr := c.ReleaseLockAny(ctx, lockKey); rerr != nil {
			return "", fmt.Errorf("take over migrated lock: %w", rerr)
		}
	}
	return "", fmt.Errorf("acquire or take over lock %q: too many conflicts", volumeName)
}

// refreshOwnLock re-stamps the recorded expiry of a lock key without touching
// its lease (the lease is already kept alive by the holder). The value is
// CAS-updated against its current revision with the existing lease re-attached
// explicitly, so the refresh can never detach or extend the lease. It returns
// whether the refresh succeeded (false when the key changed concurrently or
// vanished, in which case the caller should retry the acquire).
func (c *EtcdClient) refreshOwnLock(ctx context.Context, lockKey string, expiry int64) (bool, error) {
	refCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	for attempt := 0; attempt < 3; attempt++ {
		getResp, gerr := c.client.Get(refCtx, lockKey)
		if gerr != nil {
			return false, gerr
		}
		if len(getResp.Kvs) == 0 {
			return false, nil // lock vanished; caller retries
		}
		var cur store.OwnerEntry
		if jerr := json.Unmarshal(getResp.Kvs[0].Value, &cur); jerr != nil {
			return false, jerr
		}
		next := cur
		next.ExpiryTime = expiry
		next.Migrated = false // we are a live holder now, not a handoff
		data, jerr := json.Marshal(next)
		if jerr != nil {
			return false, jerr
		}
		put := clientv3.OpPut(lockKey, string(data))
		if lease := getResp.Kvs[0].Lease; lease != 0 {
			put = clientv3.OpPut(lockKey, string(data), clientv3.WithLease(clientv3.LeaseID(lease)))
		}
		rev := getResp.Kvs[0].ModRevision
		uresp, uerr := c.client.Txn(refCtx).
			If(clientv3.Compare(clientv3.ModRevision(lockKey), "=", rev)).
			Then(put).
			Commit()
		if uerr != nil {
			return false, uerr
		}
		if uresp.Succeeded {
			return true, nil
		}
	}
	return false, nil
}

// FindForVolume implements store.OwnerLock.
func (c *EtcdClient) FindForVolume(ctx context.Context, volumeName string) (*store.VolumeOwner, error) {
	_, owner, creation, expiry, migrated, err := c.FindLock(ctx, volumeName)
	if err != nil {
		if errors.Is(err, store.ErrKeyNotFound) {
			return &store.VolumeOwner{Volume: volumeName}, nil
		}
		return nil, store.ClassifyErr(err, "find lock")
	}
	return &store.VolumeOwner{Volume: volumeName, Owner: owner, Creation: creation, Expiry: expiry, Migrated: migrated}, nil
}

// DeleteForVolume implements store.OwnerLock.
func (c *EtcdClient) DeleteForVolume(ctx context.Context, volumeName string) error {
	lockKey, _, _, _, _, fErr := c.FindLock(ctx, volumeName)
	if fErr != nil && !errors.Is(fErr, store.ErrKeyNotFound) {
		return fmt.Errorf("find lock for volume: %w", fErr)
	}
	if fErr == nil {
		if err := c.ReleaseLock(ctx, lockKey); err != nil {
			return fmt.Errorf("release lock for volume: %w", err)
		}
	}
	return c.DeleteObjectsWithPrefix(ctx, store.OwnerPrefix(volumeName))
}

// LockIsValid implements store.OwnerLock.
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
		ttlCtx, ttlCancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
		leaseResp, lerr := c.client.TimeToLive(ttlCtx, clientv3.LeaseID(kv.Lease))
		ttlCancel()
		if lerr != nil {
			return false, fmt.Errorf("check lease TTL: %w", lerr)
		}
		if leaseResp.TTL <= 0 {
			return false, nil
		}
	}
	return true, nil
}

// FindLock reads the owner entry for a volume's lock key.
func (c *EtcdClient) FindLock(ctx context.Context, volumeName string) (string, string, int64, int64, bool, error) {
	lockKey := lockKeyFor(volumeName)

	getCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	resp, err := c.client.Get(getCtx, lockKey)
	if err != nil {
		return "", "", 0, 0, false, err
	}
	if len(resp.Kvs) == 0 {
		return "", "", 0, 0, false, store.ErrKeyNotFound
	}

	var entry store.OwnerEntry
	if err := json.Unmarshal(resp.Kvs[0].Value, &entry); err != nil {
		return "", "", 0, 0, false, fmt.Errorf("unmarshal owner entry: %w", err)
	}

	return lockKey, entry.Name, 0, entry.ExpiryTime, entry.Migrated, nil
}

// ReleaseLockAny deletes a lock key unconditionally (by key), regardless of
// whether this process acquired or tracks its lease. Used to take over a
// migrated lock, where there is no tracked lease to revoke and the lock is a
// handoff with no live holder.
func (c *EtcdClient) ReleaseLockAny(ctx context.Context, lockKey string) error {
	delCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()
	_, err := c.client.Delete(delCtx, lockKey)
	return err
}

// ListAllLocks implements store.OwnerLock.
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
			Volume:   vol,
			Owner:    entry.Name,
			Expiry:   entry.ExpiryTime,
			Migrated: entry.Migrated,
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

// NextVersion implements store.VersionAllocator using an atomic etcd CAS
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
	_ store.Backend              = (*EtcdClient)(nil)
	_ store.OwnerLock            = (*EtcdClient)(nil)
	_ store.VersionAllocator     = (*EtcdClient)(nil)
	_ store.MigratedLockImporter = (*EtcdClient)(nil)
)

// lockTTLPermanent is the TTL used for permanent locks (~68 years).
const lockTTLPermanent = math.MaxInt32
