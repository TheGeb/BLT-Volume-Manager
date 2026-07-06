package restic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

type RepoStats struct {
	TotalSize             int64 `json:"total_size"`
	TotalFileCount        int64 `json:"total_file_count"`
	TotalBlobCount        int64 `json:"total_blob_count"`
	TotalUncompressedSize int64 `json:"total_uncompressed_size"`
	UniqueBlobCount       int64 `json:"unique_blob_count"`
	UniqueBlobSize        int64 `json:"unique_blob_size"`
}

type SnapshotSizeResult struct {
	TotalSize      int64 `json:"total_size"`
	TotalFileCount int64 `json:"total_file_count"`
}

func (m *Manager) RepoExists() (bool, error) {
	return m.repositoryExists()
}

func (m *Manager) Init() error {
	return m.initRepository()
}

func (m *Manager) Stats() (*RepoStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	out, err := m.runner.Stats(ctx, "raw-data")
	if err != nil {
		log.Errorf("restic_stats_failed", err, "stderr=%s", string(out))
		return nil, fmt.Errorf("restic stats: %w", err)
	}

	var stats RepoStats
	if err := json.Unmarshal(out, &stats); err != nil {
		log.Errorf("parse_restic_stats_failed", err, "raw=%s", string(out))
		return nil, fmt.Errorf("parse restic stats: %w", err)
	}
	return &stats, nil
}

func (m *Manager) SnapshotStats(snapshotID string) (*SnapshotSizeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	out, err := m.runner.StatsSnapshot(ctx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("restic snapshot stats: %w", err)
	}

	var result SnapshotSizeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse restic snapshot stats: %w", err)
	}
	return &result, nil
}

func (m *Manager) repositoryExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	_, err := m.runner.RepoExists(ctx)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (m *Manager) initRepository() error {
	return m.runner.Init(context.Background())
}

func (m *Manager) Check(noLock bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutMedium)
	defer cancel()
	return m.runner.Check(ctx, noLock)
}

func (m *Manager) Repair() error {
	if err := m.Unlock(); err != nil {
		log.Warnf("repair_unlock_failed_continuing", "error=%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLong)
	defer cancel()
	return m.runner.Repair(ctx)
}

func (m *Manager) CopyTo(destRepo string, snapshotIDs ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutLong)
	defer cancel()
	return m.runner.Copy(ctx, destRepo, snapshotIDs...)
}

func (m *Manager) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.runner.Unlock(ctx)
}

