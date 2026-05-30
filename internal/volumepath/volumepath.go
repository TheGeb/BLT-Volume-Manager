package volumepath

import (
	"path/filepath"
	"strings"

	"github.com/example/blt-volume-manager/internal/constants"
)

// VolumePath returns the filesystem path for a volume's data directory.
func VolumePath(root, name string) string {
	return filepath.Join(root, constants.VolumesDir, name)
}

// VolumeNameFromPath extracts the volume name from a path.
func VolumeNameFromPath(path string) string {
	marker := "/" + constants.VolumesDir + "/"
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
	for _, suffix := range []string{constants.ColdSnapSuffix, constants.PreRestoreSuffix} {
		if strings.HasSuffix(last, suffix) {
			return strings.TrimSuffix(last, suffix)
		}
	}
	return ""
}

// PathBelongsToVolume checks whether a snapshot path belongs to the given volume.
// Unlike VolumeNameFromPath, this handles nested volume names containing "/".
func PathBelongsToVolume(snapPath, volume string) bool {
	marker := "/" + constants.VolumesDir + "/"
	if idx := strings.Index(snapPath, marker); idx >= 0 {
		rest := strings.TrimPrefix(snapPath[idx+len(marker):], "/")
		if rest == volume || strings.HasPrefix(rest, volume+"/") {
			return true
		}
	}
	for _, suffix := range []string{constants.ColdSnapSuffix, constants.PreRestoreSuffix} {
		if strings.HasSuffix(snapPath, "/"+volume+suffix) {
			return true
		}
	}
	return false
}
