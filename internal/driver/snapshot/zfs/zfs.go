package zfs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver/snapshot"
)

type zfsSnapshotter struct{}

func init() {
	snapshot.Register(&zfsSnapshotter{})
}

func (z *zfsSnapshotter) Type() snapshot.Type { return snapshot.TypeZFS }

func (z *zfsSnapshotter) MatchFSType(fsType string) bool {
	return fsType == "zfs"
}

func (z *zfsSnapshotter) Init(path string, opts map[string]string) error {
	parentDataset := snapshot.RootDataset()
	if p, ok := opts["zfs-pool"]; ok && p != "" {
		parentDataset = p
	}
	if parentDataset == "" {
		return fmt.Errorf("no ZFS parent dataset configured; set zfs-pool option or run on ZFS filesystem")
	}

	name := filepath.Base(path)
	full := parentDataset + "/" + name

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}

	app.Debugf("zfs_create", "path=%s dataset=%s", path, full)
	cmd := exec.Command("zfs", "create", "-o", "mountpoint="+path, full)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs create: %w\n%s", err, string(out))
	}
	return nil
}

func (z *zfsSnapshotter) Destroy(path string) error {
	dataset, err := snapshot.ZFSDataset(path)
	if err != nil {
		return err
	}
	app.Debugf("zfs_destroy_dataset", "dataset=%s", dataset)
	cmd := exec.Command("zfs", "destroy", "-r", dataset)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy: %w\n%s", err, string(out))
	}
	return nil
}

func (z *zfsSnapshotter) CreateSnapshot(volPath, accessPath, volName string, info *snapshot.SnapInfo) error {
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

func (z *zfsSnapshotter) RemoveSnapshot(info *snapshot.SnapInfo) error {
	return zfsRemoveSnapshot(info.ZfsSnap, info.AccessPath)
}

func zfsCreateSnapshot(fullSnap, mountPath string) error {
	app.Debugf("zfs_snapshot_create", "snapshot=%s", fullSnap)
	cmd := exec.Command("zfs", "snapshot", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs snapshot: %w\n%s", err, string(out))
	}

	if err := os.MkdirAll(mountPath, 0o755); err != nil {
		_ = zfsDestroy(fullSnap)
		return fmt.Errorf("create mount dir: %w", err)
	}

	app.Debugf("zfs_mount_snapshot", "snapshot=%s path=%s", fullSnap, mountPath)
	cmd = exec.Command("mount", "-t", "zfs", fullSnap, mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = zfsDestroy(fullSnap)
		_ = os.Remove(mountPath)
		return fmt.Errorf("mount zfs snapshot: %w\n%s", err, string(out))
	}

	return nil
}

func zfsRemoveSnapshot(fullSnap, mountPath string) error {
	app.Debugf("zfs_unmount", "path=%s", mountPath)
	cmd := exec.Command("umount", mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("umount zfs snapshot: %w\n%s", err, string(out))
	}

	_ = os.Remove(mountPath)
	return zfsDestroy(fullSnap)
}

func zfsDestroy(fullSnap string) error {
	app.Debugf("zfs_destroy", "snapshot=%s", fullSnap)
	cmd := exec.Command("zfs", "destroy", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy: %w\n%s", err, string(out))
	}
	return nil
}
