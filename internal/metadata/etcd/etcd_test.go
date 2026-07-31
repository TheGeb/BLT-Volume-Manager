package etcd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
)

func etcdAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("ETCD_ADDR")
	if addr == "" {
		t.Skip("Skipping etcd integration test: set ETCD_ADDR environment variable")
	}
	return addr
}

func newEtcdClient(t *testing.T, addr string) *EtcdClient {
	t.Helper()
	cfg := EtcdConfig{
		Endpoints:      strings.Split(addr, ","),
		DialTimeout:    5 * time.Second,
		RequestTimeout: 10 * time.Second,
	}
	cli, err := NewEtcdClient(cfg)
	if err != nil {
		t.Fatalf("NewEtcdClient(%q) error: %v", addr, err)
	}
	t.Cleanup(func() {
		_ = cli.Close()
	})
	return cli
}

func cleanKeys(t *testing.T, cli *EtcdClient, keys ...string) {
	t.Helper()
	for _, k := range keys {
		_, _ = cli.client.Delete(context.Background(), k)
	}
}

func TestNewClient_NoEndpoints(t *testing.T) {
	t.Parallel()
	_, err := NewEtcdClient(EtcdConfig{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewClient_WithEndpoints(t *testing.T) {
	addr := etcdAddr(t)
	cli, err := NewEtcdClient(EtcdConfig{
		Endpoints:   strings.Split(addr, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewEtcdClient(%q) error: %v", addr, err)
	}
	t.Cleanup(func() { _ = cli.Close() })
}

// --- AcquireLock tests ---

func TestAcquireLock_Success(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	lockKey := "test-lock-success/" + t.Name()
	cleanKeys(t, cli, lockKey)
	defer cleanKeys(t, cli, lockKey)

	key, err := cli.AcquireLock(ctx, t.Name(), "owner1", 30)
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty lock key")
	}

	// Lock should be valid
	valid, err := cli.LockIsValid(ctx, key)
	if err != nil {
		t.Fatalf("LockIsValid() error: %v", err)
	}
	if !valid {
		t.Fatal("expected lock to be valid")
	}
}

func TestAcquireLock_Conflict(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	// First acquisition should succeed
	_, err := cli.AcquireLock(ctx, t.Name(), "owner1", 30)
	if err != nil {
		t.Fatalf("first AcquireLock() error: %v", err)
	}

	// Second acquisition should fail with ErrLockConflict
	_, err = cli.AcquireLock(ctx, t.Name(), "owner2", 30)
	if err != store.ErrLockConflict {
		t.Fatalf("expected ErrLockConflict, got %v", err)
	}
}

func TestAcquireLock_Concurrent(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	const workers = 10
	var (
		mu     sync.Mutex
		wins   int
		errors int
		wg     sync.WaitGroup
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			owner := "owner-" + strings.TrimPrefix(t.Name(), "/")
			_, err := cli.AcquireLock(ctx, t.Name(), owner, 30)
			mu.Lock()
			if err == nil {
				wins++
			} else {
				errors++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if wins != 1 {
		t.Errorf("expected exactly 1 winner, got %d", wins)
	}
	if wins+errors != workers {
		t.Errorf("expected %d total results, got %d", workers, wins+errors)
	}
}

func TestAcquireLock_ReleaseLock(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	key, err := cli.AcquireLock(ctx, t.Name(), "owner1", 30)
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}

	// Release the lock
	if err := cli.ReleaseLock(ctx, key); err != nil {
		t.Fatalf("ReleaseLock() error: %v", err)
	}

	// After release, lock should be invalid
	valid, err := cli.LockIsValid(ctx, key)
	if err != nil {
		t.Fatalf("LockIsValid() error: %v", err)
	}
	if valid {
		t.Fatal("expected lock to be invalid after release")
	}

	// Should be able to re-acquire
	_, err = cli.AcquireLock(ctx, t.Name(), "owner2", 30)
	if err != nil {
		t.Fatalf("re-acquire AcquireLock() error: %v", err)
	}
}

func TestFindLock(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	// Without a lock, FindLock should return ErrKeyNotFound
	_, _, _, _, err := cli.FindLock(ctx, t.Name())
	if err != store.ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}

	// Acquire lock
	_, err = cli.AcquireLock(ctx, t.Name(), "test-owner", 30)
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}

	// Find it
	key, owner, creation, expiry, err := cli.FindLock(ctx, t.Name())
	if err != nil {
		t.Fatalf("FindLock() error: %v", err)
	}
	if key == "" {
		t.Fatal("expected non-empty key")
	}
	if owner != "test-owner" {
		t.Errorf("owner = %q, want %q", owner, "test-owner")
	}
	if expiry <= 0 {
		t.Errorf("expected positive expiry, got %d", expiry)
	}
	if creation != 0 {
		t.Errorf("expected creation=0 for etcd locks, got %d", creation)
	}
}

// --- Version tests ---

func TestNextVersion(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	verKey := store.VersionKeyspace + t.Name() + ".json"
	cleanKeys(t, cli, verKey)
	defer cleanKeys(t, cli, verKey)

	// First version
	tags, err := cli.NextVersion(ctx, t.Name(), false)
	if err != nil {
		t.Fatalf("NextVersion(minor) error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}
	if tags[0] != "v0" || tags[1] != "v0.1" {
		t.Errorf("expected [v0 v0.1], got %v", tags)
	}

	// Minor increment
	tags, err = cli.NextVersion(ctx, t.Name(), false)
	if err != nil {
		t.Fatalf("NextVersion(minor) error: %v", err)
	}
	if tags[0] != "v0" || tags[1] != "v0.2" {
		t.Errorf("expected [v0 v0.2], got %v", tags)
	}

	// Major increment
	tags, err = cli.NextVersion(ctx, t.Name(), true)
	if err != nil {
		t.Fatalf("NextVersion(major) error: %v", err)
	}
	if tags[0] != "v1" || tags[1] != "v1.0" {
		t.Errorf("expected [v1 v1.0], got %v", tags)
	}
}

func TestNextVersion_Concurrent(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	verKey := store.VersionKeyspace + t.Name() + ".json"
	cleanKeys(t, cli, verKey)
	defer cleanKeys(t, cli, verKey)

	const workers = 20
	var (
		mu   sync.Mutex
		tags []string
		wg   sync.WaitGroup
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			verTags, verErr := cli.NextVersion(ctx, t.Name(), false)
			if verErr != nil {
				return
			}
			mu.Lock()
			tags = append(tags, verTags[1])
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(tags) != workers {
		t.Errorf("expected %d version tags, got %d", workers, len(tags))
	}

	// All tags should be unique
	seen := make(map[string]bool)
	for _, tag := range tags {
		if seen[tag] {
			t.Errorf("duplicate tag: %s", tag)
		}
		seen[tag] = true
	}
}

func TestNextVersion_MajorMinor(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	verKey := store.VersionKeyspace + t.Name() + ".json"
	cleanKeys(t, cli, verKey)
	defer cleanKeys(t, cli, verKey)

	// Minor increments
	for i := 1; i <= 3; i++ {
		tags, err := cli.NextVersion(ctx, t.Name(), false)
		if err != nil {
			t.Fatalf("minor %d: %v", i, err)
		}
		if tags[1] != "v0."+itoa(i) {
			t.Errorf("minor %d: expected v0.%d, got %s", i, i, tags[1])
		}
	}

	// Major resets minor
	tags, err := cli.NextVersion(ctx, t.Name(), true)
	if err != nil {
		t.Fatalf("major: %v", err)
	}
	if tags[0] != "v1" || tags[1] != "v1.0" {
		t.Errorf("after major: expected [v1 v1.0], got %v", tags)
	}
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// --- OwnerStore with etcd backend tests ---

func TestOwnerStore_EtcdBackend(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	// Create an OwnerStore with the etcd backend
	s := store.NewOwnerStore(cli)

	t.Run("AcquireForVolume", func(t *testing.T) {
		vol := "test-etcd-acquire-" + t.Name()
		key := lockKeyFor(vol)
		cleanKeys(t, cli, key)
		defer cleanKeys(t, cli, key)

		expiry, err := s.AcquireForVolume(ctx, vol, "etcd-host", 10)
		if err != nil {
			t.Fatalf("AcquireForVolume error: %v", err)
		}
		if expiry <= 0 {
			t.Error("expected positive expiry")
		}

		// Second attempt should fail
		_, err = s.AcquireForVolume(ctx, vol, "other-host", 10)
		if err == nil {
			t.Fatal("expected error for competing acquire")
		}
	})

	t.Run("FindForVolume", func(t *testing.T) {
		vol := "test-etcd-find-" + t.Name()
		key := lockKeyFor(vol)
		cleanKeys(t, cli, key)
		defer cleanKeys(t, cli, key)

		// Without lock
		vo, err := s.FindForVolume(ctx, vol)
		if err != nil {
			t.Fatalf("FindForVolume error: %v", err)
		}
		if vo.Owner != "" {
			t.Errorf("expected empty owner, got %q", vo.Owner)
		}

		// With lock
		_, err = s.AcquireForVolume(ctx, vol, "find-host", 10)
		if err != nil {
			t.Fatalf("AcquireForVolume error: %v", err)
		}
		vo, err = s.FindForVolume(ctx, vol)
		if err != nil {
			t.Fatalf("FindForVolume error: %v", err)
		}
		if vo.Owner != "find-host" {
			t.Errorf("owner = %q, want %q", vo.Owner, "find-host")
		}
	})

	t.Run("LockVolume_ReleaseLock", func(t *testing.T) {
		vol := "test-etcd-lockrelease-" + t.Name()
		key := lockKeyFor(vol)
		cleanKeys(t, cli, key)
		defer cleanKeys(t, cli, key)

		lockKey, err := s.LockVolume(ctx, vol, "lock-host", time.Now().Add(30*time.Minute).Unix())
		if err != nil {
			t.Fatalf("LockVolume error: %v", err)
		}

		valid, err := s.LockIsValid(ctx, lockKey)
		if err != nil {
			t.Fatalf("LockIsValid error: %v", err)
		}
		if !valid {
			t.Fatal("expected lock to be valid")
		}

		if err := s.ReleaseLock(ctx, lockKey); err != nil {
			t.Fatalf("ReleaseLock error: %v", err)
		}

		valid, err = s.LockIsValid(ctx, lockKey)
		if err != nil {
			t.Fatalf("LockIsValid error: %v", err)
		}
		if valid {
			t.Fatal("expected lock to be invalid after release")
		}
	})
}

// --- VersionStore with etcd backend tests ---

func TestVersionStore_EtcdBackend(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	verKey := store.VersionKeyspace + t.Name() + ".json"
	cleanKeys(t, cli, verKey)
	defer cleanKeys(t, cli, verKey)

	vs := store.NewVersionStore(cli)

	tags, err := vs.NextTags(ctx, t.Name(), false)
	if err != nil {
		t.Fatalf("NextTags error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
}

// --- Lease expiration tests ---

func TestLeaseExpiration(t *testing.T) {
	addr := etcdAddr(t)
	cli := newEtcdClient(t, addr)
	ctx := context.Background()

	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	// Acquire with very short TTL (2 seconds)
	key, err := cli.AcquireLock(ctx, t.Name(), "owner1", 2)
	if err != nil {
		t.Fatalf("AcquireLock() error: %v", err)
	}

	// Should be valid immediately
	valid, err := cli.LockIsValid(ctx, key)
	if err != nil {
		t.Fatalf("LockIsValid immediately error: %v", err)
	}
	if !valid {
		t.Fatal("expected lock to be valid immediately")
	}

	// Wait for lease to expire
	time.Sleep(4 * time.Second)

	// Should be expired now
	valid, err = cli.LockIsValid(ctx, key)
	if err != nil {
		t.Fatalf("LockIsValid after expiry error: %v", err)
	}
	if valid {
		t.Fatal("expected lock to be expired")
	}
}
