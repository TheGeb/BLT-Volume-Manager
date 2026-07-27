package snapshot

import (
	"context"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type ResticManager interface {
	Repo() string
	Backup(ctx context.Context, path string, tags ...string) error
	BackupInDir(ctx context.Context, path string, tags []string, workDir string) error
	ListSnapshots(ctx context.Context) ([]restic.Snapshot, error)
	ListSnapshotsWithOpts(ctx context.Context, opts *restic.ListSnapshotsOpts) ([]restic.Snapshot, error)
	ForgetSnapshots(ctx context.Context, snapshotIDs ...string) error
	RepoExists(ctx context.Context) (bool, error)
	Init(ctx context.Context) error
	Stats(ctx context.Context) (*restic.RepoStats, error)
	SnapshotStats(ctx context.Context, snapshotID string) (*restic.SnapshotSizeResult, error)
	Check(ctx context.Context, noLock bool) error
	Repair(ctx context.Context) error
	CopyTo(ctx context.Context, destRepo string, snapshotIDs ...string) error
	GenerateHash(s restic.Snapshot) string
	FindSnapshotByHash(ctx context.Context, hash string) (*restic.Snapshot, error)
	SnapshotHosts(ctx context.Context, latest int) ([]string, error)
	ListSnapshotFiles(ctx context.Context, snapshotID, path string) ([]restic.FileNode, error)
	DumpFile(ctx context.Context, snapshotID, path string) ([]byte, error)
	DiffSnapshots(ctx context.Context, snapID1, snapID2 string) (*restic.DiffResult, error)
}
