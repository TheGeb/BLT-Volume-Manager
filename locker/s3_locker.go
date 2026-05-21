package locker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/store"
)

type ErrorCode int

const (
	LockAlreadyOwned ErrorCode = iota
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
}

type s3Lock struct {
	rw    *store.S3rw
	myKey string
}

func NewS3Locker(bucket string, endpoint string, region string) (Locker, error) {
	maxMins := 10
	if mv := os.Getenv("S3_LOCK_MAX_MINS"); mv != "" {
		if v, err := strconv.Atoi(mv); err == nil && v > 0 {
			maxMins = v
		}
	}
	return &s3Locker{bucket: bucket, maxHoldMins: maxMins, endpoint: endpoint, region: region}, nil
}

func (s *s3Locker) Acquire(ctx context.Context, name string) (Lock, error) {
	folder := s.lockFolder(name)
	opts := store.S3StoreOpts{AwsBucketName: s.bucket, AwsLockFolder: folder, S3Endpoint: s.endpoint, Region: s.region}
	rw, err := store.NewS3Store(opts)
	if err != nil {
		return nil, fmt.Errorf("create s3 store: %w", err)
	}

	myName := fmt.Sprintf("%s-%d", getHostname(), os.Getpid())
	expiry := time.Now().Add(time.Minute * time.Duration(s.maxHoldMins+2)).Unix()
	myKey := fmt.Sprintf("%s%s-%d.json", folder, myName, time.Now().UnixNano())

	proposal := store.LockOwner{Name: myName, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("marshal proposal: %v", err)}
	}

	if err := rw.PutObject(myKey, data); err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("create proposal: %v", err)}
	}

	objects, err := rw.ListObjects(folder)
	if err != nil {
		rw.DeleteObject(myKey)
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("list proposals: %v", err)}
	}

	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].LastModified, objects[j].LastModified
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		return *objects[i].Key < *objects[j].Key
	})

	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		raw, err := rw.ReadObject(*obj.Key)
		if err != nil || raw == nil {
			continue
		}
		var o store.LockOwner
		if err := json.Unmarshal(raw, &o); err != nil {
			continue
		}
		if o.GetRemainingTimeinSeconds() <= 0 {
			rw.DeleteObject(*obj.Key)
			continue
		}
		if *obj.Key == myKey {
			return &s3Lock{rw: rw, myKey: myKey}, nil
		}
		break
	}

	rw.DeleteObject(myKey)
	return nil, &LockError{Code: LockAlreadyOwned, Msg: "lock held by another host"}
}

func (l *s3Lock) Release() error {
	return l.rw.DeleteObject(l.myKey)
}

func (l *s3Lock) IsValid() (bool, error) {
	// Extract folder prefix from myKey (everything up to and including last /).
	folder := l.myKey
	if idx := strings.LastIndex(folder, "/"); idx >= 0 {
		folder = folder[:idx+1]
	}

	objects, err := l.rw.ListObjects(folder)
	if err != nil {
		return false, fmt.Errorf("list lock objects: %w", err)
	}

	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].LastModified, objects[j].LastModified
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		return *objects[i].Key < *objects[j].Key
	})

	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		raw, err := l.rw.ReadObject(*obj.Key)
		if err != nil || raw == nil {
			continue
		}
		var o store.LockOwner
		if err := json.Unmarshal(raw, &o); err != nil {
			continue
		}
		if o.GetRemainingTimeinSeconds() <= 0 {
			l.rw.DeleteObject(*obj.Key)
			continue
		}
		// First valid proposal — is it ours?
		return *obj.Key == l.myKey, nil
	}

	return false, nil
}

func (s *s3Locker) lockFolder(name string) string {
	return "volume-locks/" + name + "/"
}

func getHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}

func IsLockOwned(err error) bool {
	var le *LockError
	return errors.As(err, &le) && le.Code == LockAlreadyOwned
}
