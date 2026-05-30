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

	"github.com/example/blt-volume-manager/internal/constants"
	"github.com/example/blt-volume-manager/internal/store"
)

func (s *Server) handleVolumeAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/volume/") {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/volume/")
	if path == "" {
		http.NotFound(w, r)
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
	if volumeName == "" {
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
			Owner string `json:"owner"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
		}
		lock, err := s.createVolumeLock(volumeName, reqBody.Owner)
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

func (s *Server) handleDeleteVolume(w http.ResponseWriter, r *http.Request, volumeName string) {
	// 1. Delete S3 locks, volume marker, and restore point
	if s.s3Bucket != "" {
		s3, err := store.NewS3Store(store.S3StoreOpts{
			S3Bucket:        s.s3Bucket,
			S3VolumePrefix:  store.VolumePrefix,
			S3Endpoint:      s.s3Endpoint,
			Region:          s.s3Region,
		})
		if err == nil {
			_ = s3.DeleteObjectsWithPrefix(store.LockFolder(volumeName))
			_ = s3.DeleteVolumeMarker(volumeName)
			_ = s3.DeleteRestorePoint(volumeName)
		}
	}

	// 2. Delete S3 restic repo data (if repo is S3-based)
	if s.s3Bucket != "" && strings.HasPrefix(s.resticBase, "s3:") {
		s3, err := store.NewS3Store(store.S3StoreOpts{
			S3Bucket:    s.s3Bucket,
			S3Endpoint:    s.s3Endpoint,
			Region:        s.s3Region,
		})
		if err == nil {
			_ = s3.DeleteObjectsWithPrefix(constants.ResticDir + "/" + volumeName + "/")
		}
	}

	// 3. Delete file-based lock
	lockPath := filepath.Join(s.dataDir, constants.LocksDir, volumeName+".lock")
	_ = os.Remove(lockPath)

	// 4. Delete local restic repo directory (if resticBase is a local path)
	if !strings.HasPrefix(s.resticBase, "s3:") {
		repoPath := filepath.Join(s.resticBase, constants.ResticDir, volumeName)
		if err := os.RemoveAll(repoPath); err != nil {
			respondError(w, fmt.Errorf("delete restic repo: %w", err), http.StatusInternalServerError)
			return
		}
	}

	// 5. Delete volume data directory
	volPath := filepath.Join(s.dataDir, constants.VolumesDir, volumeName)
	if err := os.RemoveAll(volPath); err != nil {
		respondError(w, fmt.Errorf("delete volume data: %w", err), http.StatusInternalServerError)
		return
	}

	// 6. Refresh caches
	s.refreshStats()

	respondJSON(w, map[string]string{"status": fmt.Sprintf("Volume %q deleted", volumeName)})
}

func (s *Server) getVolumeLock(volumeName string) (map[string]interface{}, error) {
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

	result := map[string]interface{}{
		"volume": volumeName,
		"locked": false,
	}

	lockTTL := constants.DefaultLockTTL
	// Filter out stale objects older than lockTTL
	validObjects := objects[:0]
	for _, obj := range objects {
		if obj.LastModified != nil && time.Since(*obj.LastModified) > lockTTL {
			_ = s3.DeleteObject(*obj.Key)
			continue
		}
		validObjects = append(validObjects, obj)
	}

	key, owner := store.FilterValidLocks(s3, validObjects)
	if key != "" && owner != nil {
		result["locked"] = true
		result["owner"] = owner.Name
		result["expires_in"] = owner.GetRemainingTimeInSeconds()
	}

	return result, nil
}

func (s *Server) createVolumeLock(volumeName, ownerName string) (map[string]interface{}, error) {
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
	expiry := time.Now().Add(24 * time.Hour).Unix()
	folder := store.LockFolder(volumeName)
	myKey := fmt.Sprintf("%s%s-%d.json", folder, ownerName, time.Now().UnixNano())

	proposal := store.LockOwner{Name: ownerName, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal: %w", err)
	}
	if err := s3.PutObject(myKey, data); err != nil {
		return nil, fmt.Errorf("create proposal: %w", err)
	}

	objects, err := s3.ListObjects(folder)
	if err != nil {
		_ = s3.DeleteObject(myKey)
		return nil, fmt.Errorf("list proposals: %w", err)
	}

	store.SortLockObjects(objects)

	// Delete stale proposals from the same owner
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		k := *obj.Key
		if k == myKey || !strings.Contains(k, ownerName) {
			continue
		}
		_ = s3.DeleteObject(k)
	}

	// Re-list after cleanup
	objects, err = s3.ListObjects(folder)
	if err == nil {
		store.SortLockObjects(objects)
	}

	key, _ := store.FilterValidLocks(s3, objects)
	if key != myKey {
		_ = s3.DeleteObject(myKey)
		return nil, errors.New("another lock proposal was earlier")
	}

	return map[string]interface{}{
		"volume":     volumeName,
		"owner":      ownerName,
		"expires_at": expiry,
	}, nil
}

func (s *Server) storeForVolume(volumeName string) (store.S3Store, error) {
	opts := store.S3StoreOpts{
		S3Bucket:        s.s3Bucket,
		S3LockFolder:    store.LockFolder(volumeName),
		S3VolumePrefix:  store.VolumePrefix,
		S3Endpoint:      s.s3Endpoint,
		Region:          s.s3Region,
	}

	return store.NewS3Store(opts)
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
