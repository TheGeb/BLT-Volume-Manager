package metadata

import (
	"context"
	"errors"
	"time"
)

type ObjectStore interface {
	PutObject(key string, data []byte) error
	ReadObject(key string) ([]byte, error)
	DeleteObject(key string) error
	ListObjects(prefix string) ([]Object, error)
	ListCommonPrefixes(prefix, delimiter string) ([]string, error)
	DeleteObjectsWithPrefix(prefix string) error
}

type Object struct {
	Key          *string
	LastModified *time.Time
}

type Store struct {
	store ObjectStore
}

type OwnerEntry struct {
	Name       string `json:"name"`
	ExpiryTime int64  `json:"expiry_time"`
}

type ErrorCode int

const (
	OwnerLockHeldByAnother ErrorCode = iota
	NoOwnerLockAcquired
)

type OwnerLockError struct {
	Code ErrorCode
	Msg  string
}

var ErrOldOwnerKeyFormat = errors.New("old owner key format, GET needed")

type OwnerClient interface {
	Lock(ctx context.Context, name string) (OwnerLock, error)
}

type OwnerLock interface {
	Release() error
	IsValid() (bool, error)
}

type VersionCounter struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

type RestorePoint struct {
	SnapshotID   string `json:"snapshotID"`
	FallbackHash string `json:"fallbackHash"`
}

type RestorePointStore interface { //FIXME: Shouldn't this just use objectStore?
	WriteRestorePoint(vol string, rp RestorePoint) error
	ReadRestorePoint(vol string) (*RestorePoint, error)
	DeleteRestorePoint(vol string) error
}

var ErrRestorePointNotFound = errors.New("restore point not found")
