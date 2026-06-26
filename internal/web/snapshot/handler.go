package snapshot

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func SnapshotRouter(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/snapshot/")
	if trimmed == "sizes" {
		SnapshotSizes(s, w, r)
		return
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || (parts[1] != "tag" && parts[1] != "restore" && parts[1] != "delete") {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.VolumeManager(volName)
	store, _ := s.MetadataStore()

	if parts[1] == "delete" {
		if !server.RequireMethod(w, r, http.MethodDelete) {
			return
		}
		if err := rm.ForgetSnapshot(snapshotID); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, map[string]string{"status": "snapshot deleted"})
		return
	}

	if parts[1] == "restore" {
		if !server.RequireMethod(w, r, http.MethodPost) {
			return
		}
		target := r.URL.Query().Get("path")
		if target == "" {
			http.Error(w, "missing path query parameter", http.StatusBadRequest)
			return
		}
		s.AddWork()
		go func() {
			defer s.DoneWork()
			if err := rm.RestoreSnapshot(snapshotID, target); err != nil {
				log.Errorf("restore_failed", err, "snapshot=%s target=%s", snapshotID, target)
			} else {
				log.Info("restore_ok")
			}
		}()
		server.RespondJSON(w, map[string]string{"status": "restore started – see server logs for results"})
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
			if err := metadata.SetRestorePoint(store, volName, snapshotID); err != nil {
				server.RespondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.TagSnapshot(snapshotID, tag); err != nil {
				server.RespondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		resp, err := SnapshotListResponse(s, volName, nil, nil, 0, 0)
		if err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		resp["status"] = "tag added"
		server.RespondJSON(w, resp)
	case http.MethodDelete:
		if tag == "restore-point" {
			if err := metadata.DeleteRestorePoint(store, volName); err != nil {
				server.RespondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.UntagSnapshot(snapshotID, tag); err != nil {
				server.RespondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		resp, err := SnapshotListResponse(s, volName, nil, nil, 0, 0)
		if err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		resp["status"] = "tag removed"
		server.RespondJSON(w, resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func SnapshotFileRouter(s *server.Server, w http.ResponseWriter, r *http.Request) {
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
	rm := s.VolumeManager(volName)

	rawID := parts[0]
	version := r.URL.Query().Get("tag")
	fallbackHash := r.URL.Query().Get("fallbackHash")
	action := parts[1]

	resolve := func(id, ver, fallback string) string {
		if ver != "" {
			if resolved, err := FindSnapshotByVersion(rm, ver); err == nil {
				return resolved
			}
		}
		return id
	}

	rawID = resolve(rawID, version, fallbackHash)

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
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, nodes)

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
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		//nolint:gosec // Intentional dump of restic file contents as text/plain with
		// X-Content-Type-Options: nosniff
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
		diffVersion := r.URL.Query().Get("diffTag")
		diffFallback := r.URL.Query().Get("diffFallbackHash")

		secondID = resolve(secondID, diffVersion, diffFallback)

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
			server.RespondError(w, err, http.StatusNotFound)
			return
		}
		server.RespondJSON(w, result)

	default:
		http.NotFound(w, r)
	}
}

func SnapshotSizes(s *server.Server, w http.ResponseWriter, r *http.Request) {
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

	rm := s.VolumeManager(req.Volume)
	result := map[string]int64{}
	for _, id := range req.IDs {
		stats, err := rm.SnapshotStats(id)
		if err != nil {
			continue
		}
		result[id] = stats.TotalSize
	}
	server.RespondJSON(w, result)
}

func BatchDeleteSnapshots(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
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

	rm := s.VolumeManager(req.Volume)
	resp := batchDeleteResponse{Errors: []batchDeleteError{}}
	for _, id := range req.IDs {
		if err := rm.ForgetSnapshot(id); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, batchDeleteError{ID: id, Error: err.Error()})
		} else {
			resp.Deleted++
		}
	}
	server.RespondJSON(w, resp)
}
