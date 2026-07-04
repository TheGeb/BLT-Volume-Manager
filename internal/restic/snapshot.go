package restic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		if isRepositoryMissing(string(out)) {
			return nil, nil
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

func (m *Manager) RestoreIfExists(path, preferred string) error { // FIXME: currently unused?
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	var tagFilter string
	if preferred == BackupTagHot || preferred == BackupTagCold {
		tagFilter = preferred
	}
	out, err := m.runner.SnapshotsLast(ctx, tagFilter)
	if err != nil {
		if len(out) == 0 {
			return nil
		}
		return err
	}
	if len(out) == 0 {
		return nil
	}

	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		return fmt.Errorf("parse snapshot list: %w", err)
	}
	if len(snaps) == 0 {
		if preferred == BackupTagHot || preferred == BackupTagCold {
			return m.runner.Restore(ctx, "latest", path, preferred)
		}
		return m.runner.Restore(ctx, "latest", path)
	}
	id := snaps[0].ShortID
	if id == "" {
		id = snaps[0].ID
	}
	if id == "" {
		return m.runner.Restore(ctx, "latest", path)
	}

	return m.runner.Restore(ctx, id, path)
}

func (m *Manager) RestoreSnapshot(snapshotID, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutMedium)
	defer cancel()
	return m.runner.Restore(ctx, snapshotID, target)
}
