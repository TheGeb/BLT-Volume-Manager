package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func AcquireOwnerLock(store ObjectStore, folder, owner string, expiry int64) (myKey string, err error) {
	//TODO: Thoroughly examine every call in this method
	myKey = fmt.Sprintf("%s%s-%d.json", folder, owner, expiry)
	proposal := OwnerEntry{Name: owner, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return "", fmt.Errorf("marshal proposal: %w", err)
	}

	if err := store.PutObject(myKey, data); err != nil {
		return "", fmt.Errorf("create proposal: %w", err)
	}
	defer func() {
		if err != nil {
			_ = store.DeleteObject(myKey)
		}
	}()

	objects, err := store.ListObjects(folder)
	if err != nil {
		return "", fmt.Errorf("list proposals: %w", err)
	}

	SortOwnerObjects(objects)

	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		k := *obj.Key
		if k == myKey || !strings.Contains(k, owner) {
			continue
		}
		_ = store.DeleteObject(k)
	}

	objects, err = store.ListObjects(folder)
	if err != nil {
		return "", fmt.Errorf("re-list proposals: %w", err)
	}
	SortOwnerObjects(objects)

	key, _, _ := FindOwner(store, objects)
	if key != myKey {
		return "", fmt.Errorf("another owner proposal was earlier")
	}

	return myKey, nil
}

func (o *OwnerEntry) RemainingSeconds() int64 {
	if o.ExpiryTime == 0 {
		return 1<<62 - 1 //FIXME: This seems like an ugly way of handling no expiration
	}
	return o.ExpiryTime - time.Now().Unix()
}

func (e *OwnerLockError) Error() string { return e.Msg }

func OwnerFolder(volumeName string) string { //FIXME: Rename. Folder is misleading, but Prefix has other meaning? maybe Base or similar?
	return OwnerPrefix + volumeName + "/"
}

func SortOwnerObjects(objects []Object) {
	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].LastModified, objects[j].LastModified
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		return *objects[i].Key < *objects[j].Key
	})
}

func ParseOwnerKey(key string) (volume, owner string, expiry int64, err error) {
	if !strings.HasPrefix(key, OwnerPrefix) {
		return "", "", 0, fmt.Errorf("key does not start with owner prefix")
	}
	s := strings.TrimPrefix(key, OwnerPrefix)
	idx := strings.LastIndexByte(s, '/')
	if idx < 0 {
		return "", "", 0, fmt.Errorf("no volume/owner separator in owner key: %q", key)
	}
	volume = s[:idx]
	rest := s[idx+1:]
	if !strings.HasSuffix(rest, ".json") {
		return "", "", 0, fmt.Errorf("owner key missing .json suffix: %q", key)
	}
	rest = rest[:len(rest)-5]
	lastDash := strings.LastIndexByte(rest, '-')
	if lastDash < 0 {
		return "", "", 0, fmt.Errorf("no expiry separator in owner key: %q", key)
	}
	owner = rest[:lastDash]
	expiryStr := rest[lastDash+1:]
	expiry, err = strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("parse expiry from owner key: %w", err)
	}
	if expiry > 1e15 { //FIXME: Ugly no-expiration handling, seems like it can be completely removed
		return volume, owner, 0, ErrOldOwnerKeyFormat
	}
	return volume, owner, expiry, nil
}

func FindOwner(store ObjectStore, objects []Object) (firstKey string, firstOwner string, firstExpiry int64) {
	now := time.Now().Unix()
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, owner, expiry, err := ParseOwnerKey(*obj.Key)
		if err != nil {
			if errors.Is(err, ErrOldOwnerKeyFormat) && store != nil {
				raw, readErr := store.ReadObject(*obj.Key)
				if readErr != nil || raw == nil {
					continue
				}
				var o OwnerEntry
				if jsonErr := json.Unmarshal(raw, &o); jsonErr != nil {
					continue
				}
				if o.ExpiryTime > 0 && o.RemainingSeconds() <= 0 {
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

func RemoveStaleObjects(store ObjectStore, objects []Object, ttl time.Duration) []Object {
	kept := make([]Object, 0, len(objects))
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, _, expiry, err := ParseOwnerKey(*obj.Key)
		isPermanent := err == nil && expiry == 0
		if !isPermanent && obj.LastModified != nil && time.Since(*obj.LastModified) > ttl {
			_ = store.DeleteObject(*obj.Key)
			continue
		}
		kept = append(kept, obj)
	}
	return kept
}

func (s *Store) DeleteOwnerObjects(ownerFolder string) error {
	if ownerFolder == "" {
		return nil
	}
	return s.store.DeleteObjectsWithPrefix(ownerFolder)
}
