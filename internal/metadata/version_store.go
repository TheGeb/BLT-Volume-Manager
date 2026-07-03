package metadata

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

type VersionStore struct {
	be backend.KeyValueStore
}

func NewVersionStore(be backend.KeyValueStore) *VersionStore {
	return &VersionStore{be: be}
}

func (s *VersionStore) NextTags(name string, major bool) ([]string, error) {
	v, err := s.ReadCounter(name)
	if err != nil {
		if !errors.Is(err, ErrKeyNotFound) {
			return nil, err
		}
		v = &VersionCounter{}
	}
	if major {
		v.Major++
		v.Minor = 0
	} else {
		v.Minor++
	}
	if err := s.WriteCounter(name, *v); err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("v%d", v.Major),
		fmt.Sprintf("v%d.%d", v.Major, v.Minor),
	}, nil
}

func (s *VersionStore) WriteCounter(vol string, v VersionCounter) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal version counter: %w", err)
	}
	return s.be.PutObject(VersionPrefix+vol+".json", data)
}

func (s *VersionStore) ReadCounter(vol string) (*VersionCounter, error) {
	data, err := s.be.ReadObject(VersionPrefix + vol + ".json")
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
