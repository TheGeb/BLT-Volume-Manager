package store

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

const VersionKeyspace = "blt-volume-manager/versions/"

type VersionStore struct {
	be backend.KeyValueStore
}

func NewVersionStore(be backend.KeyValueStore) *VersionStore {
	return &VersionStore{be: be}
}

type VersionCounter struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

func (s *VersionStore) NextTags(name string, major bool) ([]string, error) {
	v, err := s.ReadCounter(name)
	if err != nil {
		if !errors.Is(err, backend.ErrKeyNotFound) {
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
	return s.be.PutObject(VersionKeyspace+vol+".json", data)
}

func (s *VersionStore) ReadCounter(vol string) (*VersionCounter, error) {
	data, err := s.be.ReadObject(VersionKeyspace + vol + ".json")
	if err != nil {
		if errors.Is(err, backend.ErrKeyNotFound) {
			return nil, backend.ErrKeyNotFound
		}
		return nil, err
	}
	var v VersionCounter
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse version counter: %w", err)
	}
	return &v, nil
}
