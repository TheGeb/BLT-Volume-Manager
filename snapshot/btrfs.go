package snapshot

import (
	"fmt"
	"log"
	"os/exec"
)

func btrfsCreate(source, dest string) error {
	log.Printf("btrfs subvolume snapshot -r %s %s", source, dest)
	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", source, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume snapshot: %w\n%s", err, string(out))
	}
	return nil
}

func btrfsRemove(path string) error {
	log.Printf("btrfs subvolume delete %s", path)
	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume delete: %w\n%s", err, string(out))
	}
	return nil
}
