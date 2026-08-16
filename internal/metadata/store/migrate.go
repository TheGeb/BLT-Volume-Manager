package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// KeyspaceRoot is the common prefix for every metadata keyspace.
const KeyspaceRoot = "blt-volume-manager/"

// OwnerLockKey returns the etcd-style lock key for a volume. etcd stores a
// single lock key per volume (as opposed to S3's timestamped proposals).
func OwnerLockKey(volume string) string {
	return OwnerKeyspace + volume + "/lock"
}

// OwnerProposalKey builds an S3-style owner lock key for the given volume,
// owner, creation timestamp and expiry. An expiry of 0 means permanent.
func OwnerProposalKey(volume, owner string, creation, expiry int64) (string, error) {
	var durStr string
	switch {
	case expiry == 0:
		durStr = "0"
	case expiry > creation:
		durStr = formatDuration(time.Duration(expiry-creation) * time.Second)
	default:
		return "", fmt.Errorf("expiry must be in the future or 0 for permanent")
	}
	return fmt.Sprintf("%s%s-%d-%s.json", OwnerPrefix(volume), encodeOwner(owner), creation, durStr), nil
}

// WriteOwnerLock writes a refreshed owner lock for volume to the backend
// using its native lock representation. For S3-like backends it writes a
// timestamped proposal key parsed by ParseOwnerKey. Coordinating backends
// (etcd) are handled by the migration tool directly via the concrete backend,
// not through this generic helper.
//
// It returns the written key.
func WriteOwnerLock(ctx context.Context, b Backend, volume, owner string, expiry int64) (string, error) {
	entry := OwnerEntry{Name: owner, ExpiryTime: expiry, Migrated: true}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("marshal owner entry: %w", err)
	}
	key, err := OwnerProposalKey(volume, owner, time.Now().Unix(), expiry)
	if err != nil {
		return "", err
	}
	if err := b.PutObject(ctx, key, data); err != nil {
		return "", fmt.Errorf("write owner lock: %w", err)
	}
	return key, nil
}
