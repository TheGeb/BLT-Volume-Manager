package snapshot

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func SnapshotRouter(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/snapshot/")
	if trimmed == "sizes" {
		SnapshotSizes(s, w, r)
		return
	}

	http.NotFound(w, r)
}

func SnapshotFileRouter(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, server.ErrMissingVolume.Error(), http.StatusBadRequest)
		return
	}
	rm := s.ResticManager(volName)

	rawID := parts[0]
	version := r.URL.Query().Get("tag")
	fallbackHash := r.URL.Query().Get("fallbackHash")
	action := parts[1]

	rawID = ResolveSnapshotID(r.Context(), rm, rawID, version, volName)

	switch action {
	case "ls":
		listSnapshotFiles(s, w, r, rm, rawID, fallbackHash)
	case "dump":
		dumpSnapshotFile(s, w, r, rm, rawID, fallbackHash)
	case "diff":
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}
		secondID := parts[2]
		diffVersion := r.URL.Query().Get("diffTag")
		diffFallback := r.URL.Query().Get("diffFallbackHash")
		secondID = ResolveSnapshotID(r.Context(), rm, secondID, diffVersion, volName)
		diffSnapshots(s, w, r, rm, rawID, secondID, fallbackHash, diffFallback)
	default:
		http.NotFound(w, r)
	}
}

func ListSnapshots(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	opts, filter, offset, limit := ParseSnapshotListOpts(r)
	resp, err := BuildSnapshotListResponse(ctx, s, volName, opts, filter, offset, limit)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, resp)
}

func SnapshotSizes(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
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

	ctx := r.Context()
	rm := s.ResticManager(req.Volume)
	result := map[string]int64{}
	for _, id := range req.IDs {
		stats, err := rm.SnapshotStats(ctx, id)
		if err != nil {
			continue
		}
		result[id] = stats.TotalSize
	}
	server.RespondJSON(w, result)
}

func ListSnapshotHosts(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	rm := s.ResticManager(volName)
	latest := 1
	if l := r.URL.Query().Get("latest"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			latest = n
		}
	}

	hosts, err := rm.SnapshotHosts(ctx, latest)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, hosts)
}

func BatchDeleteSnapshots(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
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

	ctx := r.Context()
	rm := s.ResticManager(req.Volume)
	resp := batchDeleteResponse{Errors: []batchDeleteError{}}
	for _, id := range req.IDs {
		if err := rm.ForgetSnapshots(ctx, id); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, batchDeleteError{ID: id, Error: err.Error()})
		} else {
			resp.Deleted++
		}
	}
	server.RespondJSON(w, resp)
}

func listSnapshotFiles(s *server.BLTService, w http.ResponseWriter, r *http.Request, rm ResticManager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	path := r.URL.Query().Get("path")
	nodes, err := withFallback(ctx, rm, rawID, fallbackHash, func(ctx context.Context, id string) ([]restic.FileNode, error) {
		return rm.ListSnapshotFiles(ctx, id, path)
	})
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, nodes)
}

func dumpSnapshotFile(s *server.BLTService, w http.ResponseWriter, r *http.Request, rm ResticManager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	data, err := withFallback(ctx, rm, rawID, fallbackHash, func(ctx context.Context, id string) ([]byte, error) {
		return rm.DumpFile(ctx, id, path)
	})
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data) //nolint:gosec // data is served as text/plain with nosniff
}

func diffSnapshots(s *server.BLTService, w http.ResponseWriter, r *http.Request, rm ResticManager, rawID, secondID, fallbackHash, diffFallback string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	result, err := rm.DiffSnapshots(ctx, rawID, secondID)
	if err != nil {
		resolvedFirst, resolvedSecond := rawID, secondID
		if fallbackHash != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(ctx, fallbackHash); lookupErr == nil {
				resolvedFirst = snap.ID
			}
		}
		if diffFallback != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(ctx, diffFallback); lookupErr == nil {
				resolvedSecond = snap.ID
			}
		}
		result, err = rm.DiffSnapshots(ctx, resolvedFirst, resolvedSecond)
	}
	if err != nil {
		server.RespondError(w, err, http.StatusNotFound)
		return
	}
	server.RespondJSON(w, result)
}
