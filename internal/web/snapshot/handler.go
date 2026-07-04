package snapshot

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

const (
	snapshotOpTag     = "tag"
	snapshotOpRestore = "restore"
	snapshotOpDelete  = "delete"
)

func SnapshotRouter(s *server.Service, w http.ResponseWriter, r *http.Request) {
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
	if len(parts) != 2 || (parts[1] != snapshotOpTag && parts[1] != snapshotOpRestore && parts[1] != snapshotOpDelete) {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.ResticManager(volName)

	switch parts[1] {
	case snapshotOpDelete:
		deleteSnapshot(s, w, r, rm, snapshotID)
	case snapshotOpRestore:
		restoreSnapshot(s, w, r, rm, snapshotID)
	case snapshotOpTag:
		tagSnapshot(s, w, r, rm, snapshotID)
	}
}

func SnapshotFileRouter(s *server.Service, w http.ResponseWriter, r *http.Request) {
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

	rawID = ResolveSnapshotID(rm, rawID, version, volName)

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
		secondID = ResolveSnapshotID(rm, secondID, diffVersion, volName)
		diffSnapshots(s, w, r, rm, rawID, secondID, fallbackHash, diffFallback)
	default:
		http.NotFound(w, r)
	}
}

func ListSnapshots(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	opts, filter, offset, limit := ParseSnapshotListOpts(r)
	resp, err := BuildSnapshotListResponse(s, volName, opts, filter, offset, limit)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, resp)
}

func SnapshotSizes(s *server.Service, w http.ResponseWriter, r *http.Request) {
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

	rm := s.ResticManager(req.Volume)
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

func ListSnapshotHosts(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	rm := s.ResticManager(volName)
	latest := 1
	if l := r.URL.Query().Get("latest"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			latest = n
		}
	}

	hosts, err := rm.SnapshotHosts(latest)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, hosts)
}

func deleteSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
	if !server.RequireMethod(w, r, http.MethodDelete) {
		return
	}
	if err := rm.ForgetSnapshots(snapshotID); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, server.StatusResponse{Status: "snapshot deleted"})
}

func BatchDeleteSnapshots(s *server.Service, w http.ResponseWriter, r *http.Request) {
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

	rm := s.ResticManager(req.Volume)
	resp := batchDeleteResponse{Errors: []batchDeleteError{}}
	for _, id := range req.IDs {
		if err := rm.ForgetSnapshots(id); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, batchDeleteError{ID: id, Error: err.Error()})
		} else {
			resp.Deleted++
		}
	}
	server.RespondJSON(w, resp)
}

func restoreSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
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
	server.RespondJSON(w, server.StatusResponse{Status: "restore started – see server logs for results"})
}

func tagSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		addTag(s, w, r, rm, snapshotID, tag)
	case http.MethodDelete:
		removeTag(s, w, r, rm, snapshotID, tag)
	default:
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
	}
}

func addTag(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID, tag string) {
	volName := r.URL.Query().Get("volume")
	if err := SetSnapshotTag(s, rm, volName, snapshotID, tag); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp, err := BuildSnapshotListResponse(s, volName, nil, nil, 0, 0)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp.Status = "tag added"
	server.RespondJSON(w, resp)
}

func removeTag(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID, tag string) {
	volName := r.URL.Query().Get("volume")
	if err := DeleteSnapshotTag(s, rm, volName, snapshotID, tag); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp, err := BuildSnapshotListResponse(s, volName, nil, nil, 0, 0)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp.Status = "tag removed"
	server.RespondJSON(w, resp)
}

func listSnapshotFiles(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	nodes, err := withFallback(rm, rawID, fallbackHash, func(id string) ([]restic.FileNode, error) {
		return rm.ListSnapshotFiles(id, path)
	})
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, nodes)
}

func dumpSnapshotFile(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	data, err := withFallback(rm, rawID, fallbackHash, func(id string) ([]byte, error) {
		return rm.DumpFile(id, path)
	})
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data) //nolint:gosec // data is served as text/plain with nosniff
}

func diffSnapshots(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, secondID, fallbackHash, diffFallback string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
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
}
