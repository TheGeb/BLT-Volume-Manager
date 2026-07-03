package volume

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver" //FIXME: Web should not depend on driver
	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/owner"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func VolumeRouter(s *server.Server, w http.ResponseWriter, r *http.Request) {
	escapedPath := r.URL.EscapedPath()
	if !strings.HasPrefix(escapedPath, "/api/volume/") {
		http.NotFound(w, r)
		return
	}

	rawPath := strings.TrimPrefix(escapedPath, "/api/volume/")

	if strings.HasSuffix(rawPath, "/copy") {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/copy"))
		if err != nil || !server.ValidVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		CopyVolume(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(rawPath, "/rename") {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/rename"))
		if err != nil || !server.ValidVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		RenameVolume(s, w, r, volumeName)
		return
	}

	if strings.HasSuffix(rawPath, "/"+driver.OwnersDir) {
		volumeName, err := url.PathUnescape(strings.TrimSuffix(rawPath, "/"+driver.OwnersDir))
		if err != nil || !server.ValidVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		owner.OwnerRouter(s, w, r, volumeName)
		return
	}

	path, err := url.PathUnescape(rawPath)
	if err != nil || !server.ValidVolumeName(path) {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodDelete {
		DeleteVolume(s, w, r, path)
	} else {
		http.NotFound(w, r)
	}
}

func DeleteVolume(s *server.Server, w http.ResponseWriter, r *http.Request, volumeName string) {
	if !server.ValidVolumeName(volumeName) { //TODO: Need to ensure that volume groups are handled properly
		http.Error(w, "invalid volume name", http.StatusBadRequest)
		return
	}
	CleanupVolumeData(s, volumeName)
	s.RefreshStats()
	server.RespondJSON(w, map[string]string{"status": fmt.Sprintf("Volume %q deleted", volumeName)})
}

//FIXME: awkward naming
func checkedTargetManager(s *server.Server, w http.ResponseWriter, target string) *restic.Manager {
	if !server.ValidVolumeName(target) {
		server.RespondError(w, fmt.Errorf("invalid target volume name"), http.StatusBadRequest)
		return nil
	}
	for _, v := range s.VolumeNames() {
		if v == target {
			server.RespondError(w, fmt.Errorf("target volume %q already exists", target), http.StatusConflict)
			return nil
		}
	}
	tm := s.VolumeManager(target)
	if err := tm.Init(); err != nil {
		server.RespondError(w, fmt.Errorf("init target repo: %w", err), http.StatusInternalServerError)
		return nil
	}
	return tm
}

func writeVolumeMarker(s *server.Server, w http.ResponseWriter, target string) bool {
	ms, err := s.MetadataStore()
	if err != nil {
		server.RespondError(w, fmt.Errorf("initialize metadata store: %w", err), http.StatusInternalServerError)
		return false
	}
	if ms != nil {
		if err := ms.WriteVolumeMarker(target); err != nil {
			server.RespondError(w, fmt.Errorf("write volume marker: %w", err), http.StatusInternalServerError)
			return false
		}
	}
	return true
}

func CopyVolume(s *server.Server, w http.ResponseWriter, r *http.Request, volumeName string) {
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

	targetManager := checkedTargetManager(s, w, req.Target)
	if targetManager == nil {
		return
	}

	owned, ownerName, err := owner.IsVolumeOwned(s, volumeName)
	if err != nil {
		server.RespondError(w, fmt.Errorf("check owner: %w", err), http.StatusInternalServerError)
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
		//FIXME: will this default ever be hit? User should be forced to multiselect or take all
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

	if !writeVolumeMarker(s, w, req.Target) {
		return
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

	targetManager := checkedTargetManager(s, w, req.Target)
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

	sourceManager := s.VolumeManager(volumeName)
	if err := sourceManager.CopyTo(targetManager.Repo()); err != nil {
		server.RespondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
		return
	}

	snapshots, err := sourceManager.ListSnapshots()
	if err == nil {
		for _, snap := range snapshots { //FIXME: is there a way to do this in bulk? Could take a while for lots of snapshots
			if err := sourceManager.ForgetSnapshot(snap.ID); err != nil {
				log.Error("forget_snapshot_failed", err)
			}
		}
	}

	if !writeVolumeMarker(s, w, req.Target) {
		return
	}

	CleanupVolumeData(s, volumeName)
	s.RefreshStats()

	server.RespondJSON(w, map[string]any{
		"status": fmt.Sprintf("Volume %q renamed to %q", volumeName, req.Target),
	})
}
