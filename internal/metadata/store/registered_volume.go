package store

import (
	"context"
	"strings"
)

const RegisteredVolumeKeyspace = "blt-volume-manager/registered-volumes/"

type RegisteredVolumeStore struct {
	b Backend
}

func NewRegisteredVolumeStore(b Backend) *RegisteredVolumeStore {
	return &RegisteredVolumeStore{b: b}
}

func (s *RegisteredVolumeStore) Register(ctx context.Context, name string) error {
	return s.b.PutObject(ctx, RegisteredVolumeKeyspace+name+".json", nil)
}

func (s *RegisteredVolumeStore) Delete(ctx context.Context, name string) error {
	return s.b.DeleteObject(ctx, RegisteredVolumeKeyspace+name+".json")
}

func (s *RegisteredVolumeStore) List(ctx context.Context) ([]string, error) {
	objects, err := s.b.ListObjects(ctx, RegisteredVolumeKeyspace)
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
