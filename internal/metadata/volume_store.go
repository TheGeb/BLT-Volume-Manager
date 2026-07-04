package metadata

import (
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

type VolumeStore struct {
	be backend.KeyValueStore
}

func NewVolumeStore(be backend.KeyValueStore) *VolumeStore {
	return &VolumeStore{be: be}
}

func (s *VolumeStore) Register(name string) error {
	return s.be.PutObject(RegisteredVolumesPrefix+name+".json", nil)
}

func (s *VolumeStore) Delete(name string) error {
	return s.be.DeleteObject(RegisteredVolumesPrefix + name + ".json")
}

func (s *VolumeStore) List() ([]string, error) {
	objects, err := s.be.ListObjects(RegisteredVolumesPrefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, RegisteredVolumesPrefix)
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (s *VolumeStore) DeleteVolumeData(volumeName string) error {
	if err := s.Delete(volumeName); err != nil {
		return err
	}
	if err := s.be.DeleteObjectsWithPrefix(OwnerPrefix + volumeName + "/"); err != nil {
		return err
	}
	if err := s.be.DeleteObject(RestorePointPrefix + volumeName + ".json"); err != nil {
		return err
	}
	return s.be.DeleteObjectsWithPrefix("restic/" + volumeName + "/")
}
