package store

import (
	"encoding/json"
	"errors"
	"fmt"
)

func WriteVersionCounter(s3 ObjectStore, vol string, v VersionCounter) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal version counter: %w", err)
	}
	return s3.PutObject(VersionPrefix+vol+".json", data)
}

func ReadVersionCounter(s3 ObjectStore, vol string) (*VersionCounter, error) {
	data, err := s3.ReadObject(VersionPrefix + vol + ".json")
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
