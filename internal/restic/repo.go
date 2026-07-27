package restic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

type HostSnapshots struct {
	Host      string     `json:"host"`
	Snapshots []Snapshot `json:"snapshots"`
}

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

func (m *Manager) RepoExists(ctx context.Context) (bool, error) {
	return m.repositoryExists(ctx)
}

func (m *Manager) Init(ctx context.Context) error {
	return m.initRepository(ctx)
}

func (m *Manager) Stats(ctx context.Context) (*RepoStats, error) {
	statsCtx, cancel := context.WithTimeout(ctx, TimeoutShort)
	defer cancel()

	out, err := m.runner.Stats(statsCtx, "raw-data")
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

func (m *Manager) SnapshotStats(ctx context.Context, snapshotID string) (*SnapshotSizeResult, error) {
	statsCtx, cancel := context.WithTimeout(ctx, TimeoutShort)
	defer cancel()

	out, err := m.runner.StatsSnapshot(statsCtx, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("restic snapshot stats: %w", err)
	}

	var result SnapshotSizeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse restic snapshot stats: %w", err)
	}
	return &result, nil
}

func (m *Manager) repositoryExists(ctx context.Context) (bool, error) {
	checkCtx, cancel := context.WithTimeout(ctx, TimeoutShort)
	defer cancel()

	_, err := m.runner.RepoExists(checkCtx)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (m *Manager) initRepository(ctx context.Context) error {
	return m.runner.Init(ctx)
}

func (m *Manager) Check(ctx context.Context, noLock bool) error {
	checkCtx, cancel := context.WithTimeout(ctx, TimeoutMedium)
	defer cancel()
	return m.runner.Check(checkCtx, noLock)
}

func (m *Manager) Repair(ctx context.Context) error {
	if err := m.Unlock(ctx); err != nil {
		log.Warnf("repair_unlock_failed_continuing", "error=%v", err)
	}
	repairCtx, cancel := context.WithTimeout(ctx, TimeoutLong)
	defer cancel()
	return m.runner.Repair(repairCtx)
}

func (m *Manager) CopyTo(ctx context.Context, destRepo string, snapshotIDs ...string) error {
	copyCtx, cancel := context.WithTimeout(ctx, TimeoutLong)
	defer cancel()
	return m.runner.Copy(copyCtx, destRepo, snapshotIDs...)
}

func (m *Manager) Unlock(ctx context.Context) error {
	unlockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return m.runner.Unlock(unlockCtx)
}

func (m *Manager) DeleteRepo(ctx context.Context) error {
	if m.backend == nil {
		return fmt.Errorf("no backend configured for repo cleanup")
	}
	return m.backend.DeleteRepo(ctx, m.repo)
}

func (m *Manager) SnapshotHosts(ctx context.Context, latest int) ([]string, error) {
	hostCtx, cancel := context.WithTimeout(ctx, TimeoutShort)
	defer cancel()

	out, err := m.runner.HostSnapshots(hostCtx)
	if err != nil {
		return nil, err
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	hostSet := make(map[string]bool)
	for _, item := range raw {
		var group struct {
			GroupKey struct {
				Hostname string `json:"hostname"`
			} `json:"group_key"`
		}
		if json.Unmarshal(item, &group) == nil && group.GroupKey.Hostname != "" {
			hostSet[group.GroupKey.Hostname] = true
		}
	}

	hosts := make([]string, 0, len(hostSet))
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts, nil
}
