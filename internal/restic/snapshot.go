package restic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic/cli"
)

type ListSnapshotsOpts = cli.ListSnapshotsOpts

func (m *Manager) ListSnapshots(ctx context.Context) ([]Snapshot, error) {
	return m.ListSnapshotsWithOpts(ctx, nil)
}

func (m *Manager) ListSnapshotsWithOpts(ctx context.Context, opts *ListSnapshotsOpts) ([]Snapshot, error) {
	listCtx, cancel := context.WithTimeout(ctx, TimeoutShort)
	defer cancel()

	out, err := m.runner.Snapshots(listCtx, opts)
	if err != nil {
		exists, repoErr := m.RepoExists(ctx)
		if repoErr == nil && !exists {
			return []Snapshot{}, nil
		}
		return nil, err
	}

	var snapshots []Snapshot
	if err := json.Unmarshal(out, &snapshots); err != nil {
		return nil, err
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})
	return snapshots, nil
}

func (m *Manager) ForgetSnapshots(ctx context.Context, snapshotIDs ...string) error {
	if len(snapshotIDs) == 0 {
		return errors.New("at least one snapshot ID is required")
	}
	return m.runner.Forget(ctx, snapshotIDs...)
}

func (m *Manager) TagSnapshot(ctx context.Context, snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	return m.runner.TagAdd(ctx, snapshotID, tag)
}

func (m *Manager) UntagSnapshot(ctx context.Context, snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	return m.runner.TagRemove(ctx, snapshotID, tag)
}

func (m *Manager) RestoreSnapshot(ctx context.Context, snapshotID, target string) error {
	restoreCtx, cancel := context.WithTimeout(ctx, TimeoutMedium)
	defer cancel()
	return m.runner.Restore(restoreCtx, snapshotID, target)
}

func (m *Manager) GenerateHash(s Snapshot) string {
	paths := make([]string, len(s.Paths))
	copy(paths, s.Paths)
	sort.Strings(paths)

	data := s.Hostname + s.Time.Format(time.RFC3339Nano) + s.Tree + strings.Join(paths, ",")
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func (m *Manager) FindSnapshotByHash(ctx context.Context, hash string) (*Snapshot, error) {
	snapshots, err := m.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range snapshots {
		fullHash := m.GenerateHash(s)
		shortHash := fullHash[:len(s.ShortID)]
		log.Debugf("comparing_hash", "hash=%s snapshot=%s", hash, s.ID)
		if shortHash == hash {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found for hash %s", hash)
}
