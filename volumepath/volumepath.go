package volumepath

import "strings"

// VolumeNameFromPath extracts the volume name from a path.
func VolumeNameFromPath(path string) string {
	marker := "/volumes/"
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
	for _, suffix := range []string{"-cold-snap", "-pre-restore"} {
		if strings.HasSuffix(last, suffix) {
			return strings.TrimSuffix(last, suffix)
		}
	}
	return ""
}

// PathBelongsToVolume checks whether a snapshot path belongs to the given volume.
// Unlike VolumeNameFromPath, this handles nested volume names containing "/".
func PathBelongsToVolume(snapPath, volume string) bool {
	marker := "/volumes/"
	if idx := strings.Index(snapPath, marker); idx >= 0 {
		rest := strings.TrimPrefix(snapPath[idx+len(marker):], "/")
		if rest == volume || strings.HasPrefix(rest, volume+"/") {
			return true
		}
	}
	for _, suffix := range []string{"-cold-snap", "-pre-restore"} {
		if strings.HasSuffix(snapPath, "/"+volume+suffix) {
			return true
		}
	}
	return false
}
