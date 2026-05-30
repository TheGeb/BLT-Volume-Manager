package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/example/blt-volume-manager/restic"
)

type SnapshotWithVolume struct {
	restic.Snapshot
	Volume string `json:"volume"`
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := requireVolumeParam(w, r)
	if !ok {
		return
	}

	resp, err := s.snapshotListResponse(volName)
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, resp)
}

func (s *Server) handleSnapshotAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/snapshot/")
	if trimmed == "sizes" {
		s.handleSnapshotSizes(w, r)
		return
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || (parts[1] != "tag" && parts[1] != "restore" && parts[1] != "delete") {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	volName, ok := requireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.volumeManager(volName)

	if parts[1] == "delete" {
		if !requireMethod(w, r, http.MethodDelete) {
			return
		}
		if err := rm.ForgetSnapshot(snapshotID); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "snapshot deleted"})
		return
	}

	if parts[1] == "restore" {
		if !requireMethod(w, r, http.MethodPost) {
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
			if err := rm.SetRestorePoint(snapshotID, volName); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.TagSnapshot(snapshotID, tag); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		resp, err := s.snapshotListResponse(volName)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		resp["status"] = "tag added"
		respondJSON(w, resp)
	case http.MethodDelete:
		if tag == "restore-point" {
			if err := rm.DeleteRestorePoint(volName); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.UntagSnapshot(snapshotID, tag); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		resp, err := s.snapshotListResponse(volName)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		resp["status"] = "tag removed"
		respondJSON(w, resp)
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

	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.volumeManager(volName)

	rawID := parts[0]
	fallbackHash := r.URL.Query().Get("fallbackHash")
	action := parts[1]

	switch action {
	case "ls":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		nodes, err := rm.ListSnapshotFiles(rawID, path)
		if err != nil && fallbackHash != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
				nodes, err = rm.ListSnapshotFiles(snap.ID, path)
			}
		}
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
		data, err := rm.DumpFile(rawID, path)
		if err != nil && fallbackHash != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
				data, err = rm.DumpFile(snap.ID, path)
			}
		}
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)

	case "diff":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if len(parts) < 3 || parts[2] == "" {
			http.Error(w, "missing second snapshot id", http.StatusBadRequest)
			return
		}

		secondID := parts[2]
		diffFallback := r.URL.Query().Get("diffFallbackHash")
		result, err := rm.DiffSnapshots(rawID, secondID)
		if err != nil {
			resolvedFirst, resolvedSecond := rawID, secondID
			if fallbackHash != "" {
				if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
					resolvedFirst = snap.ID
				}
			}
			if diffFallback != "" {
				if snap, lookupErr := rm.FindSnapshotByHash(diffFallback); lookupErr == nil {
					resolvedSecond = snap.ID
				}
			}
			result, err = rm.DiffSnapshots(resolvedFirst, resolvedSecond)
		}
		if err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
		respondJSON(w, result)

	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSnapshotSizes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Volume string   `json:"volume"`
		IDs    []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Volume == "" || len(req.IDs) == 0 {
		http.Error(w, "missing volume or ids", http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(req.Volume)
	result := map[string]int64{}
	for _, id := range req.IDs {
		stats, err := rm.SnapshotStats(id)
		if err != nil {
			continue
		}
		result[id] = stats.TotalSize
	}
	respondJSON(w, result)
}
