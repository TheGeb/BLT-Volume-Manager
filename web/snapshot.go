package web

import (
	"net/http"
	"strings"

	"github.com/example/blt-volume-manager/restic"
)

type SnapshotWithVolume struct {
	restic.Snapshot
	Volume string `json:"volume"`
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	volumeFilter := r.URL.Query().Get("volume")
	if volumeFilter == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(volumeFilter)
	snapshots, err := rm.ListSnapshots()
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	var result []SnapshotWithVolume
	for _, snap := range snapshots {
		result = append(result, SnapshotWithVolume{Snapshot: snap, Volume: volumeFilter})
	}

	respondJSON(w, result)
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
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.volumeManager(volName)

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
			if err := rm.RestoreSnapshot(snapshotID, target); err != nil {
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
			if err := rm.SetRestorePoint(snapshotID); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.TagSnapshot(snapshotID, tag); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		respondJSON(w, map[string]string{"status": "tag added"})
	case http.MethodDelete:
		if err := rm.UntagSnapshot(snapshotID, tag); err != nil {
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
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.volumeManager(volName)

	switch action {
	case "ls":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		nodes, err := rm.ListSnapshotFiles(snapshotID, path)
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
		data, err := rm.DumpFile(snapshotID, path)
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
		result, err := rm.DiffSnapshots(snapshotID, parts[2])
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, result)

	default:
		http.NotFound(w, r)
	}
}
