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

func snapshotMatchesVolume(snapshot restic.Snapshot, volume string) bool {
	for _, path := range snapshot.Paths {
		if volumeNameFromPath(path) == volume {
			return true
		}
	}
	return false
}

func respondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
