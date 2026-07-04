package store

import (
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

const RegisteredVolumeKeyspace = "blt-volume-manager/registered-volumes/"

type RegisteredVolumeStore struct {
	be backend.KeyValueStore
}

func NewRegisteredVolumeStore(be backend.KeyValueStore) *RegisteredVolumeStore {
	return &RegisteredVolumeStore{be: be}
}

func (s *RegisteredVolumeStore) Register(name string) error {
	return s.be.PutObject(RegisteredVolumeKeyspace+name+".json", nil)
}

func (s *RegisteredVolumeStore) Delete(name string) error {
	return s.be.DeleteObject(RegisteredVolumeKeyspace + name + ".json")
}

func (s *RegisteredVolumeStore) List() ([]string, error) {
	objects, err := s.be.ListObjects(RegisteredVolumeKeyspace)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, RegisteredVolumeKeyspace)
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}
