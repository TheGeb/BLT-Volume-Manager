package volume

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/owner"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type VolumeListResponse struct {
	Volumes []string `json:"volumes"`
}

const OwnersDir = "owners"

func ListVolumes(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	volumes := s.VolumeNames()
	if volumes == nil {
		volumes = []string{}
	}
	server.RespondJSON(w, VolumeListResponse{Volumes: volumes})
}

// Allows groups with "/"
func validVolumeName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..")
}

func VolumeRouter(s *server.Service, w http.ResponseWriter, r *http.Request) {
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

func DeleteVolume(s *server.Service, w http.ResponseWriter, r *http.Request, volumeName string) {
	if !validVolumeName(volumeName) {
		http.Error(w, "invalid volume name", http.StatusBadRequest)
		return
	}
	CleanupVolumeData(s, volumeName)
	s.RefreshStats()
	server.RespondJSON(w, server.StatusResponse{Status: fmt.Sprintf("Volume %q deleted", volumeName)})
}

func initTargetManager(s *server.Service, w http.ResponseWriter, target string) *restic.Manager {
	if !validVolumeName(target) {
		server.RespondError(w, fmt.Errorf("invalid target volume name"), http.StatusBadRequest)
		return nil
	}
	for _, v := range s.VolumeNames() {
		if v == target {
			server.RespondError(w, fmt.Errorf("target volume %q already exists", target), http.StatusConflict)
			return nil
		}
	}
	tm := s.ResticManager(target)
	if err := tm.Init(); err != nil {
		server.RespondError(w, fmt.Errorf("init target repo: %w", err), http.StatusInternalServerError)
		return nil
	}
	return tm
}

func writeRegisteredVolume(s *server.Service, w http.ResponseWriter, target string) bool {
	if err := s.RegisterVolume(target); err != nil {
		server.RespondError(w, fmt.Errorf("register volume: %w", err), http.StatusInternalServerError)
		return false
	}
	return true
}

func CopyVolume(s *server.Service, w http.ResponseWriter, r *http.Request, volumeName string) {
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

	targetManager := initTargetManager(s, w, req.Target)
	if targetManager == nil {
		return
	}

	owned, ownerName, err := owner.IsVolumeOwned(s, volumeName)
	if err != nil {
		server.RespondError(w, fmt.Errorf("check owner: %w", err), http.StatusInternalServerError)
		return
	}

	sourceManager := s.ResticManager(volumeName)
	preserveHistory := true
	if req.PreserveHistory != nil {
		preserveHistory = *req.PreserveHistory
	}
	switch {
	case len(req.SnapshotIDs) > 0:
		if err := sourceManager.CopyTo(targetManager.Repo(), req.SnapshotIDs...); err != nil {
			server.RespondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
			return
		}
	case preserveHistory:
		if err := sourceManager.CopyTo(targetManager.Repo()); err != nil {
			server.RespondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
			return
		}
	default:
		// FIXME: will this default ever be hit? User should be forced to multiselect or take all
		// Maybe if they try to multiselect but choose nothing??
		snaps, err := sourceManager.ListSnapshots()
		if err != nil {
			server.RespondError(w, fmt.Errorf("list snapshots: %w", err), http.StatusInternalServerError)
			return
		}
		if len(snaps) == 0 {
			server.RespondError(w, fmt.Errorf("no snapshots to copy"), http.StatusBadRequest)
			return
		}
		if err := sourceManager.CopyTo(targetManager.Repo(), snaps[0].ID); err != nil {
			server.RespondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
			return
		}
	}

	if !writeRegisteredVolume(s, w, req.Target) {
		return
	}

	s.RefreshStats()

	type copyResponse struct {
		Status          string `json:"status"`
		SourceOwned     bool   `json:"source_owned"`
		PreserveHistory bool   `json:"preserve_history"`
		SourceOwner     string `json:"source_owner,omitempty"`
	}
	cr := copyResponse{
		Status:          fmt.Sprintf("Volume %q copied to %q", volumeName, req.Target),
		SourceOwned:     owned,
		PreserveHistory: preserveHistory,
	}
	if owned {
		cr.SourceOwner = ownerName
	}
	server.RespondJSON(w, cr)
}

func RenameVolume(s *server.Service, w http.ResponseWriter, r *http.Request, volumeName string) {
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

	targetManager := initTargetManager(s, w, req.Target)
	if targetManager == nil {
		return
	}

	owned, ownerName, err := owner.IsVolumeOwned(s, volumeName)
	if err != nil {
		server.RespondError(w, fmt.Errorf("check owner: %w", err), http.StatusInternalServerError)
		return
	}
	if owned {
		server.RespondError(w, fmt.Errorf("cannot rename owned volume %q (owned by %q)", volumeName, ownerName), http.StatusConflict)
		return
	}

	sourceManager := s.ResticManager(volumeName)
	if err := sourceManager.CopyTo(targetManager.Repo()); err != nil {
		server.RespondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
		return
	}

	snapshots, err := sourceManager.ListSnapshots()
	if err == nil {
		ids := make([]string, len(snapshots))
		for i, snap := range snapshots {
			ids[i] = snap.ID
		}
		if err := sourceManager.ForgetSnapshots(ids...); err != nil {
			log.Error("forget_snapshots_failed", err)
		}
	}

	if !writeRegisteredVolume(s, w, req.Target) {
		return
	}

	CleanupVolumeData(s, volumeName)
	s.RefreshStats()

	type renameResponse struct {
		Status string `json:"status"`
	}
	server.RespondJSON(w, renameResponse{
		Status: fmt.Sprintf("Volume %q renamed to %q", volumeName, req.Target),
	})
}
