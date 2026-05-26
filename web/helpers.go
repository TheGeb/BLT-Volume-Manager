package web

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/example/blt-volume-manager/restic"
)

func mustHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

func volumeNameFromPath(path string) string {
	marker := "/volumes/"
	idx := strings.Index(path, marker)
	if idx >= 0 {
		subpath := strings.TrimPrefix(path[idx+len(marker):], "/")
		parts := strings.Split(subpath, "/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// pathBelongsToVolume checks whether a snapshot path belongs to the given volume.
// Unlike volumeNameFromPath, this handles nested volume names containing "/".
func pathBelongsToVolume(snapPath, volume string) bool {
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

func snapshotMatchesVolume(snapshot restic.Snapshot, volume string) bool {
	for _, path := range snapshot.Paths {
		if pathBelongsToVolume(path, volume) {
			return true
		}
	}
	return false
}

func respondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
		logError("request_error", err)
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
