package store

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

const RestorePointKeyspace = "blt-volume-manager/restore-points/"

type RestorePointStore struct {
	be backend.KeyValueStore
}

func NewRestorePointStore(be backend.KeyValueStore) *RestorePointStore {
	return &RestorePointStore{be: be}
}

type restorePoint struct {
	SnapshotID   string `json:"snapshotID"`
	FallbackHash string `json:"fallbackHash"`
}

func (s *RestorePointStore) Set(volume, snapshotID string) error {
	if snapshotID == "" {
		return errors.New("snapshot ID is required")
	}
	rp := restorePoint{SnapshotID: snapshotID}
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	return s.be.PutObject(RestorePointKeyspace+volume+".json", data)
}

func (s *RestorePointStore) FindByName(volName string) (string, error) {
	if volName == "" {
		return "", nil
	}
	data, err := s.be.ReadObject(RestorePointKeyspace + volName + ".json")
	if err != nil {
		if errors.Is(err, backend.ErrKeyNotFound) {
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

func (s *RestorePointStore) Delete(volume string) error {
	return s.be.DeleteObject(RestorePointKeyspace + volume + ".json")
}
