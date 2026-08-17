package store

import (
	"context"
	"encoding/json"
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

// CheckAndUpdateLock implements store.OwnerLock. Compared to AcquireLock it
// may also hand over an existing lock on conflict:
//
//   - a lock held by the same owner is released and re-proposed with the
//     requested expiry - this is how mount-mode renewal re-anchors a lock
//     (compare-and-list leaves no way to win against one's own earlier
//     proposal, so the old proposal must go first);
//   - a lock written by the metadata migration tool (a handoff with no live
//     holder, marked in the proposal body) is taken over so hosts can mount
//     migrated volumes after cutover;
//   - a live lock held by another owner is still refused with ErrLockConflict.
func (s *s3OwnerLock) CheckAndUpdateLock(ctx context.Context, volumeName, owner string, expiry int64) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		key, err := s.AcquireLock(ctx, volumeName, owner, expiry)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, ErrLockConflict) {
			return "", err
		}
		winKey, winOwner, winEntry, ferr := s.findWinner(ctx, volumeName)
		if ferr != nil {
			return "", ferr
		}
		if winKey == "" {
			continue // lock vanished; retry acquire
		}
		takeOver := winOwner == owner || (winEntry != nil && winEntry.Migrated)
		if !takeOver {
			return "", ErrLockConflict
		}
		// Either our own proposal (renewal) or a migrated handoff with no
		// live holder; dropping the winner lets the next proposal win.
		if rerr := s.b.DeleteObjectsWithPrefix(ctx, OwnerPrefix(volumeName)); rerr != nil {
			return "", fmt.Errorf("release lock for %q: %w", volumeName, rerr)
		}
	}
	return "", fmt.Errorf("acquire or take over lock %q: too many conflicts", volumeName)
}

// findWinner lists the volume's lock proposals and returns the winning
// proposal key, the key-encoded owner, and the parsed proposal body (nil when
// the body is missing or unparseable - the migrated flag then reads as false).
func (s *s3OwnerLock) findWinner(ctx context.Context, volumeName string) (string, string, *OwnerEntry, error) {
	objects, err := s.b.ListObjects(ctx, OwnerPrefix(volumeName))
	if err != nil {
		return "", "", nil, ClassifyErr(err, "list owner objects")
	}
	objects = RemoveStaleObjects(ctx, s.b, objects, DefaultOwnerTTL)

	key, owner, _, _ := determineOwner(objects)
	if key == "" {
		return "", "", nil, nil
	}
	data, rerr := s.b.ReadObject(ctx, key)
	if rerr != nil {
		if errors.Is(rerr, ErrKeyNotFound) {
			return "", "", nil, nil // vanished between list and read
		}
		return "", "", nil, ClassifyErr(rerr, "read owner object")
	}
	var entry OwnerEntry
	if jerr := json.Unmarshal(data, &entry); jerr != nil {
		return key, owner, nil, nil
	}
	return key, owner, &entry, nil
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
