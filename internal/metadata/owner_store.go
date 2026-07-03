package metadata

import (
	"fmt"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

type OwnerStore struct {
	be backend.KeyValueStore
}

type VolumeOwner struct {
	Volume string
	Owner  string
	Expiry int64
}

func NewOwnerStore(be backend.KeyValueStore) *OwnerStore {
	return &OwnerStore{be: be}
}

func (s *OwnerStore) FindForVolume(volumeName string) (*VolumeOwner, error) {
	objects, err := s.be.ListObjects(OwnerFolder(volumeName))
	if err != nil {
		return nil, fmt.Errorf("list owner objects: %w", err)
	}

	SortOwnersByExpiry(objects)
	objects = RemoveStaleObjects(s.be, objects, DefaultOwnerTTL)

	key, owner, expiry := FindOwner(objects)
	if key == "" {
		return &VolumeOwner{Volume: volumeName}, nil
	}
	return &VolumeOwner{Volume: volumeName, Owner: owner, Expiry: expiry}, nil
}

func (s *OwnerStore) ListAllGrouped() (map[string]VolumeOwner, error) {
	objects, err := s.be.ListObjects(OwnerPrefix)
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
		SortOwnersByExpiry(objs)
		key, owner, expiry := FindOwner(objs)
		if key != "" {
			result[vol] = VolumeOwner{Volume: vol, Owner: owner, Expiry: expiry}
		}
	}
	return result, nil
}

func (s *OwnerStore) DeleteForVolume(volumeName string) error {
	return s.be.DeleteObjectsWithPrefix(OwnerFolder(volumeName))
}

func (s *OwnerStore) AcquireForVolume(volumeName, ownerName string, durationMins int) (int64, error) {
	if ownerName == "" {
		return 0, fmt.Errorf("owner name is required")
	}
	var expiry int64
	if durationMins > 0 {
		expiry = time.Now().Add(time.Duration(durationMins) * time.Minute).Unix()
	}

	folder := OwnerFolder(volumeName)
	_, err := AcquireOwnerLock(s.be, folder, ownerName, expiry)
	if err != nil {
		return 0, err
	}
	return expiry, nil
}
