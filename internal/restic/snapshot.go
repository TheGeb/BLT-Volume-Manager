package restic

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

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

func (m *Manager) PurgeSnapshots(ctx context.Context) error {
	exists, err := m.RepoExists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return m.runner.ForgetAll(ctx)
}

func (m *Manager) RestoreSnapshot(ctx context.Context, snapshotID, target string) error {
	restoreCtx, cancel := context.WithTimeout(ctx, TimeoutMedium)
	defer cancel()
	return m.runner.Restore(restoreCtx, snapshotID, target)
}
