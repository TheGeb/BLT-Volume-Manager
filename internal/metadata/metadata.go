package metadata

import (
	"context"
	"os"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
)

var ErrKeyNotFound = store.ErrKeyNotFound

const (
	BackendS3   = "s3"
	BackendEtcd = "etcd"
)

const (
	DefaultOwnerMaxHoldMins    = 10
	DefaultOwnerTTL            = 24 * time.Hour
	DefaultOwnerAcquireTimeout = 5 * time.Second
)

func Hostname() string { // FIXME: Awkward placement, where should this live?
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}

// MigrateOwnerLock writes a refreshed owner lock for volume to b. The concrete
// etcd backend may only be referenced by this package and cfg (see test/arch),
// so the capability check lives here rather than in the migration tool.
//
//   - a backend that implements MigratedLockImporter (etcd) imports the lock
//     with a target-database lease flagged as migrated;
//   - every other backend writes a timestamped proposal key via the S3
//     compare-and-list format (store.WriteOwnerLock).
func MigrateOwnerLock(ctx context.Context, b store.Backend, volume, owner string, expiry int64) error {
	if imp, ok := b.(store.MigratedLockImporter); ok {
		return imp.SetMigratedOwnerLock(ctx, volume, owner, expiry)
	}
	_, err := store.WriteOwnerLock(ctx, b, volume, owner, expiry)
	return err
}
