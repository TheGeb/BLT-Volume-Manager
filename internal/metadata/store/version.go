package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const VersionKeyspace = "blt-volume-manager/versions/"

type VersionStore struct {
	b Backend
}

func NewVersionStore(b Backend) *VersionStore {
	return &VersionStore{b: b}
}

type VersionCounter struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

// S3-only mode note: Version allocation is not safe across independent
// writers unless a Coordinator (etcd) is configured. S3 operations cannot
// provide an atomic compare-and-swap for distributed version counters.

// NextTags returns the next version tags for a volume. When the backend
// implements Coordinator (etcd), the increment is performed atomically
// with an etcd CAS transaction. Otherwise the existing S3 read-then-write
// is used, which is not safe across independent writers.
func (s *VersionStore) NextTags(ctx context.Context, name string, major bool) ([]string, error) {
	if coord, ok := s.b.(Coordinator); ok {
		return coord.NextVersion(ctx, name, major)
	}

	v, err := s.ReadCounter(ctx, name)
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
	if err := s.WriteCounter(ctx, name, *v); err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("v%d", v.Major),
		fmt.Sprintf("v%d.%d", v.Major, v.Minor),
	}, nil
}

func (s *VersionStore) WriteCounter(ctx context.Context, vol string, v VersionCounter) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal version counter: %w", err)
	}
	return s.b.PutObject(ctx, VersionKeyspace+vol+".json", data)
}

func (s *VersionStore) ReadCounter(ctx context.Context, vol string) (*VersionCounter, error) {
	data, err := s.b.ReadObject(ctx, VersionKeyspace+vol+".json")
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
