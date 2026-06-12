package locker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
)

type ErrorCode int

const (
	LockHeldByAnother ErrorCode = iota
	NoLockAcquired
)

type LockError struct {
	Code ErrorCode
	Msg  string
}

func (e *LockError) Error() string { return e.Msg }

type s3Locker struct {
	bucket      string
	maxHoldMins int
	endpoint    string
	region      string
	store       store.ObjectStore
}

type s3Lock struct {
	store store.ObjectStore
	myKey string
}

func NewS3Locker(bucket string, endpoint string, region string, maxHoldMins int) (Locker, error) {
	s3, err := store.NewS3Store(store.S3StoreConfig{
		S3Bucket:   bucket,
		S3Endpoint: endpoint,
		Region:     region,
		Logger:     applog.S3Call,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 store for locker: %w", err)
	}
	return &s3Locker{bucket: bucket, maxHoldMins: maxHoldMins, endpoint: endpoint, region: region, store: s3}, nil
}

func (s *s3Locker) Acquire(ctx context.Context, name string) (Lock, error) {
	folder := store.LockFolder(name)
	myName := fmt.Sprintf("%s-%d", store.Hostname(), os.Getpid())
	expiry := time.Now().Add(time.Minute * time.Duration(s.maxHoldMins+2)).Unix()

	myKey, err := store.AcquireLock(s.store, folder, myName, expiry)
	if err != nil {
		return nil, &LockError{Code: LockHeldByAnother, Msg: err.Error()}
	}

	return &s3Lock{store: s.store, myKey: myKey}, nil
}

func (l *s3Lock) Release() error {
	return l.store.DeleteObject(l.myKey)
}

func (l *s3Lock) IsValid() (bool, error) {
	folder := l.myKey
	if idx := strings.LastIndex(folder, "/"); idx >= 0 {
		folder = folder[:idx+1]
	}

	objects, err := l.store.ListObjects(folder)
	if err != nil {
		return false, fmt.Errorf("list lock objects: %w", err)
	}

	store.SortLockObjects(objects)
	key, _, _ := store.FilterValidLocksByKey(l.store, objects)
	return key == l.myKey, nil
}
