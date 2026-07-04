package store

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

const OwnerKeyspace = "blt-volume-manager/owners/"

const DefaultOwnerTTL = 24 * time.Hour

type OwnerEntry struct {
	Name       string `json:"name"`
	ExpiryTime int64  `json:"expiry_time"`
}

type VolumeOwner struct {
	Volume string
	Owner  string
	Expiry int64
}

type OwnerStore struct {
	be backend.KeyValueStore
}

func NewOwnerStore(be backend.KeyValueStore) *OwnerStore {
	return &OwnerStore{be: be}
}

func (s *OwnerStore) LockVolume(volumeName, ownerName string, expiry int64) (string, error) {
	folder := OwnerPrefix(volumeName)
	return AcquireOwnerLock(s.be, folder, ownerName, expiry)
}

func (s *OwnerStore) LockIsValid(key string) (bool, error) {
	_, _, expiry, err := ParseOwnerKey(key)
	if err != nil {
		return false, fmt.Errorf("parse lock key: %w", err)
	}
	if expiry > 0 && expiry <= time.Now().Unix() {
		return false, nil
	}
	_, err = s.be.ReadObject(key)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (s *OwnerStore) ReleaseLock(key string) error {
	return s.be.DeleteObject(key)
}

func (s *OwnerStore) FindForVolume(volumeName string) (*VolumeOwner, error) {
	objects, err := s.be.ListObjects(OwnerPrefix(volumeName))
	if err != nil {
		return nil, fmt.Errorf("list owner objects: %w", err)
	}

	objects = RemoveStaleObjects(s.be, objects, DefaultOwnerTTL)

	key, owner, expiry := determineWinner(objects)
	if key == "" {
		return &VolumeOwner{Volume: volumeName}, nil
	}
	return &VolumeOwner{Volume: volumeName, Owner: owner, Expiry: expiry}, nil
}

func (s *OwnerStore) ListAllGrouped() (map[string]VolumeOwner, error) {
	objects, err := s.be.ListObjects(OwnerKeyspace)
	if err != nil {
		return nil, err
	}

	objects = RemoveStaleObjects(s.be, objects, DefaultOwnerTTL)

	grouped := make(map[string][]backend.Entry)
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		vol, _, _, err := ParseOwnerKey(*obj.Key)
		if err != nil || vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	result := make(map[string]VolumeOwner, len(grouped))
	for vol, objs := range grouped {
		key, owner, expiry := determineWinner(objs)
		if key != "" {
			result[vol] = VolumeOwner{Volume: vol, Owner: owner, Expiry: expiry}
		}
	}
	return result, nil
}

func (s *OwnerStore) DeleteForVolume(volumeName string) error {
	return s.be.DeleteObjectsWithPrefix(OwnerPrefix(volumeName))
}

func (s *OwnerStore) AcquireForVolume(volumeName, ownerName string, durationMins int) (int64, error) {
	if ownerName == "" {
		return 0, fmt.Errorf("owner name is required")
	}
	var expiry int64
	if durationMins > 0 {
		expiry = time.Now().Add(time.Duration(durationMins) * time.Minute).Unix()
	}

	_, err := s.LockVolume(volumeName, ownerName, expiry)
	if err != nil {
		return 0, err
	}
	return expiry, nil
}

func AcquireOwnerLock(store backend.KeyValueStore, folder, owner string, expiry int64) (myKey string, err error) {
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

	key, _, _ := determineWinner(objects)
	if key != myKey {
		return "", fmt.Errorf("another owner proposal was earlier")
	}

	return myKey, nil
}

func (o *OwnerEntry) RemainingSeconds() int64 {
	if o.ExpiryTime == 0 {
		return math.MaxInt64
	}
	return o.ExpiryTime - time.Now().Unix()
}

func OwnerPrefix(volumeName string) string {
	return OwnerKeyspace + volumeName + "/"
}

func ParseOwnerKey(key string) (volume, owner string, expiry int64, err error) {
	if !strings.HasPrefix(key, OwnerKeyspace) {
		return "", "", 0, fmt.Errorf("key does not start with owner prefix")
	}
	s := strings.TrimPrefix(key, OwnerKeyspace)
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
	return volume, owner, expiry, nil
}

func determineWinner(objects []backend.Entry) (firstKey string, firstOwner string, firstExpiry int64) {
	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].ModificationCounter, objects[j].ModificationCounter
		if ti != nil && tj != nil && *ti != *tj {
			return *ti < *tj
		}
		if ti == nil && tj != nil {
			return false
		}
		if ti != nil && tj == nil {
			return true
		}

		_, _, ei, erri := ParseOwnerKey(*objects[i].Key)
		_, _, ej, errj := ParseOwnerKey(*objects[j].Key)
		if erri == nil && errj == nil && ei != ej {
			return ei < ej
		}

		return *objects[i].Key < *objects[j].Key
	})

	for i := 1; i < len(objects); i++ {
		if objects[i].ModificationCounter != nil && objects[i-1].ModificationCounter != nil &&
			*objects[i].ModificationCounter == *objects[i-1].ModificationCounter {
			log.Warnf("duplicate_modification_counter", "count=%d entries=%s %s",
				*objects[i].ModificationCounter,
				*objects[i-1].Key,
				*objects[i].Key)
		}
	}

	now := time.Now().Unix()
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, owner, expiry, err := ParseOwnerKey(*obj.Key)
		if err != nil {
			continue
		}
		if expiry > 0 && expiry <= now {
			continue
		}
		return *obj.Key, owner, expiry
	}
	return "", "", 0
}

func RemoveStaleObjects(store backend.KeyValueStore, objects []backend.Entry, ttl time.Duration) []backend.Entry {
	now := time.Now()
	kept := make([]backend.Entry, 0, len(objects))
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		_, _, expiry, err := ParseOwnerKey(*obj.Key)
		isPermanent := err == nil && expiry == 0
		if err == nil && expiry > now.Unix() {
			kept = append(kept, obj)
			continue
		}
		if !isPermanent && obj.ModificationCounter != nil {
			modTime := time.Unix(0, *obj.ModificationCounter)
			if now.Sub(modTime) > ttl {
				_ = store.DeleteObject(*obj.Key)
				continue
			}
		}
		kept = append(kept, obj)
	}
	return kept
}
