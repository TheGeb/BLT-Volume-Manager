package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

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
		if o.GetRemainingTimeInSeconds() <= 0 {
			_ = s3.DeleteObject(*obj.Key)
			continue
		}
		return *obj.Key, &o
	}
	return "", nil
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
