package web

import (
	"encoding/json"
	"net/http"
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

func (s *Server) snapshotListResponse(volName string) (map[string]any, error) {
	rm := s.volumeManager(volName)
	snaps, err := rm.ListSnapshots()
	if err != nil {
		return nil, err
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
	}, nil
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
