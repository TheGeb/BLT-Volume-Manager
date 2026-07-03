package btrfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"
)

type btrfsProvider struct{}

func init() {
	snapshot.Register(&btrfsProvider{})
}

func (b *btrfsProvider) Type() snapshot.Type { return snapshot.TypeBtrfs }

func (b *btrfsProvider) MatchFSType(fsType string) bool {
	return fsType == "btrfs"
}

func (b *btrfsProvider) Init(path string, _ snapshot.FsOptions) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), app.DefaultDirPerm); err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	log.Debugf("btrfs_create_subvolume", "path=%s", path)
	return btrfsCreateSubvolume(path)
}

func btrfsCreateSubvolume(path string) error {
	cmd := exec.Command("btrfs", "subvolume", "create", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume create: %w\n%s", err, string(out))
	}
	return nil
}

func (b *btrfsProvider) Destroy(path string) error {
	return btrfsRemove(path)
}

func (b *btrfsProvider) CreateSnapshot(volPath, accessPath, volName string, info *snapshot.Info) error {
	if !isSubvolume(volPath) {
		return fmt.Errorf("%s is not a btrfs subvolume; init with btrfs first", volPath)
	}
	if err := os.MkdirAll(info.SnapDir, app.DefaultDirPerm); err != nil {
		return fmt.Errorf("create snap dir: %w", err)
	}
	return btrfsCreate(volPath, accessPath)
}

func (b *btrfsProvider) RemoveSnapshot(info *snapshot.Info) error {
	return btrfsRemove(info.AccessPath)
}

func isSubvolume(path string) bool {
	log.Debugf("btrfs_check_subvolume", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "show", path)
	return cmd.Run() == nil
}

func btrfsCreate(source, dest string) error {
	log.Debugf("btrfs_snapshot_create", "source=%s dest=%s", source, dest)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", source, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume snapshot: %w\n%s", err, string(out))
	}
	return nil
}

func btrfsRemove(path string) error {
	log.Debugf("btrfs_snapshot_delete", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete: %w\n%s", err, string(out))
	}
	return nil
}
