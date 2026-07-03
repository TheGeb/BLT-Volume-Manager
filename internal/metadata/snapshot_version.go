package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
)

func (s *Store) NextVersionTags(name string, major bool) ([]string, error) {
	v, err := s.ReadVersionCounter(name)
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
	if err := s.WriteVersionCounter(name, *v); err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("v%d", v.Major),
		fmt.Sprintf("v%d.%d", v.Major, v.Minor),
	}, nil
}

func (s *Store) WriteVersionCounter(vol string, v VersionCounter) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal version counter: %w", err)
	}
	return s.store.PutObject(VersionPrefix+vol+".json", data)
}

func (s *Store) ReadVersionCounter(vol string) (*VersionCounter, error) {
	data, err := s.store.ReadObject(VersionPrefix + vol + ".json")
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
