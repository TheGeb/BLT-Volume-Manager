package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
)

// NewS3OwnerLock creates an OwnerLock backed by the generic S3 compare-and-
// list algorithm (timestamped proposal objects). It implements the same
// OwnerLock protocol as etcd but with weaker atomicity: acquisition is
// best-effort against concurrent writers.
func NewS3OwnerLock(b Backend) OwnerLock {
	return &s3OwnerLock{b: b}
}

type s3OwnerLock struct {
	b Backend
}

func (s *s3OwnerLock) AcquireLock(ctx context.Context, volumeName, owner string, expiry int64) (string, error) {
	return AcquireOwnerLock(ctx, s.b, OwnerPrefix(volumeName), owner, expiry)
}

// CheckAndUpdateLock for S3 is the same as AcquireLock: a conflicted S3
// proposal is either stale (never the winner) or genuinely held, and the
// compare-and-list acquire already handles both. There is no migration
// "handoff" flag in the proposal format, so there is nothing extra to take over.
func (s *s3OwnerLock) CheckAndUpdateLock(ctx context.Context, volumeName, owner string, expiry int64) (string, error) {
	return s.AcquireLock(ctx, volumeName, owner, expiry)
}

func (s *s3OwnerLock) LockIsValid(ctx context.Context, key string) (bool, error) {
	_, _, _, expiry, err := ParseOwnerKey(key)
	if err != nil {
		return false, fmt.Errorf("parse lock key: %w", err)
	}
	if expiry > 0 && expiry <= time.Now().Unix() {
		return false, nil
	}
	_, err = s.b.ReadObject(ctx, key)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return false, nil
		}
		return false, ClassifyErr(err, "read lock object")
	}
	return true, nil
}

func (s *s3OwnerLock) ReleaseLock(ctx context.Context, key string) error {
	return s.b.DeleteObject(ctx, key)
}

func (s *s3OwnerLock) FindForVolume(ctx context.Context, volumeName string) (*VolumeOwner, error) {
	objects, err := s.b.ListObjects(ctx, OwnerPrefix(volumeName))
	if err != nil {
		return nil, fmt.Errorf("list owner objects: %w", ClassifyErr(err, "list owner objects"))
	}

	objects = RemoveStaleObjects(ctx, s.b, objects, DefaultOwnerTTL)

	key, owner, creation, expiry := determineOwner(objects)
	if key == "" {
		return &VolumeOwner{Volume: volumeName}, nil
	}
	return &VolumeOwner{Volume: volumeName, Owner: owner, Creation: creation, Expiry: expiry}, nil
}

func (s *s3OwnerLock) ListAllLocks(ctx context.Context) (map[string]VolumeOwner, error) {
	objects, err := s.b.ListObjects(ctx, OwnerKeyspace)
	if err != nil {
		return nil, ClassifyErr(err, "list owner keyspace")
	}

	objects = RemoveStaleObjects(ctx, s.b, objects, DefaultOwnerTTL)

	grouped := make(map[string][]s3.Object)
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		vol, _, _, _, err := ParseOwnerKey(*obj.Key)
		if err != nil || vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	result := make(map[string]VolumeOwner, len(grouped))
	for vol, objs := range grouped {
		key, owner, creation, expiry := determineOwner(objs)
		if key != "" {
			result[vol] = VolumeOwner{Volume: vol, Owner: owner, Creation: creation, Expiry: expiry}
		}
	}
	return result, nil
}

func (s *s3OwnerLock) DeleteForVolume(ctx context.Context, volumeName string) error {
	return s.b.DeleteObjectsWithPrefix(ctx, OwnerPrefix(volumeName))
}
