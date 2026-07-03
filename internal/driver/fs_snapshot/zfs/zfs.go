package zfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"
)

type zfsProvider struct{}

func init() {
	snapshot.Register(&zfsProvider{})
}

func (z *zfsProvider) Type() snapshot.Type { return snapshot.TypeZFS }

func (z *zfsProvider) MatchFSType(fsType string) bool {
	return fsType == "zfs"
}

func (z *zfsProvider) Init(path string, opts snapshot.FsOptions) error {
	parentDataset := snapshot.RootDataset()
	if opts.ZfsPool != "" {
		parentDataset = opts.ZfsPool
	}
	if parentDataset == "" {
		return fmt.Errorf("no ZFS parent dataset configured; set zfs-pool option or run on ZFS filesystem")
	}

	name := filepath.Base(path)
	full := parentDataset + "/" + name

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}

	log.Debugf("zfs_create", "path=%s dataset=%s", path, full)
	cmd := exec.Command("zfs", "create", "-o", "mountpoint="+path, full)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs create: %w\n%s", err, string(out))
	}
	return nil
}

func (z *zfsProvider) Destroy(path string) error {
	dataset, err := snapshot.ZFSDataset(path)
	if err != nil {
		return err
	}
	log.Debugf("zfs_destroy_dataset", "dataset=%s", dataset)
	cmd := exec.Command("zfs", "destroy", "-r", dataset)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy: %w\n%s", err, string(out))
	}
	return nil
}

func (z *zfsProvider) CreateSnapshot(volPath, accessPath, volName string, info *snapshot.Info) error {
	dataset, err := snapshot.ZFSDataset(volPath)
	if err != nil {
		return fmt.Errorf("resolve zfs dataset: %w", err)
	}
	snapName := filepath.Base(accessPath)
	fullSnap := dataset + "@" + snapName
	if err := zfsCreateSnapshot(fullSnap, accessPath); err != nil {
		return err
	}
	info.ZfsSnap = fullSnap
	return nil
}

func (z *zfsProvider) RemoveSnapshot(info *snapshot.Info) error {
	return zfsRemoveSnapshot(info.ZfsSnap, info.AccessPath)
}

func zfsCreateSnapshot(fullSnap, mountPath string) error {
	log.Debugf("zfs_snapshot_create", "snapshot=%s", fullSnap)
	cmd := exec.Command("zfs", "snapshot", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs snapshot: %w\n%s", err, string(out))
	}

	if err := os.MkdirAll(mountPath, 0o755); err != nil {
		_ = zfsDestroy(fullSnap)
		return fmt.Errorf("create mount dir: %w", err)
	}

	log.Debugf("zfs_mount_snapshot", "snapshot=%s path=%s", fullSnap, mountPath)
	cmd = exec.Command("mount", "-t", "zfs", fullSnap, mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = zfsDestroy(fullSnap)
		_ = os.Remove(mountPath)
		return fmt.Errorf("mount zfs snapshot: %w\n%s", err, string(out))
	}

	return nil
}

func zfsRemoveSnapshot(fullSnap, mountPath string) error {
	log.Debugf("zfs_unmount", "path=%s", mountPath)
	cmd := exec.Command("umount", mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("umount zfs snapshot: %w\n%s", err, string(out))
	}

	_ = os.Remove(mountPath)
	return zfsDestroy(fullSnap)
}

func zfsDestroy(fullSnap string) error {
	log.Debugf("zfs_destroy", "snapshot=%s", fullSnap)
	cmd := exec.Command("zfs", "destroy", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy: %w\n%s", err, string(out))
	}
	return nil
}
