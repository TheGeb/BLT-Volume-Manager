package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func SnapshotRouter(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		server.RespondError(w, server.ErrNotFound, http.StatusNotFound)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/snapshot/")
	if trimmed == "sizes" {
		SnapshotSizes(s, w, r)
		return
	}

	server.RespondError(w, server.ErrNotFound, http.StatusNotFound)
}

func SnapshotFileRouter(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot-view/") {
		server.RespondError(w, server.ErrNotFound, http.StatusNotFound)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/snapshot-view/")
	parts := strings.Split(rest, "/")

	if len(parts) < 2 {
		server.RespondError(w, server.ErrNotFound, http.StatusNotFound)
		return
	}

	volName := r.URL.Query().Get("volume")
	if volName == "" {
		server.RespondError(w, server.ErrMissingVolume, http.StatusBadRequest)
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
			server.RespondError(w, server.ErrNotFound, http.StatusNotFound)
			return
		}
		secondID := parts[2]
		diffVersion := r.URL.Query().Get("diffTag")
		diffFallback := r.URL.Query().Get("diffFallbackHash")
		secondID = ResolveSnapshotID(r.Context(), rm, secondID, diffVersion, volName)
		diffSnapshots(s, w, r, rm, rawID, secondID, fallbackHash, diffFallback)
	default:
		server.RespondError(w, server.ErrNotFound, http.StatusNotFound)
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
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Volume string   `json:"volume"`
		IDs    []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid JSON"), http.StatusBadRequest)
		return
	}
	if req.Volume == "" || len(req.IDs) == 0 {
		server.RespondError(w, fmt.Errorf("missing volume or ids"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	rm := s.ResticManager(req.Volume)
	result := map[string]int64{}
	var lastErr error
	for _, id := range req.IDs {
		stats, err := rm.SnapshotStats(ctx, id)
		if err != nil {
			lastErr = err
			continue
		}
		result[id] = stats.TotalSize
	}
	if len(result) == 0 && lastErr != nil {
		server.RespondError(w, lastErr, http.StatusInternalServerError)
		return
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
		server.RespondError(w, fmt.Errorf("invalid JSON"), http.StatusBadRequest)
		return
	}
	if req.Volume == "" || len(req.IDs) == 0 {
		server.RespondError(w, fmt.Errorf("missing volume or ids"), http.StatusBadRequest)
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
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
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
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	path := r.URL.Query().Get("path")
	if path == "" {
		server.RespondError(w, fmt.Errorf("missing path"), http.StatusBadRequest)
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
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
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
