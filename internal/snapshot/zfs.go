package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
)

type zfsSnapshotter struct{}

func init() {
	register(&zfsSnapshotter{})
}

func (z *zfsSnapshotter) Type() Type { return TypeZFS }

func (z *zfsSnapshotter) MatchFSType(fsType string) bool {
	return fsType == "zfs"
}

func (z *zfsSnapshotter) CreateSnapshot(volPath, accessPath, volName string, info *SnapInfo) error {
	dataset, err := zfsDataset(volPath)
	if err != nil {
		return fmt.Errorf("resolve zfs dataset: %w", err)
	}
	snapName := filepath.Base(accessPath)
	fullSnap := dataset + "@" + snapName
	if err := zfsCreateSnapshot(fullSnap, accessPath); err != nil {
		return err
	}
	info.zfsSnap = fullSnap
	return nil
}

func (z *zfsSnapshotter) RemoveSnapshot(info *SnapInfo) error {
	return zfsRemoveSnapshot(info.zfsSnap, info.AccessPath)
}

func zfsDataset(path string) (string, error) {
	applog.Debugf("findmnt_lookup", "path=%s", path)
	cmd := exec.Command("findmnt", "-T", path, "-o", "SOURCE", "-n")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("findmnt for %s: %w", path, err)
	}
	source := strings.TrimSpace(string(out))
	if source == "" {
		return "", fmt.Errorf("no mount source for %s", path)
	}
	if !strings.Contains(source, "/") {
		return "", fmt.Errorf("mount source %q does not look like a ZFS dataset", source)
	}
	return source, nil
}

func zfsCreateSnapshot(fullSnap, mountPath string) error {
	applog.Debugf("zfs_snapshot_create", "snapshot=%s", fullSnap)
	cmd := exec.Command("zfs", "snapshot", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs snapshot: %w\n%s", err, string(out))
	}

	if err := os.MkdirAll(mountPath, 0o755); err != nil {
		_ = zfsDestroy(fullSnap)
		return fmt.Errorf("create mount dir: %w", err)
	}

	applog.Debugf("zfs_mount_snapshot", "snapshot=%s path=%s", fullSnap, mountPath)
	cmd = exec.Command("mount", "-t", "zfs", fullSnap, mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = zfsDestroy(fullSnap)
		_ = os.Remove(mountPath)
		return fmt.Errorf("mount zfs snapshot: %w\n%s", err, string(out))
	}

	return nil
}

func zfsRemoveSnapshot(fullSnap, mountPath string) error {
	applog.Debugf("zfs_unmount", "path=%s", mountPath)
	cmd := exec.Command("umount", mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("umount zfs snapshot: %w\n%s", err, string(out))
	}

	_ = os.Remove(mountPath)
	return zfsDestroy(fullSnap)
}

func zfsDestroy(fullSnap string) error {
	applog.Debugf("zfs_destroy", "snapshot=%s", fullSnap)
	cmd := exec.Command("zfs", "destroy", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy: %w\n%s", err, string(out))
	}
	return nil
}
