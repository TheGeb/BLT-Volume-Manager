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

type WithVolume struct {
	restic.Snapshot
	Volume string `json:"volume"`
}

type SnapshotListResponse struct {
	Snapshots      []WithVolume `json:"snapshots"`
	RestorePointID string       `json:"restorePointID"`
	HasMore        bool         `json:"hasMore"`
	Status         string       `json:"status,omitempty"`
}

type snapshotNotFoundError struct {
	version string
}

func (e *snapshotNotFoundError) Error() string {
	return "snapshot not found for version " + e.version
}

type batchDeleteError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

type batchDeleteResponse struct {
	Deleted int                `json:"deleted"`
	Failed  int                `json:"failed"`
	Errors  []batchDeleteError `json:"errors"`
}

// --- Lookup ---

func FindSnapshotByVersion(rm *restic.Manager, version, volumeName string) (string, error) {
	tag := version
	if !strings.HasPrefix(version, "v") {
		tag = "v" + version
	}
	snapshots, err := rm.ListSnapshotsWithOpts(&restic.ListSnapshotsOpts{Tags: []string{tag}})
	if err != nil {
		return "", err
	}
	if len(snapshots) == 0 {
		return "", &snapshotNotFoundError{version: version}
	}
	newest := snapshots[0]
	if len(snapshots) > 1 {
		log.Warnf("multiple_snapshots_found", "version=%s count=%d volume=%s", version, len(snapshots), volumeName)
	}
	for _, s := range snapshots[1:] {
		if s.Time.After(newest.Time) {
			newest = s
		}
	}
	return newest.ID, nil
}

// --- List ---

func BuildSnapshotListResponse(s *server.Service, volName string, opts *restic.ListSnapshotsOpts, filter *SnapshotFilter, offset, limit int) (*SnapshotListResponse, error) {
	rm := s.ResticManager(volName)
	snaps, err := rm.ListSnapshotsWithOpts(opts)
	if err != nil {
		return nil, err
	}

	snaps = ApplySnapshotFilter(snaps, filter)

	rawLen := len(snaps)
	hasMore := false
	if limit > 0 {
		switch {
		case offset+limit <= rawLen:
			hasMore = rawLen > offset+limit
			snaps = snaps[offset : offset+limit]
		case offset < rawLen:
			snaps = snaps[offset:]
		default:
			snaps = nil
		}
	} else if offset > 0 && offset < rawLen {
		snaps = snaps[offset:]
	}

	restorePointID := ""
	if id, err := s.FindRestorePointByName(volName); err == nil {
		restorePointID = id
	}

	result := make([]WithVolume, 0, len(snaps))
	for _, snap := range snaps {
		fullHash := rm.GenerateHash(snap)
		snap.FallbackHash = fullHash[:len(snap.ShortID)]
		result = append(result, WithVolume{Snapshot: snap, Volume: volName})
	}

	return &SnapshotListResponse{
		Snapshots:      result,
		RestorePointID: restorePointID,
		HasMore:        hasMore,
	}, nil
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

// --- Delete ---

func handleDeleteSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
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

// --- Restore ---

func handleRestoreSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
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

// --- Tag ---

func handleTagSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		handleAddTag(s, w, r, rm, snapshotID, tag)
	case http.MethodDelete:
		handleRemoveTag(s, w, r, rm, snapshotID, tag)
	default:
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
	}
}

func handleAddTag(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID, tag string) {
	if tag == "restore-point" {
		if err := s.SetRestorePoint(r.URL.Query().Get("volume"), snapshotID); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	} else {
		if err := rm.TagSnapshot(snapshotID, tag); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	}
	resp, err := BuildSnapshotListResponse(s, r.URL.Query().Get("volume"), nil, nil, 0, 0)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp.Status = "tag added"
	server.RespondJSON(w, resp)
}

func handleRemoveTag(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID, tag string) {
	if tag == "restore-point" {
		if err := s.DeleteRestorePoint(r.URL.Query().Get("volume")); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	} else {
		if err := rm.UntagSnapshot(snapshotID, tag); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	}
	resp, err := BuildSnapshotListResponse(s, r.URL.Query().Get("volume"), nil, nil, 0, 0)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp.Status = "tag removed"
	server.RespondJSON(w, resp)
}

// --- Contents ---

func handleListSnapshotFiles(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
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
}

func handleDumpSnapshotFile(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
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
	_, _ = w.Write(data)
}

func handleDiffSnapshots(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, secondID, fallbackHash, diffFallback string) {
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
