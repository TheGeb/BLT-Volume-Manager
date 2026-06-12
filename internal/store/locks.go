package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// FilterStaleLockObjects removes lock objects older than ttl (unless permanent)
// and deletes them from the store. Returns the remaining valid objects.
func FilterStaleLockObjects(s3 ObjectStore, objects []types.Object, ttl time.Duration) []types.Object {
	kept := make([]types.Object, 0, len(objects))
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, _, expiry, err := ParseLockKey(*obj.Key)
		isPermanent := err == nil && expiry == 0
		if !isPermanent && obj.LastModified != nil && time.Since(*obj.LastModified) > ttl {
			_ = s3.DeleteObject(*obj.Key)
			continue
		}
		kept = append(kept, obj)
	}
	return kept
}

// AcquireLock attempts to acquire a distributed lock using S3 object storage.
// It writes a proposal object, lists existing proposals, and determines if ours
// is the first valid one. Returns the lock object key on success.
func AcquireLock(s3 ObjectStore, folder, owner string, expiry int64) (myKey string, err error) {
	myKey = fmt.Sprintf("%s%s-%d.json", folder, owner, expiry)
	proposal := LockOwner{Name: owner, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return "", fmt.Errorf("marshal proposal: %w", err)
	}

	if err := s3.PutObject(myKey, data); err != nil {
		return "", fmt.Errorf("create proposal: %w", err)
	}
	defer func() {
		if err != nil {
			_ = s3.DeleteObject(myKey)
		}
	}()

	objects, err := s3.ListObjects(folder)
	if err != nil {
		return "", fmt.Errorf("list proposals: %w", err)
	}

	SortLockObjects(objects)

	// Clean stale proposals from the same owner
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		k := *obj.Key
		if k == myKey || !strings.Contains(k, owner) {
			continue
		}
		_ = s3.DeleteObject(k)
	}

	// Re-list after cleanup
	objects, err = s3.ListObjects(folder)
	if err != nil {
		return "", fmt.Errorf("re-list proposals: %w", err)
	}
	SortLockObjects(objects)

	key, _, _ := FilterValidLocksByKey(s3, objects)
	if key != myKey {
		return "", fmt.Errorf("another lock proposal was earlier")
	}

	return myKey, nil
}
