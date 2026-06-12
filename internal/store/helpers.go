package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// LockFolder returns the S3 prefix for a volume's lock objects.
func LockFolder(volumeName string) string {
	return LockPrefix + volumeName + "/"
}

// Hostname returns os.Hostname() or "unknown" on error.
func Hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}

// SortLockObjects sorts lock objects by LastModified (oldest first),
// falling back to key string comparison for equal timestamps.
func SortLockObjects(objects []types.Object) {
	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].LastModified, objects[j].LastModified
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		return *objects[i].Key < *objects[j].Key
	})
}

// ErrOldLockKeyFormat is returned for keys using the legacy nanosecond timestamp.
var ErrOldLockKeyFormat = errors.New("old lock key format, GET needed")

// ParseLockKey extracts volume, owner, and expiry from a lock object key.
// The key must have format: blt-volume-manager/locks/<volume>/<owner>-<expiry>.json
func ParseLockKey(key string) (volume, owner string, expiry int64, err error) {
	if !strings.HasPrefix(key, LockPrefix) {
		return "", "", 0, fmt.Errorf("key does not start with lock prefix")
	}
	s := strings.TrimPrefix(key, LockPrefix)
	idx := strings.LastIndexByte(s, '/')
	if idx < 0 {
		return "", "", 0, fmt.Errorf("no volume/owner separator in lock key: %q", key)
	}
	volume = s[:idx]
	rest := s[idx+1:]
	if !strings.HasSuffix(rest, ".json") {
		return "", "", 0, fmt.Errorf("lock key missing .json suffix: %q", key)
	}
	rest = rest[:len(rest)-5]
	lastDash := strings.LastIndexByte(rest, '-')
	if lastDash < 0 {
		return "", "", 0, fmt.Errorf("no expiry separator in lock key: %q", key)
	}
	owner = rest[:lastDash]
	expiryStr := rest[lastDash+1:]
	expiry, err = strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("parse expiry from lock key: %w", err)
	}
	if expiry > 1e15 {
		return volume, owner, 0, ErrOldLockKeyFormat
	}
	return volume, owner, expiry, nil
}

// FilterValidLocksByKey validates lock objects using the expiry timestamp encoded
// in the key name, avoiding per-lock GET requests for new-format keys. Locks with
// old key formats fall back to a GET to read the LockOwner body.
func FilterValidLocksByKey(s3 ObjectStore, objects []types.Object) (firstKey string, firstOwner string, firstExpiry int64) {
	now := time.Now().Unix()
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, owner, expiry, err := ParseLockKey(*obj.Key)
		if err != nil {
			if errors.Is(err, ErrOldLockKeyFormat) && s3 != nil {
				raw, readErr := s3.ReadObject(*obj.Key)
				if readErr != nil || raw == nil {
					continue
				}
				var o LockOwner
				if jsonErr := json.Unmarshal(raw, &o); jsonErr != nil {
					continue
				}
				if o.ExpiryTime > 0 && o.GetRemainingTimeInSeconds() <= 0 {
					_ = s3.DeleteObject(*obj.Key)
					continue
				}
				return *obj.Key, o.Name, o.ExpiryTime
			}
			continue
		}
		if expiry > 0 && expiry <= now {
			continue
		}
		return *obj.Key, owner, expiry
	}
	return "", "", 0
}

// ListVolumeMarkers returns all volume names registered under the given prefix.
func ListVolumeMarkers(s3 ObjectStore, prefix string) ([]string, error) {
	objects, err := s3.ListObjects(prefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, prefix)
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}
