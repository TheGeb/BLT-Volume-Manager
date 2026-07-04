package driver

import (
	"path/filepath"
	"strings"

	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"
)

const (
	VolumesDir   = "volumes"
	SnapshotsDir = "snapshots"
)

func VolumePath(root, name string) string {
	return filepath.Join(root, VolumesDir, name)
}

func VolumeNameFromPath(path string) string {
	cleanPath := filepath.Clean(path)
	marker := "/" + VolumesDir + "/"
	if idx := strings.Index(cleanPath, marker); idx >= 0 {
		rest := strings.TrimPrefix(cleanPath[idx+len(marker):], "/")
		if parts := strings.SplitN(rest, "/", 2); len(parts) > 0 && parts[0] != "" {
			if strings.Contains(parts[0], "..") || strings.Contains(parts[0], "/") {
				return ""
			}
			return parts[0]
		}
	}

	parts := strings.Split(strings.Trim(cleanPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	for _, suffix := range []string{snapshot.ColdSuffix, snapshot.PreRestoreSuffix} {
		if strings.HasSuffix(last, suffix) {
			return strings.TrimSuffix(last, suffix)
		}
	}
	return ""
}

func PathBelongsToVolume(snapPath, volume string) bool {
	marker := "/" + VolumesDir + "/"
	if idx := strings.Index(snapPath, marker); idx >= 0 {
		rest := strings.TrimPrefix(snapPath[idx+len(marker):], "/")
		if rest == volume || strings.HasPrefix(rest, volume+"/") {
			return true
		}
	}
	for _, suffix := range []string{snapshot.ColdSuffix, snapshot.PreRestoreSuffix} {
		if strings.HasSuffix(snapPath, "/"+volume+suffix) {
			return true
		}
	}
	return false
}
