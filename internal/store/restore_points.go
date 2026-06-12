package store

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrRestorePointNotFound = errors.New("restore point not found")

func WriteRestorePoint(s3 ObjectStore, volumeName string, rp RestorePoint) error {
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	return s3.PutObject(RestorePointPrefix+volumeName+".json", data)
}

func ReadRestorePoint(s3 ObjectStore, volumeName string) (*RestorePoint, error) {
	data, err := s3.ReadObject(RestorePointPrefix + volumeName + ".json")
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

func DeleteRestorePoint(s3 ObjectStore, volumeName string) error {
	return s3.DeleteObject(RestorePointPrefix + volumeName + ".json")
}
