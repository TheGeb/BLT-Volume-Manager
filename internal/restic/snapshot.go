package restic

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic/cli"
)

type ListSnapshotsOpts = cli.ListSnapshotsOpts

func (m *Manager) ListSnapshots() ([]Snapshot, error) {
	return m.ListSnapshotsWithOpts(nil)
}

func (m *Manager) ListSnapshotsWithOpts(opts *ListSnapshotsOpts) ([]Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	out, err := m.runner.Snapshots(ctx, opts)
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

func (m *Manager) ForgetSnapshots(snapshotIDs ...string) error {
	if len(snapshotIDs) == 0 {
		return errors.New("at least one snapshot ID is required")
	}
	return m.runner.Forget(context.Background(), snapshotIDs...)
}

func (m *Manager) TagSnapshot(snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	return m.runner.TagAdd(context.Background(), snapshotID, tag)
}

func (m *Manager) UntagSnapshot(snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	return m.runner.TagRemove(context.Background(), snapshotID, tag)
}

func (m *Manager) RestoreSnapshot(snapshotID, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutMedium)
	defer cancel()
	return m.runner.Restore(ctx, snapshotID, target)
}
