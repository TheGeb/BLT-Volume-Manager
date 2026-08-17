package store

import (
	"context"
	"encoding/json"
	"fmt"
)

const VersionKeyspace = "blt-volume-manager/versions/"

// VersionStore is a thin facade over a Backend for reading/writing counters
// and a VersionAllocator for allocating the next tags. It never inspects the
// concrete backend type.
type VersionStore struct {
	b     Backend
	alloc VersionAllocator
}

// NewVersionStore constructs a VersionStore from a fully-wired MetadataStore.
func NewVersionStore(m MetadataStore) *VersionStore {
	return &VersionStore{b: m, alloc: m}
}

type VersionCounter struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

// NextTags returns the next version tags for a volume. The allocator
// (etcd CAS or S3 read-then-write) performs the increment.
func (s *VersionStore) NextTags(ctx context.Context, name string, major bool) ([]string, error) {
	return s.alloc.NextVersion(ctx, name, major)
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
		return nil, err
	}
	var v VersionCounter
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse version counter: %w", err)
	}
	return &v, nil
}
