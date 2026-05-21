package snapshot

import (
	"fmt"
	"os/exec"
)

func btrfsCreate(source, dest string) error {
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", source, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume snapshot: %w\n%s", err, string(out))
	}
	return nil
}

func btrfsRemove(path string) error {
	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete: %w\n%s", err, string(out))
	}
	return nil
}
