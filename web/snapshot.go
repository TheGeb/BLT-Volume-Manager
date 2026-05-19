package web

import (
	"net/http"
	"strings"

	"github.com/example/docker-s3-volume-plugin/restic"
)

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	volumeFilter := r.URL.Query().Get("volume")

	snapshots, err := s.restic.ListSnapshots()
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	if volumeFilter != "" {
		filtered := make([]restic.Snapshot, 0, len(snapshots))
		for _, snap := range snapshots {
			if snapshotMatchesVolume(snap, volumeFilter) {
				filtered = append(filtered, snap)
			}
		}
		snapshots = filtered
	}

	respondJSON(w, snapshots)
}

func (s *Server) handleSnapshotAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/snapshot/"), "/")
	if len(parts) != 2 || parts[1] != "tag" {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if err := s.restic.TagSnapshot(snapshotID, tag); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "tag added"})
	case http.MethodDelete:
		if err := s.restic.UntagSnapshot(snapshotID, tag); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "tag removed"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
