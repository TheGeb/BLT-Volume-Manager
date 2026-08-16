package migrate

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
)

// memBackend is an in-memory store.Backend used to exercise migrate().
type memBackend struct {
	mu      sync.Mutex
	entries map[string][]byte
	order   []string
}

func newMemBackend() *memBackend {
	return &memBackend{entries: make(map[string][]byte)}
}

func (b *memBackend) PutObject(_ context.Context, key string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.entries[key]; !ok {
		b.order = append(b.order, key)
	}
	b.entries[key] = data
	return nil
}

func (b *memBackend) ReadObject(_ context.Context, key string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, ok := b.entries[key]
	if !ok {
		return nil, store.ErrKeyNotFound
	}
	return data, nil
}

func (b *memBackend) DeleteObject(_ context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, key)
	return nil
}

func (b *memBackend) ListObjects(_ context.Context, prefix string) ([]s3.Object, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var objs []s3.Object
	for i, k := range b.order {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			key := k
			mc := int64(i + 1)
			objs = append(objs, s3.Object{Key: &key, ModificationCounter: &mc})
		}
	}
	return objs, nil
}

func (b *memBackend) DeleteObjectsWithPrefix(_ context.Context, prefix string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for k := range b.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(b.entries, k)
		}
	}
	return nil
}

func TestRefreshExpiry(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()

	if got, ok := refreshExpiry(0); !ok || got != 0 {
		t.Errorf("refreshExpiry(0) = (%d, %v), want (0, true)", got, ok)
	}
	expiry := now + 600
	got, ok := refreshExpiry(expiry)
	if !ok {
		t.Fatal("expected refreshable lock")
	}
	if got < expiry {
		t.Errorf("refreshed expiry %d should not be earlier than original %d", got, expiry)
	}
	if _, ok := refreshExpiry(now - 10); ok {
		t.Error("expected expired lock to be skipped")
	}
}

func TestMigrateCopiesKeysAndRefreshesLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := newMemBackend()
	dst := newMemBackend()
	now := time.Now().Unix()

	// Owner lock (time-limited) for vol-a.
	lockKey, err := store.OwnerProposalKey("vol-a", "host-1", now-3600, now+3600)
	if err != nil {
		t.Fatal(err)
	}
	_ = src.PutObject(ctx, lockKey, []byte(`{"name":"host-1","expiry_time":`+strconv.FormatInt(now+3600, 10)+`}`))

	// Non-lock metadata.
	versionKey := store.VersionKeyspace + "vol-a.json"
	regKey := store.RegisteredVolumeKeyspace + "vol-b.json"
	_ = src.PutObject(ctx, versionKey, []byte(`{"major":3,"minor":2}`))
	_ = src.PutObject(ctx, regKey, nil)

	res, err := Copy(ctx, store.NewS3MetadataStore(src), dst, false)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.OwnerLocks != 1 || res.Keys != 2 {
		t.Errorf("result = %+v, want 1 owner lock and 2 keys", res)
	}

	// Version + registered keys copied verbatim.
	if data, err := dst.ReadObject(ctx, versionKey); err != nil || string(data) != `{"major":3,"minor":2}` {
		t.Errorf("version key not copied: data=%q err=%v", data, err)
	}
	if _, err := dst.ReadObject(ctx, regKey); err != nil {
		t.Errorf("registered-volume key not copied: %v", err)
	}

	// Owner lock written with a refreshed (future) expiry and correct owner.
	objs, err := dst.ListObjects(ctx, store.OwnerPrefix("vol-a"))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 migrated lock object, got %d", len(objs))
	}
	vol, owner, _, expiry, err := store.ParseOwnerKey(*objs[0].Key)
	if err != nil {
		t.Fatalf("parse migrated lock: %v", err)
	}
	if vol != "vol-a" || owner != "host-1" {
		t.Errorf("migrated lock = (%q,%q), want (vol-a, host-1)", vol, owner)
	}
	if expiry <= now {
		t.Errorf("migrated lock expiry %d should be in the future (refreshed)", expiry)
	}
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := newMemBackend()
	dst := newMemBackend()
	now := time.Now().Unix()

	lockKey, err := store.OwnerProposalKey("vol-a", "host-1", now-3600, now+3600)
	if err != nil {
		t.Fatal(err)
	}
	_ = src.PutObject(ctx, lockKey, nil)
	_ = src.PutObject(ctx, store.VersionKeyspace+"vol-a.json", []byte(`{"major":1,"minor":0}`))

	res, err := Copy(ctx, store.NewS3MetadataStore(src), dst, true)
	if err != nil {
		t.Fatalf("migrate(dry-run): %v", err)
	}
	if res.OwnerLocks != 1 || res.Keys != 1 {
		t.Errorf("dry-run result = %+v, want 1 lock and 1 key counted", res)
	}
	objs, err := dst.ListObjects(ctx, store.KeyspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 0 {
		t.Errorf("dry-run wrote %d objects to destination, want 0", len(objs))
	}
}

func TestMigrateSkipsExpiredLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	src := newMemBackend()
	dst := newMemBackend()
	now := time.Now().Unix()

	expiredKey, err := store.OwnerProposalKey("vol-a", "host-1", now-7200, now-3600)
	if err != nil {
		t.Fatal(err)
	}
	_ = src.PutObject(ctx, expiredKey, nil)

	res, err := Copy(ctx, store.NewS3MetadataStore(src), dst, false)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.OwnerLocks != 0 {
		t.Errorf("expected 0 migrated locks, got %d", res.OwnerLocks)
	}
	if objs, _ := dst.ListObjects(ctx, store.OwnerKeyspace); len(objs) != 0 {
		t.Errorf("expected no expired lock migrated, got %d objects", len(objs))
	}
}
