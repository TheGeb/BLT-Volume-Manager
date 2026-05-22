package web

import (
	"net/http"
	"strings"

	"github.com/example/blt-volume-manager/restic"
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
	if len(parts) != 2 || (parts[1] != "tag" && parts[1] != "restore") {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]

	if parts[1] == "restore" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		target := r.URL.Query().Get("path")
		if target == "" {
			http.Error(w, "missing path query parameter", http.StatusBadRequest)
			return
		}
		go func() {
			if err := s.restic.RestoreSnapshot(snapshotID, target); err != nil {
				logInfo("restore_failed: " + err.Error())
			} else {
				logInfo("restore_ok")
			}
		}()
		respondJSON(w, map[string]string{"status": "restore started – see server logs for results"})
		return
	}

	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if tag == "restore-point" {
			if err := s.restic.SetRestorePoint(snapshotID); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := s.restic.TagSnapshot(snapshotID, tag); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
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

func (s *Server) handleSnapshotView(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot-view/") {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/snapshot-view/")
	parts := strings.Split(rest, "/")

	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	action := parts[1]

	switch action {
	case "ls":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		nodes, err := s.restic.ListSnapshotFiles(snapshotID, path)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, nodes)

	case "dump":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}
		data, err := s.restic.DumpFile(snapshotID, path)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)

	case "diff":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if len(parts) < 3 || parts[2] == "" {
			http.Error(w, "missing second snapshot id", http.StatusBadRequest)
			return
		}
		result, err := s.restic.DiffSnapshots(snapshotID, parts[2])
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, result)

	default:
		http.NotFound(w, r)
	}
}
