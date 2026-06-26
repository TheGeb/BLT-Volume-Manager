package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
)

func SetRestorePoint(store RestorePointStore, volume, snapshotID string) error {
	if store == nil {
		return errors.New("restore point store not configured")
	}
	if snapshotID == "" {
		return errors.New("snapshot ID is required")
	}
	return store.WriteRestorePoint(volume, RestorePoint{SnapshotID: snapshotID})
}

func FindRestorePointByName(store RestorePointStore, volName string) (string, error) {
	if store == nil || volName == "" {
		return "", nil
	}
	rp, err := store.ReadRestorePoint(volName)
	if err != nil {
		if errors.Is(err, ErrRestorePointNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("read restore point: %w", err)
	}
	if rp.SnapshotID == "" {
		return "", nil
	}
	return rp.SnapshotID, nil
}

func DeleteRestorePoint(store RestorePointStore, volume string) error {
	if store == nil {
		return nil
	}
	return store.DeleteRestorePoint(volume)
}

func (s *Store) WriteRestorePoint(volumeName string, rp RestorePoint) error {
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	return s.store.PutObject(RestorePointPrefix+volumeName+".json", data)
}

func (s *Store) ReadRestorePoint(volumeName string) (*RestorePoint, error) {
	data, err := s.store.ReadObject(RestorePointPrefix + volumeName + ".json")
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrRestorePointNotFound
		}
		return nil, err
	}
	var rp RestorePoint
	if err := json.Unmarshal(data, &rp); err != nil {
		return nil, fmt.Errorf("parse restore point: %w", err)
	}
	return &rp, nil
}

func (s *Store) DeleteRestorePoint(volumeName string) error {
	return s.store.DeleteObject(RestorePointPrefix + volumeName + ".json")
}
