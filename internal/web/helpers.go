package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func requireVolumeParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	vol := r.URL.Query().Get("volume")
	if vol == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return "", false
	}
	return vol, true
}

func (s *Server) snapshotListResponse(volName string, opts *restic.ListSnapshotsOpts, offset, limit int) (map[string]any, error) {
	rm := s.volumeManager(volName)
	snaps, err := rm.ListSnapshotsWithOpts(opts)
	if err != nil {
		return nil, err
	}

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
	if id, err := rm.FindRestorePointByName(volName); err == nil {
		restorePointID = id
	}

	result := make([]SnapshotWithVolume, 0, len(snaps))
	for _, snap := range snaps {
		fullHash := rm.GenerateHash(snap)
		snap.FallbackHash = fullHash[:len(snap.ShortID)]
		result = append(result, SnapshotWithVolume{Snapshot: snap, Volume: volName})
	}

	return map[string]any{
		"snapshots":      result,
		"restorePointID": restorePointID,
		"hasMore":        hasMore,
	}, nil
}

func parseSnapshotListOpts(r *http.Request) (*restic.ListSnapshotsOpts, int, int) {
	hosts := r.URL.Query()["host"]
	latestStr := r.URL.Query().Get("latest")
	latest := 0
	if latestStr != "" {
		if n, err := strconv.Atoi(latestStr); err == nil && n > 0 {
			latest = n
		}
	}
	tags := r.URL.Query()["tag"]

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	if limit > 0 {
		latest = offset + limit + 1
	}

	var opts *restic.ListSnapshotsOpts
	if len(hosts) > 0 || latest > 0 || len(tags) > 0 {
		opts = &restic.ListSnapshotsOpts{Hosts: hosts, Latest: latest, Tags: tags}
	}
	return opts, offset, limit
}

func respondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
		logError("request_error", err)
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logError("encode_error_response_failed", err)
	}
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logError("encode_response_failed", err)
	}
}
