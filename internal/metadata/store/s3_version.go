package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// NewS3VersionAllocator creates a VersionAllocator backed by a read-then-write
// of the version counter. This is not atomic across independent writers; it is
// only safe with a single writer (the S3 compare-and-list does not extend to
// version counters).
func NewS3VersionAllocator(b Backend) VersionAllocator {
	return &s3VersionAllocator{b: b}
}

type s3VersionAllocator struct {
	b Backend
}

func (s *s3VersionAllocator) NextVersion(ctx context.Context, volumeName string, major bool) ([]string, error) {
	v, err := s.readCounter(ctx, volumeName)
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
	if err := s.writeCounter(ctx, volumeName, *v); err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("v%d", v.Major),
		fmt.Sprintf("v%d.%d", v.Major, v.Minor),
	}, nil
}

func (s *s3VersionAllocator) readCounter(ctx context.Context, vol string) (*VersionCounter, error) {
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

func (s *s3VersionAllocator) writeCounter(ctx context.Context, vol string, v VersionCounter) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal version counter: %w", err)
	}
	return s.b.PutObject(ctx, VersionKeyspace+vol+".json", data)
}
