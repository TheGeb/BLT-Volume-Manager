package locker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/example/docker-s3-volume-plugin/store"
)

type ErrorCode int

const (
	LockAlreadyOwned ErrorCode = iota
	MultipleSavesInProgress
	TooSlowAbandoned
	NoLockAcquired
)

type LockError struct {
	Code ErrorCode
	Msg  string
}

func (e *LockError) Error() string { return e.Msg }

// S3 locker that uses the provided store implementation (owner+counter algorithm).
type s3Locker struct {
	bucket      string
	maxHoldMins int
	endpoint    string
	region      string
}

type s3Lock struct {
	rw        *store.S3rw
	ownerName string
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
	opts := store.S3StoreOpts{AwsBucketName: s.bucket, AwsLockFolder: s.lockFolder(name), S3Endpoint: s.endpoint, Region: s.region}
	rw, err := store.NewS3Store(opts)
	if err != nil {
		return nil, fmt.Errorf("create s3 store: %w", err)
	}

	newOwnerName := fmt.Sprintf("%s-%d", getHostname(), os.Getpid())

	ilc, err := rw.GetLockCounter()
	if err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("get initial lock counter: %v", err)}
	}

	clo, err := rw.GetLockOwner()
	if err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("get lock owner: %v", err)}
	}

	isLockExpired := clo == nil || clo.GetRemainingTimeinSeconds() <= 0

	if clo == nil || clo.Name == newOwnerName || isLockExpired {
		acquireLockDurationInMins := s.maxHoldMins + 2
		lockExpiry := time.Now().Add(time.Minute * time.Duration(acquireLockDurationInMins)).Unix()
		owner := store.LockOwner{Name: newOwnerName, ExpiryTime: lockExpiry}
		if err := rw.SetLockOwner(owner); err != nil {
			return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("unable to set the new lock owner %s: %v", newOwnerName, err)}
		}
	} else {
		return nil, &LockError{Code: LockAlreadyOwned, Msg: fmt.Sprintf("lock is currently held by owner %s", clo.Name)}
	}

	clc, err := rw.GetLockCounter()
	if err != nil {
		return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("get lock counter after set owner: %v", err)}
	}

	if (ilc == nil && clc == nil) || (ilc != nil && clc != nil && ilc.Counter == clc.Counter) {
		newCounter := 0
		if clc != nil {
			newCounter = clc.Counter + 1
		}
		if err := rw.SetLockCounter(store.LockCounter{Counter: newCounter}); err != nil {
			s.releaseOwner(rw, newOwnerName)
			return nil, &LockError{Code: NoLockAcquired, Msg: fmt.Sprintf("error updating the lock counter while acquiring the lock: %v", err)}
		}
	} else {
		s.releaseOwner(rw, newOwnerName)
		return nil, &LockError{Code: MultipleSavesInProgress, Msg: "multiple saves in progress, please retry"}
	}

	clo2, err := rw.GetLockOwner()
	if err != nil || clo2 == nil {
		s.releaseOwner(rw, newOwnerName)
		return nil, &LockError{Code: NoLockAcquired, Msg: "lock is not currently held by anyone but should be"}
	}
	if clo2.Name != newOwnerName {
		s.releaseOwner(rw, newOwnerName)
		return nil, &LockError{Code: LockAlreadyOwned, Msg: fmt.Sprintf("lock currently held by %s", clo2.Name)}
	}
	if clo2.GetRemainingTimeinSeconds() <= int64(s.maxHoldMins)*60 {
		s.releaseOwner(rw, newOwnerName)
		return nil, &LockError{Code: TooSlowAbandoned, Msg: fmt.Sprintf("acquiring the lock took too long, insufficient time remaining")}
	}

	return &s3Lock{rw: rw, ownerName: newOwnerName}, nil
}

func (s *s3Locker) releaseOwner(rw *store.S3rw, name string) {
	clo, err := rw.GetLockOwner()
	if err != nil {
		return
	}
	if clo != nil && clo.Name == name {
		rw.RollBackLockOwner()
	}
}

func (l *s3Lock) Release() error {
	clo, err := l.rw.GetLockOwner()
	if err != nil {
		return fmt.Errorf("error releasing lock: %w", err)
	}
	if clo != nil && clo.Name == l.ownerName {
		if err := l.rw.RollBackLockOwner(); err != nil {
			return fmt.Errorf("error releasing lock: %w", err)
		}
	}
	return nil
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

func IsLockContended(err error) bool {
	var le *LockError
	return errors.As(err, &le) && le.Code == MultipleSavesInProgress
}
