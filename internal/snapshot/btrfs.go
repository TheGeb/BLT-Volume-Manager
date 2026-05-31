package snapshot

import (
	"fmt"
	"os/exec"

	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
)

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
