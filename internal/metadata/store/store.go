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
// It is the storage-only capability: it knows how to read/write/list bytes
// but nothing about locks or version allocation.
type Backend interface {
	PutObject(ctx context.Context, key string, data []byte) error
	ReadObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
	ListObjects(ctx context.Context, prefix string) ([]s3.Object, error)
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}

// OwnerLock is the distributed owner-lock protocol a metadata backend
// provides. It is expressed in absolute-expiry semantics (expiry <= 0 means a
// permanent lock) that both storage models honor: etcd translates to a lease
// TTL, S3 encodes it on the proposal object.
//
// Backends choose one implementation at composition time (see NewS3OwnerLock
// and etcd.EtcdClient); stores depend on this interface rather than probing a
// concrete type.
type OwnerLock interface {
	// AcquireLock acquires the lock for the volume on behalf of owner, or
	// returns ErrLockConflict if another owner holds it.
	AcquireLock(ctx context.Context, volumeName, owner string, expiry int64) (lockKey string, err error)

	// CheckAndUpdateLock acquires the lock like AcquireLock, except that on
	// conflict it may refresh a lock the caller already holds (re-stamping its
	// expiry for renewal) or take over a lock written by the metadata
	// migration tool (a handoff with no live holder). A live lock held by
	// another owner is still refused with ErrLockConflict.
	CheckAndUpdateLock(ctx context.Context, volumeName, owner string, expiry int64) (lockKey string, err error)

	// LockIsValid reports whether lockKey still represents a valid lock.
	LockIsValid(ctx context.Context, lockKey string) (bool, error)

	// ReleaseLock releases a lock previously acquired via AcquireLock.
	ReleaseLock(ctx context.Context, lockKey string) error

	// FindForVolume reads the winning owner for a volume (empty if none).
	FindForVolume(ctx context.Context, volumeName string) (*VolumeOwner, error)

	// ListAllLocks returns all active locks grouped by volume name.
	ListAllLocks(ctx context.Context) (map[string]VolumeOwner, error)

	// DeleteForVolume releases and removes all lock state for a volume.
	DeleteForVolume(ctx context.Context, volumeName string) error
}

// VersionAllocator allocates the next version tags for a volume. etcd performs
// the increment atomically with a CAS transaction; the S3 implementation uses
// a best-effort read-increment-write (safe only with a single writer).
type VersionAllocator interface {
	NextVersion(ctx context.Context, volumeName string, major bool) ([]string, error)
}

// MigratedLockImporter is an optional capability of a backend that knows how
// to import an owner lock written by the metadata migration tool (e.g. etcd,
// which writes a target-database lease flagged as migrated). Probed once at
// the migration boundary; see metadata.MigrateOwnerLock.
type MigratedLockImporter interface {
	SetMigratedOwnerLock(ctx context.Context, volumeName, owner string, expiry int64) error
}

// MetadataStore is the fully-wired capability of a metadata backend: raw
// object storage plus its owner-lock and version-allocation implementations.
// It is what cfg.OpenMetadataBackend returns, so a single constructed value
// serves owner stores, version stores, and generic storage without any
// runtime type probing.
type MetadataStore interface {
	Backend
	OwnerLock
	VersionAllocator
}

// NewS3MetadataStore wires a raw S3-style Backend with its owner-lock and
// version-allocation implementations (the compare-and-list algorithms).
func NewS3MetadataStore(b Backend) MetadataStore {
	return &s3MetadataStore{
		Backend:          b,
		OwnerLock:        NewS3OwnerLock(b),
		VersionAllocator: NewS3VersionAllocator(b),
	}
}

type s3MetadataStore struct {
	Backend
	OwnerLock
	VersionAllocator
}

// ClassifyErr wraps a backend error, preserving ErrKeyNotFound and ErrLockConflict,
// and converting other errors into ErrBackendUnavailable where appropriate.
func ClassifyErr(err error, op string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrKeyNotFound) || errors.Is(err, ErrLockConflict) {
		return err
	}
	return fmt.Errorf("%s: %w", op, ErrBackendUnavailable)
}
