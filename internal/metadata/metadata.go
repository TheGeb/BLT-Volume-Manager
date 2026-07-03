package metadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

var ErrKeyNotFound = backend.ErrKeyNotFound

const (
	BackendS3   = "s3"
	BackendEtcd = "etcd"
)

// TODO: Are these only used for S3? perhaps move them to that package
const (
	OwnerPrefix             = "blt-volume-manager/owners/"
	RegisteredVolumesPrefix = "blt-volume-manager/registered-volumes/"
	RestorePointPrefix      = "blt-volume-manager/restore-points/"
	VersionPrefix           = "blt-volume-manager/versions/"
)

const (
	DefaultOwnerMaxHoldMins    = 10
	DefaultOwnerTTL            = 24 * time.Hour
	DefaultOwnerAcquireTimeout = 5 * time.Second
)

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

type OwnerLocker interface {
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

var ErrRestorePointNotFound = errors.New("restore point not found")

func Hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}

type ownerLocker struct {
	stores      *Stores
	maxHoldMins int
}

type ownerLock struct {
	be    backend.KeyValueStore
	myKey string
}

func NewOwnerLocker(s *Stores, maxHoldMins int) OwnerLocker {
	return &ownerLocker{stores: s, maxHoldMins: maxHoldMins}
}

func (c *ownerLocker) Lock(ctx context.Context, name string) (OwnerLock, error) {
	folder := OwnerFolder(name)
	myName := os.Getenv("BLT_OWNER_NAME")
	if myName == "" {
		myName = fmt.Sprintf("%s-%d", Hostname(), os.Getpid())
	}
	expiry := time.Now().Add(time.Minute * time.Duration(c.maxHoldMins+2)).Unix()

	myKey, err := AcquireOwnerLock(c.stores.be, folder, myName, expiry)
	if err != nil {
		return nil, &OwnerLockError{Code: OwnerLockHeldByAnother, Msg: err.Error()}
	}

	return &ownerLock{be: c.stores.be, myKey: myKey}, nil
}

func (o *ownerLock) Release() error {
	return o.be.DeleteObject(o.myKey)
}

func (o *ownerLock) IsValid() (bool, error) {
	_, err := o.be.ReadObject(o.myKey)
	if err != nil {
		return false, nil
	}
	return true, nil
}
