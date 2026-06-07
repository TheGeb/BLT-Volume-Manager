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

// ErrRestorePointNotFound is returned when a restore point does not exist.
var ErrRestorePointNotFound = errors.New("restore point not found")

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

// FilterValidLocks reads lock objects, deletes expired ones, and returns
// the first valid lock owner and its key. Returns ("", nil, 0) if no valid lock.
func FilterValidLocks(s3 ObjectStore, objects []types.Object) (firstKey string, firstOwner *LockOwner) {
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		raw, err := s3.ReadObject(*obj.Key)
		if err != nil || raw == nil {
			continue
		}
		var o LockOwner
		if err := json.Unmarshal(raw, &o); err != nil {
			continue
		}
		if o.ExpiryTime > 0 && o.GetRemainingTimeInSeconds() <= 0 {
			_ = s3.DeleteObject(*obj.Key)
			continue
		}
		return *obj.Key, &o
	}
	return "", nil
}

// LockKey extracts volume name, owner, and expiry unix timestamp from a lock
// key created with the format: blt-volume-manager/locks/<volume>/<owner>-<expiry>.json
//
// Expiry is the unix timestamp when the lock expires, or 0 for permanent locks.
// Returns ErrOldLockKeyFormat for keys still using the legacy nanosecond timestamp.
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

// WriteRestorePoint marshals and stores a restore point for the given volume.
func WriteRestorePoint(s3 ObjectStore, volumeName string, rp RestorePoint) error {
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	return s3.PutObject(RestorePointPrefix+volumeName+".json", data)
}

// ReadRestorePoint reads and unmarshals a restore point for the given volume.
func ReadRestorePoint(s3 ObjectStore, volumeName string) (*RestorePoint, error) {
	data, err := s3.ReadObject(RestorePointPrefix + volumeName + ".json")
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrRestorePointNotFound
		}
		return nil, err
	}
	var rp RestorePoint
	if err := json.Unmarshal(data, &rp); err != nil {
		return nil, fmt.Errorf("parse restore point: %w", err)
	}
	return &rp, nil
}

// DeleteRestorePoint removes the restore point for the given volume.
func DeleteRestorePoint(s3 ObjectStore, volumeName string) error {
	return s3.DeleteObject(RestorePointPrefix + volumeName + ".json")
}

func WriteVersionCounter(s3 ObjectStore, vol string, v VersionCounter) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal version counter: %w", err)
	}
	return s3.PutObject(VersionPrefix+vol+".json", data)
}

func ReadVersionCounter(s3 ObjectStore, vol string) (*VersionCounter, error) {
	data, err := s3.ReadObject(VersionPrefix + vol + ".json")
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	var v VersionCounter
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse version counter: %w", err)
	}
	return &v, nil
}
