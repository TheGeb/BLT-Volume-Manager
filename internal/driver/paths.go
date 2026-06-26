package driver

import (
	"path/filepath"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver/snapshot"
)

const (
	VolumesDir       = "volumes"
	OwnersDir        = "owners"
	SnapshotsDir     = "snapshots"
	VolumeConfigFile = "volume.json"
)

func VolumePath(root, name string) string {
	return filepath.Join(root, VolumesDir, name)
}

func VolumeNameFromPath(path string) string {
	marker := "/" + VolumesDir + "/"
	if idx := strings.Index(path, marker); idx >= 0 {
		rest := strings.TrimPrefix(path[idx+len(marker):], "/")
		if parts := strings.SplitN(rest, "/", 2); len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	for _, suffix := range []string{snapshot.ColdSnapSuffix, snapshot.PreRestoreSuffix} {
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
	for _, suffix := range []string{snapshot.ColdSnapSuffix, snapshot.PreRestoreSuffix} {
		if strings.HasSuffix(snapPath, "/"+volume+suffix) {
			return true
		}
	}
	return false
}
