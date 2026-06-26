package volume

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/owner"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func VolumeRouter(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/volume/") {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/volume/")
	if !server.ValidVolumeName(path) {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(path, "/copy") {
		volumeName := strings.TrimSuffix(path, "/copy")
		if !server.ValidVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		CopyVolume(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(path, "/rename") {
		volumeName := strings.TrimSuffix(path, "/rename")
		if !server.ValidVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		RenameVolume(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(path, "/"+driver.OwnersDir) {
		volumeName := strings.TrimSuffix(path, "/"+driver.OwnersDir)
		if !server.ValidVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		owner.OwnerRouter(s, w, r, volumeName)
		return
	}

	if r.Method == http.MethodDelete {
		DeleteVolume(s, w, r, path)
	} else {
		http.NotFound(w, r)
	}
}

func DeleteVolume(s *server.Server, w http.ResponseWriter, r *http.Request, volumeName string) {
	if !server.ValidVolumeName(volumeName) || strings.Contains(volumeName, "/") {
		http.Error(w, "invalid volume name", http.StatusBadRequest)
		return
	}
	CleanupVolumeData(s, volumeName)
	s.RefreshStats()
	server.RespondJSON(w, map[string]string{"status": fmt.Sprintf("Volume %q deleted", volumeName)})
}

func CopyVolume(s *server.Server, w http.ResponseWriter, r *http.Request, volumeName string) {
	if r.Method != http.MethodPost {
		server.RespondError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
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
	if !server.ValidVolumeName(req.Target) {
		server.RespondError(w, fmt.Errorf("invalid target volume name"), http.StatusBadRequest)
		return
	}

	owned, ownerName, err := owner.IsVolumeOwned(s, volumeName)
	if err != nil {
		server.RespondError(w, fmt.Errorf("check owner: %w", err), http.StatusInternalServerError)
		return
	}

	existing := s.VolumeNames()
	for _, v := range existing {
		if v == req.Target {
			server.RespondError(w, fmt.Errorf("target volume %q already exists", req.Target), http.StatusConflict)
			return
		}
	}

	targetManager := s.VolumeManager(req.Target)
	if err := targetManager.Init(); err != nil {
		server.RespondError(w, fmt.Errorf("init target repo: %w", err), http.StatusInternalServerError)
		return
	}

	sourceManager := s.VolumeManager(volumeName)
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

	ms, err := s.MetadataStore()
	if err != nil {
		server.RespondError(w, fmt.Errorf("initialize metadata store: %w", err), http.StatusInternalServerError)
		return
	}
	if ms != nil {
		if err := ms.WriteVolumeMarker(req.Target); err != nil {
			server.RespondError(w, fmt.Errorf("write volume marker: %w", err), http.StatusInternalServerError)
			return
		}
	}

	s.RefreshStats()

	resp := map[string]any{
		"status":           fmt.Sprintf("Volume %q copied to %q", volumeName, req.Target),
		"source_owned":     owned,
		"preserve_history": preserveHistory,
	}
	if owned {
		resp["source_owner"] = ownerName
	}
	server.RespondJSON(w, resp)
}

func RenameVolume(s *server.Server, w http.ResponseWriter, r *http.Request, volumeName string) {
	if r.Method != http.MethodPost {
		server.RespondError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}
	if !server.ValidVolumeName(req.Target) {
		server.RespondError(w, fmt.Errorf("invalid target volume name"), http.StatusBadRequest)
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

	existing := s.VolumeNames()
	for _, v := range existing {
		if v == req.Target {
			server.RespondError(w, fmt.Errorf("target volume %q already exists", req.Target), http.StatusConflict)
			return
		}
	}

	targetManager := s.VolumeManager(req.Target)
	if err := targetManager.Init(); err != nil {
		server.RespondError(w, fmt.Errorf("init target repo: %w", err), http.StatusInternalServerError)
		return
	}

	sourceManager := s.VolumeManager(volumeName)
	if err := sourceManager.CopyTo(targetManager.Repo()); err != nil {
		server.RespondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
		return
	}

	snapshots, err := sourceManager.ListSnapshots()
	if err == nil {
		for _, snap := range snapshots {
			if err := sourceManager.ForgetSnapshot(snap.ID); err != nil {
				log.Error("forget_snapshot_failed", err)
			}
		}
	}

	vs, err := s.MetadataStore()
	if err != nil {
		server.RespondError(w, fmt.Errorf("initialize metadata store: %w", err), http.StatusInternalServerError)
		return
	}
	if vs != nil {
		if err := vs.WriteVolumeMarker(req.Target); err != nil {
			server.RespondError(w, fmt.Errorf("write volume marker: %w", err), http.StatusInternalServerError)
			return
		}
	}

	CleanupVolumeData(s, volumeName)
	s.RefreshStats()

	server.RespondJSON(w, map[string]any{
		"status": fmt.Sprintf("Volume %q renamed to %q", volumeName, req.Target),
	})
}
