package volume

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/owner"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type VolumeListResponse struct {
	Volumes []string `json:"volumes"`
}

const OwnersDir = "owners"

func ListVolumes(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	volumes := s.VolumeNames(r.Context())
	if volumes == nil {
		volumes = []string{}
	}
	server.RespondJSON(w, VolumeListResponse{Volumes: volumes})
}

func VolumeRouter(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	escapedPath := r.URL.EscapedPath()
	if !strings.HasPrefix(escapedPath, "/api/volume/") {
		http.NotFound(w, r)
		return
	}

	rawPath := strings.TrimPrefix(escapedPath, "/api/volume/")

	if strings.HasSuffix(rawPath, "/copy") {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/copy"))
		if err != nil || !validVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		CopyVolume(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(rawPath, "/rename") {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/rename"))
		if err != nil || !validVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		RenameVolume(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(rawPath, "/"+OwnersDir) {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/"+OwnersDir))
		if err != nil || !validVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		owner.OwnerRouter(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(rawPath, "/restore-point") {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/restore-point"))
		if err != nil || !validVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		RestorePointRouter(s, w, r, volumeName)
		return
	}

	path, err := url.PathUnescape(rawPath)
	if err != nil || !validVolumeName(path) {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodDelete {
		DeleteVolume(s, w, r, path)
	} else {
		http.NotFound(w, r)
	}
}

func DeleteVolume(s *server.BLTService, w http.ResponseWriter, r *http.Request, volumeName string) {
	if !validVolumeName(volumeName) {
		http.Error(w, "invalid volume name", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if err := CleanupVolumeData(ctx, s, volumeName); err != nil {
		server.RespondError(w, fmt.Errorf("cleanup volume data: %w", err), http.StatusInternalServerError)
		return
	}
	s.RefreshStats(ctx)
	server.RespondJSON(w, server.StatusResponse{Status: fmt.Sprintf("Volume %q deleted", volumeName)})
}

func errorStatus(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid target volume name"),
		strings.Contains(msg, "invalid JSON"),
		strings.Contains(msg, "no snapshots to copy"):
		return http.StatusBadRequest
	case strings.Contains(msg, "already exists"),
		strings.Contains(msg, "cannot rename owned volume"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func CopyVolume(s *server.BLTService, w http.ResponseWriter, r *http.Request, volumeName string) {
	if r.Method != http.MethodPost {
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Target          string   `json:"target"`
		PreserveHistory *bool    `json:"preserve_history"`
		SnapshotIDs     []string `json:"snapshot_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	result, err := CopyVolumeData(ctx, s, volumeName, req.Target, req.PreserveHistory, req.SnapshotIDs)
	if err != nil {
		server.RespondError(w, err, errorStatus(err))
		return
	}

	s.RefreshStats(ctx)

	type copyResponse struct {
		Status          string `json:"status"`
		SourceOwned     bool   `json:"source_owned"`
		PreserveHistory bool   `json:"preserve_history"`
		SourceOwner     string `json:"source_owner,omitempty"`
	}
	cr := copyResponse{
		Status:          fmt.Sprintf("Volume %q copied to %q", volumeName, req.Target),
		SourceOwned:     result.SourceOwned,
		PreserveHistory: result.PreserveHistory,
	}
	if result.SourceOwned {
		cr.SourceOwner = result.OwnerName
	}
	server.RespondJSON(w, cr)
}

func RenameVolume(s *server.BLTService, w http.ResponseWriter, r *http.Request, volumeName string) {
	if r.Method != http.MethodPost {
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	err := RenameVolumeData(ctx, s, volumeName, req.Target)
	if err != nil {
		server.RespondError(w, err, errorStatus(err))
		return
	}

	s.RefreshStats(ctx)

	type renameResponse struct {
		Status string `json:"status"`
	}
	server.RespondJSON(w, renameResponse{
		Status: fmt.Sprintf("Volume %q renamed to %q", volumeName, req.Target),
	})
}
