package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const RestorePointKeyspace = "blt-volume-manager/restore-points/"

type RestorePointStore struct {
	b Backend
}

func NewRestorePointStore(b Backend) *RestorePointStore {
	return &RestorePointStore{b: b}
}

type restorePoint struct {
	SnapshotID   string `json:"snapshotID"`
	FallbackHash string `json:"fallbackHash"`
}

func (s *RestorePointStore) Set(ctx context.Context, volume, snapshotID string) error {
	if snapshotID == "" {
		return errors.New("snapshot ID is required")
	}
	rp := restorePoint{SnapshotID: snapshotID}
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	return s.b.PutObject(ctx, RestorePointKeyspace+volume+".json", data)
}

func (s *RestorePointStore) FindByName(ctx context.Context, volName string) (string, error) {
	if volName == "" {
		return "", nil
	}
	data, err := s.b.ReadObject(ctx, RestorePointKeyspace+volName+".json")
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("read restore point: %w", err)
	}
	var rp restorePoint
	if err := json.Unmarshal(data, &rp); err != nil {
		return "", fmt.Errorf("parse restore point: %w", err)
	}
	return rp.SnapshotID, nil
}

func (s *RestorePointStore) Delete(ctx context.Context, volume string) error {
	return s.b.DeleteObject(ctx, RestorePointKeyspace+volume+".json")
}
