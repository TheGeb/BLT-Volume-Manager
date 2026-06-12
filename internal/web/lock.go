package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (s *Server) handleVolumeAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/volume/") {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/volume/")
	if !validVolumeName(path) {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(path, "/copy") {
		volumeName := strings.TrimSuffix(path, "/copy")
		if !validVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		s.handleCopyVolume(w, r, volumeName)
		return
	}

	if strings.HasSuffix(path, "/rename") {
		volumeName := strings.TrimSuffix(path, "/rename")
		if !validVolumeName(volumeName) {
			http.NotFound(w, r)
			return
		}
		s.handleRenameVolume(w, r, volumeName)
		return
	}

	if !strings.HasSuffix(path, "/"+constants.LocksDir) {
		if r.Method == http.MethodDelete {
			s.handleDeleteVolume(w, r, path)
		} else {
			http.NotFound(w, r)
		}
		return
	}

	volumeName := strings.TrimSuffix(path, "/"+constants.LocksDir)
	if !validVolumeName(volumeName) {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		status, err := s.getVolumeLock(volumeName)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, status)
	case http.MethodPost:
		var reqBody struct {
			Owner            string `json:"owner"`
			LockDurationMins int    `json:"lock_duration_mins"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				respondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
				return
			}
		}
		lock, err := s.createVolumeLock(volumeName, reqBody.Owner, reqBody.LockDurationMins)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, lock)
	case http.MethodDelete:
		if err := s.deleteVolumeLocks(volumeName); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "locks deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) cleanupVolumeData(volumeName string) {
	if volumeName == "" {
		return
	}
	if s.s3Bucket != "" {
		if volStore, err := s.storeForVolume(volumeName); err == nil {
			if err := volStore.DeleteObjectsWithPrefix(store.LockFolder(volumeName)); err != nil {
				logError("delete_lock_objects_failed", err)
			}
			if err := volStore.DeleteVolumeMarker(volumeName); err != nil {
				logError("delete_volume_marker_failed", err)
			}
			if err := volStore.DeleteRestorePoint(volumeName); err != nil {
				logError("delete_restore_point_failed", err)
			}
		}
		if s.isS3Backend() {
			if dataStore, err := s.storeForResticData(); err == nil && dataStore != nil {
				if err := dataStore.DeleteObjectsWithPrefix(constants.ResticDir + "/" + volumeName + "/"); err != nil {
					logError("delete_restic_data_failed", err)
				}
			}
		}
	}
	if !s.isS3Backend() {
		repoPath := filepath.Join(s.resticBase, constants.ResticDir, volumeName)
		//nolint:gosec // volumeName is validated by caller
		if err := os.RemoveAll(repoPath); err != nil {
			logError("delete_restic_repo_failed", err)
		}
	}
}

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request, volumeName string) {
	if !validVolumeName(volumeName) || strings.Contains(volumeName, "/") {
		http.Error(w, "invalid volume name", http.StatusBadRequest)
		return
	}
	s.cleanupVolumeData(volumeName)
	s.refreshStats()
	respondJSON(w, map[string]string{"status": fmt.Sprintf("Volume %q deleted", volumeName)})
}

func (s *Server) getVolumeLock(volumeName string) (map[string]any, error) {
	if s.s3Bucket == "" {
		return nil, errors.New("S3_LOCK_BUCKET, RESTIC_REPOSITORY, or S3_ENDPOINT must be configured")
	}

	s3, err := s.storeForVolume(volumeName)
	if err != nil {
		return nil, err
	}

	folder := store.LockFolder(volumeName)
	objects, err := s3.ListObjects(folder)
	if err != nil {
		return nil, fmt.Errorf("list lock objects: %w", err)
	}

	store.SortLockObjects(objects)
	objects = store.FilterStaleLockObjects(s3, objects, constants.DefaultLockTTL)

	result := map[string]any{
		"volume": volumeName,
		"locked": false,
	}

	key, owner, expiry := store.FilterValidLocksByKey(s3, objects)
	if key != "" {
		result["locked"] = true
		result["owner"] = owner
		if expiry > 0 {
			remaining := expiry - time.Now().Unix()
			if remaining < 0 {
				remaining = 0
			}
			result["expires_in"] = remaining
		}
	}

	return result, nil
}

func (s *Server) handleVolumesLocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	locks := make(map[string]map[string]any)

	if s.s3Bucket == "" {
		respondJSON(w, map[string]any{"locks": locks})
		return
	}

	s3Store, err := s.getOrCreateS3Store()
	if err != nil || s3Store == nil {
		respondJSON(w, map[string]any{"locks": locks})
		return
	}

	objects, listErr := s3Store.ListObjects(store.LockPrefix)
	if listErr != nil {
		respondJSON(w, map[string]any{"locks": locks})
		return
	}

	kept := store.FilterStaleLockObjects(s3Store, objects, constants.DefaultLockTTL)

	grouped := make(map[string][]types.Object)
	for _, obj := range kept {
		vol, _, _, err := store.ParseLockKey(*obj.Key)
		if err != nil && !errors.Is(err, store.ErrOldLockKeyFormat) {
			continue
		}
		if vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	for vol, objs := range grouped {
		store.SortLockObjects(objs)
		key, owner, expiry := store.FilterValidLocksByKey(s3Store, objs)
		if key != "" {
			result := map[string]any{
				"volume": vol,
				"locked": true,
				"owner":  owner,
			}
			if expiry > 0 {
				remaining := expiry - time.Now().Unix()
				if remaining < 0 {
					remaining = 0
				}
				result["expires_in"] = remaining
			}
			locks[vol] = result
		}
	}

	respondJSON(w, map[string]any{"locks": locks})
}

func (s *Server) createVolumeLock(volumeName, ownerName string, lockDurationMins int) (map[string]any, error) {
	if s.s3Bucket == "" {
		return nil, errors.New("S3_LOCK_BUCKET, RESTIC_REPOSITORY, or S3_ENDPOINT must be configured")
	}

	s3, err := s.storeForVolume(volumeName)
	if err != nil {
		return nil, err
	}

	if ownerName == "" {
		ownerName = fmt.Sprintf("webadmin-%s-%d", store.Hostname(), os.Getpid())
	}
	var expiry int64
	if lockDurationMins > 0 {
		expiry = time.Now().Add(time.Duration(lockDurationMins) * time.Minute).Unix()
	}

	folder := store.LockFolder(volumeName)
	_, err = store.AcquireLock(s3, folder, ownerName, expiry)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"volume":     volumeName,
		"owner":      ownerName,
		"expires_at": expiry,
	}, nil
}

func (s *Server) isVolumeLocked(volumeName string) (bool, string, error) {
	status, err := s.getVolumeLock(volumeName)
	if err != nil {
		return false, "", err
	}
	if locked, ok := status["locked"].(bool); ok && locked {
		owner, ok := status["owner"].(string)
		if !ok {
			owner = ""
		}
		return true, owner, nil
	}
	return false, "", nil
}

func (s *Server) handleCopyVolume(w http.ResponseWriter, r *http.Request, volumeName string) {
	if r.Method != http.MethodPost {
		respondError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Target          string   `json:"target"`
		PreserveHistory *bool    `json:"preserve_history"`
		SnapshotIDs     []string `json:"snapshot_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}
	if !validVolumeName(req.Target) {
		respondError(w, fmt.Errorf("invalid target volume name"), http.StatusBadRequest)
		return
	}

	locked, owner, err := s.isVolumeLocked(volumeName)
	if err != nil {
		respondError(w, fmt.Errorf("check lock: %w", err), http.StatusInternalServerError)
		return
	}

	// Check that the target doesn't already exist
	existing := s.volumeNames()
	for _, v := range existing {
		if v == req.Target {
			respondError(w, fmt.Errorf("target volume %q already exists", req.Target), http.StatusConflict)
			return
		}
	}

	// Init target repo
	targetManager := s.volumeManager(req.Target)
	if err := targetManager.Init(); err != nil {
		respondError(w, fmt.Errorf("init target repo: %w", err), http.StatusInternalServerError)
		return
	}

	// Copy snapshots from source to target
	sourceManager := s.volumeManager(volumeName)
	preserveHistory := true
	if req.PreserveHistory != nil {
		preserveHistory = *req.PreserveHistory
	}
	switch {
	case len(req.SnapshotIDs) > 0:
		if err := sourceManager.CopyTo(targetManager.Repo(), req.SnapshotIDs...); err != nil {
			respondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
			return
		}
	case preserveHistory:
		if err := sourceManager.CopyTo(targetManager.Repo()); err != nil {
			respondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
			return
		}
	default:
		snaps, err := sourceManager.ListSnapshots()
		if err != nil {
			respondError(w, fmt.Errorf("list snapshots: %w", err), http.StatusInternalServerError)
			return
		}
		if len(snaps) == 0 {
			respondError(w, fmt.Errorf("no snapshots to copy"), http.StatusBadRequest)
			return
		}
		if err := sourceManager.CopyTo(targetManager.Repo(), snaps[0].ID); err != nil {
			respondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
			return
		}
	}

	// Write target volume marker
	s3, err := s.getOrCreateS3StoreWithPrefix(store.VolumePrefix)
	if err != nil {
		respondError(w, fmt.Errorf("create s3 store: %w", err), http.StatusInternalServerError)
		return
	}
	if s3 != nil {
		if err := s3.WriteVolumeMarker(req.Target); err != nil {
			respondError(w, fmt.Errorf("write volume marker: %w", err), http.StatusInternalServerError)
			return
		}
	}

	s.refreshStats()

	resp := map[string]any{
		"status":           fmt.Sprintf("Volume %q copied to %q", volumeName, req.Target),
		"source_locked":    locked,
		"preserve_history": preserveHistory,
	}
	if locked {
		resp["source_owner"] = owner
	}
	respondJSON(w, resp)
}

func (s *Server) handleRenameVolume(w http.ResponseWriter, r *http.Request, volumeName string) {
	if r.Method != http.MethodPost {
		respondError(w, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Target string `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}
	if !validVolumeName(req.Target) {
		respondError(w, fmt.Errorf("invalid target volume name"), http.StatusBadRequest)
		return
	}

	locked, owner, err := s.isVolumeLocked(volumeName)
	if err != nil {
		respondError(w, fmt.Errorf("check lock: %w", err), http.StatusInternalServerError)
		return
	}
	if locked {
		respondError(w, fmt.Errorf("cannot rename locked volume %q (locked by %q)", volumeName, owner), http.StatusConflict)
		return
	}

	// Check that the target doesn't already exist
	existing := s.volumeNames()
	for _, v := range existing {
		if v == req.Target {
			respondError(w, fmt.Errorf("target volume %q already exists", req.Target), http.StatusConflict)
			return
		}
	}

	// Init target repo
	targetManager := s.volumeManager(req.Target)
	if err := targetManager.Init(); err != nil {
		respondError(w, fmt.Errorf("init target repo: %w", err), http.StatusInternalServerError)
		return
	}

	// Copy snapshots from source to target
	sourceManager := s.volumeManager(volumeName)
	if err := sourceManager.CopyTo(targetManager.Repo()); err != nil {
		respondError(w, fmt.Errorf("copy snapshots: %w", err), http.StatusInternalServerError)
		return
	}

	// Forget all snapshots in source
	snapshots, err := sourceManager.ListSnapshots()
	if err == nil {
		for _, snap := range snapshots {
			if err := sourceManager.ForgetSnapshot(snap.ID); err != nil {
				logError("forget_snapshot_failed", err)
			}
		}
	}

	// Write target volume marker
	vs, err := s.getOrCreateS3StoreWithPrefix(store.VolumePrefix)
	if err != nil {
		respondError(w, fmt.Errorf("create s3 store: %w", err), http.StatusInternalServerError)
		return
	}
	if vs != nil {
		if err := vs.WriteVolumeMarker(req.Target); err != nil {
			respondError(w, fmt.Errorf("write volume marker: %w", err), http.StatusInternalServerError)
			return
		}
	}

	s.cleanupVolumeData(volumeName)
	s.refreshStats()

	respondJSON(w, map[string]any{
		"status": fmt.Sprintf("Volume %q renamed to %q", volumeName, req.Target),
	})
}

func (s *Server) deleteVolumeLocks(volumeName string) error {
	if s.s3Bucket == "" {
		return errors.New("S3_LOCK_BUCKET is not configured")
	}

	s3, err := s.storeForVolume(volumeName)
	if err != nil {
		return fmt.Errorf("create s3 store: %w", err)
	}

	return s3.DeleteLockObjects()
}
