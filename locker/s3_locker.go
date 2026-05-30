package locker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/store"
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
	store    store.S3Store
}

type s3Lock struct {
	store    store.S3Store
	myKey string
}

func NewS3Locker(bucket string, endpoint string, region string) (Locker, error) {
	maxMins := 10
	if mv := os.Getenv("S3_LOCK_MAX_MINS"); mv != "" {
		if v, err := strconv.Atoi(mv); err == nil && v > 0 {
			maxMins = v
		}
	}
	s3, err := store.NewS3Store(store.S3StoreOpts{
		S3Bucket:   bucket,
		S3Endpoint: endpoint,
		Region:     region,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 store for locker: %w", err)
	}
	return &s3Locker{bucket: bucket, maxHoldMins: maxMins, endpoint: endpoint, region: region, store: s3}, nil
}

func (s *s3Locker) Acquire(ctx context.Context, name string) (Lock, error) {
	folder := store.LockFolder(name)
	s3 := s.store

	myName := fmt.Sprintf("%s-%d", store.Hostname(), os.Getpid())
	expiry := time.Now().Add(time.Minute * time.Duration(s.maxHoldMins+2)).Unix()
	myKey := fmt.Sprintf("%s%s-%d.json", folder, myName, time.Now().UnixNano())

	proposal := store.LockOwner{Name: myName, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("marshal proposal: %v", err)}
	}

	if err := s3.PutObject(myKey, data); err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("create proposal: %v", err)}
	}

	objects, err := s3.ListObjects(folder)
	if err != nil {
		_ = s3.DeleteObject(myKey)
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("list proposals: %v", err)}
	}

	store.SortLockObjects(objects)

	key, _ := store.FilterValidLocks(s3, objects)
	if key == "" {
		_ = s3.DeleteObject(myKey)
		return nil, &LockError{Code: LockHeldByAnother, Msg: "lock held by another host"}
	}
	if key == myKey {
		return &s3Lock{store: s3, myKey: myKey}, nil
	}

	_ = s3.DeleteObject(myKey)
	return nil, &LockError{Code: LockHeldByAnother, Msg: "lock held by another host"}
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
	key, _ := store.FilterValidLocks(l.store, objects)
	return key == l.myKey, nil
}

func getHostname() string {
	return store.Hostname()
}
