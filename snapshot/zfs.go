package snapshot

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func zfsDataset(path string) (string, error) {
	log.Printf("findmnt -T %s -o SOURCE -n", path)
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
	log.Printf("zfs snapshot %s", fullSnap)
	cmd := exec.Command("zfs", "snapshot", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs snapshot: %w\n%s", err, string(out))
	}

	if err := os.MkdirAll(mountPath, 0755); err != nil {
		_ = zfsDestroy(fullSnap)
		return fmt.Errorf("create mount dir: %w", err)
	}

	log.Printf("mount -t zfs %s %s", fullSnap, mountPath)
	cmd = exec.Command("mount", "-t", "zfs", fullSnap, mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = zfsDestroy(fullSnap)
		_ = os.Remove(mountPath)
		return fmt.Errorf("mount zfs snapshot: %w\n%s", err, string(out))
	}

	return nil
}

func zfsRemoveSnapshot(fullSnap, mountPath string) error {
	log.Printf("umount %s", mountPath)
	cmd := exec.Command("umount", mountPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("umount zfs snapshot: %w\n%s", err, string(out))
	}

	_ = os.Remove(mountPath)
	return zfsDestroy(fullSnap)
}

func zfsDestroy(fullSnap string) error {
	log.Printf("zfs destroy %s", fullSnap)
	cmd := exec.Command("zfs", "destroy", fullSnap)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zfs destroy: %w\n%s", err, string(out))
	}
	return nil
}


