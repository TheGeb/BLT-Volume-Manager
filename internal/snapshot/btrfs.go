package snapshot

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
)

type btrfsSnapshotter struct{}

func init() {
	register(&btrfsSnapshotter{})
}

func (b *btrfsSnapshotter) Type() Type { return TypeBtrfs }

func (b *btrfsSnapshotter) MatchFSType(fsType string) bool {
	return fsType == "btrfs"
}

func (b *btrfsSnapshotter) CreateSnapshot(volPath, accessPath, volName string, info *SnapInfo) error {
	if !IsSubvolume(volPath) {
		return fmt.Errorf("%s is not a btrfs subvolume; init with btrfs first", volPath)
	}
	if err := os.MkdirAll(info.SnapDir, 0o755); err != nil {
		return fmt.Errorf("create snap dir: %w", err)
	}
	return btrfsCreate(volPath, accessPath)
}

func (b *btrfsSnapshotter) RemoveSnapshot(info *SnapInfo) error {
	return btrfsRemove(info.AccessPath)
}

func btrfsCreate(source, dest string) error {
	applog.Debugf("btrfs_snapshot_create", "source=%s dest=%s", source, dest)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", source, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume snapshot: %w\n%s", err, string(out))
	}
	return nil
}

func btrfsRemove(path string) error {
	applog.Debugf("btrfs_snapshot_delete", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete: %w\n%s", err, string(out))
	}
	return nil
}
