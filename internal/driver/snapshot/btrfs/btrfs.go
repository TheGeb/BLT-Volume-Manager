package btrfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver/snapshot"
)

type btrfsSnapshotter struct{}

func init() {
	snapshot.Register(&btrfsSnapshotter{})
}

func (b *btrfsSnapshotter) Type() snapshot.Type { return snapshot.TypeBtrfs }

func (b *btrfsSnapshotter) MatchFSType(fsType string) bool {
	return fsType == "btrfs"
}

func (b *btrfsSnapshotter) Init(path string, _ map[string]string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	app.Debugf("btrfs_create_subvolume", "path=%s", path)
	return btrfsCreateSubvolume(path)
}

func btrfsCreateSubvolume(path string) error {
	cmd := exec.Command("btrfs", "subvolume", "create", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume create: %w\n%s", err, string(out))
	}
	return nil
}

func (b *btrfsSnapshotter) Destroy(path string) error {
	return btrfsRemove(path)
}

func (b *btrfsSnapshotter) CreateSnapshot(volPath, accessPath, volName string, info *snapshot.SnapInfo) error {
	if !isSubvolume(volPath) {
		return fmt.Errorf("%s is not a btrfs subvolume; init with btrfs first", volPath)
	}
	if err := os.MkdirAll(info.SnapDir, 0o755); err != nil {
		return fmt.Errorf("create snap dir: %w", err)
	}
	return btrfsCreate(volPath, accessPath)
}

func (b *btrfsSnapshotter) RemoveSnapshot(info *snapshot.SnapInfo) error {
	return btrfsRemove(info.AccessPath)
}

func isSubvolume(path string) bool {
	app.Debugf("btrfs_check_subvolume", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "show", path)
	return cmd.Run() == nil
}

func btrfsCreate(source, dest string) error {
	app.Debugf("btrfs_snapshot_create", "source=%s dest=%s", source, dest)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", source, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume snapshot: %w\n%s", err, string(out))
	}
	return nil
}

func btrfsRemove(path string) error {
	app.Debugf("btrfs_snapshot_delete", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete: %w\n%s", err, string(out))
	}
	return nil
}
