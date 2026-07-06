package volume

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type setRestorePointRequest struct {
	SnapshotID string `json:"snapshot_id"`
}

func RestorePointRouter(s *server.Service, w http.ResponseWriter, r *http.Request, volumeName string) {
	switch r.Method {
	case http.MethodPut:
		setRestorePoint(s, w, r, volumeName)
	case http.MethodDelete:
		deleteRestorePoint(s, w, r, volumeName)
	default:
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
	}
}

func setRestorePoint(s *server.Service, w http.ResponseWriter, r *http.Request, volumeName string) {
	var req setRestorePointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}
	if req.SnapshotID == "" {
		server.RespondError(w, fmt.Errorf("snapshot_id is required"), http.StatusBadRequest)
		return
	}
	if err := s.SetRestorePoint(volumeName, req.SnapshotID); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, server.StatusResponse{Status: fmt.Sprintf("Restore point set to %q for volume %q", req.SnapshotID, volumeName)})
}

func deleteRestorePoint(s *server.Service, w http.ResponseWriter, r *http.Request, volumeName string) {
	if err := s.DeleteRestorePoint(volumeName); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, server.StatusResponse{Status: fmt.Sprintf("Restore point cleared for volume %q", volumeName)})
}
