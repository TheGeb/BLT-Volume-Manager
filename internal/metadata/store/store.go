package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
)

var (
	ErrKeyNotFound        = errors.New("key not found")
	ErrLockConflict       = errors.New("lock already held by another owner")
	ErrBackendUnavailable = errors.New("backend unavailable")
	ErrLeaseExpired       = errors.New("lease expired")
)

// Backend is the interface for metadata persistence backends (S3, etcd, etc.).
type Backend interface {
	PutObject(ctx context.Context, key string, data []byte) error
	ReadObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
	ListObjects(ctx context.Context, prefix string) ([]s3.Object, error)
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}

// Coordinator is optionally implemented by backends that support distributed
// coordination (etcd). When a Coordinator is available, it replaces the
// S3 compare-and-list algorithm for ownership locks and version allocation
// with atomic etcd transaction/CAS operations.
//
// S3-only mode remains best-effort: version allocation is not safe across
// independent writers without a configured coordinator.
type Coordinator interface {
	// AcquireLock attempts to acquire a distributed lock for the given volume.
	// Returns the lock key on success or an error (ErrLockConflict if held).
	AcquireLock(ctx context.Context, volumeName, ownerID string, ttlSeconds int64) (lockKey string, err error)

	// ReleaseLock releases a lock previously acquired via AcquireLock.
	ReleaseLock(ctx context.Context, lockKey string) error

	// RenewLock renews the lease/expiry for an active lock.
	RenewLock(ctx context.Context, lockKey string) error

	// LockIsValid checks whether a lock key still exists and is valid.
	LockIsValid(ctx context.Context, lockKey string) (bool, error)

	// FindLock reads owner information for a volume lock.
	FindLock(ctx context.Context, volumeName string) (key string, owner string, creation int64, expiry int64, err error)

	// ListAllLocks returns all active locks grouped by volume name.
	ListAllLocks(ctx context.Context) (map[string]VolumeOwner, error)

	// NextVersion atomically increments and returns the next version tags
	// for a volume using an etcd CAS transaction.
	NextVersion(ctx context.Context, volumeName string, major bool) ([]string, error)
}

// classifyErr wraps a backend error, preserving ErrKeyNotFound and ErrLockConflict,
// and converting other errors into ErrBackendUnavailable where appropriate.
func classifyErr(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrKeyNotFound) || errors.Is(err, ErrLockConflict) {
		return err
	}
	return fmt.Errorf("%s: %w", op, ErrBackendUnavailable)
}
