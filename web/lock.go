package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/example/docker-s3-volume-plugin/store"
)

func (s *Server) handleVolumeAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/volume/") {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/volume/"), "/")
	if len(parts) != 2 || parts[1] != "locks" {
		http.NotFound(w, r)
		return
	}

	volumeName := parts[0]

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
			json.NewDecoder(r.Body).Decode(&reqBody)
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

func (s *Server) getVolumeLock(volumeName string) (map[string]interface{}, error) {
	if s.s3Bucket == "" {
		return nil, errors.New("S3_LOCK_BUCKET, RESTIC_REPOSITORY, or S3_ENDPOINT must be configured")
	}

	rw, err := s.storeForVolume(volumeName)
	if err != nil {
		return nil, err
	}

	folder := "volume-locks/" + volumeName + "/"
	objects, err := rw.ListObjects(folder)
	if err != nil {
		return nil, fmt.Errorf("list lock objects: %w", err)
	}

	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].LastModified, objects[j].LastModified
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		return *objects[i].Key < *objects[j].Key
	})

	result := map[string]interface{}{
		"volume": volumeName,
		"locked": false,
	}

	lockTTL := 24 * time.Hour

	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		if obj.LastModified != nil && time.Since(*obj.LastModified) > lockTTL {
			rw.DeleteObject(*obj.Key)
			continue
		}
		raw, err := rw.ReadObject(*obj.Key)
		if err != nil || raw == nil {
			continue
		}
		var owner store.LockOwner
		if err := json.Unmarshal(raw, &owner); err != nil {
			continue
		}
		if owner.GetRemainingTimeinSeconds() <= 0 {
			rw.DeleteObject(*obj.Key)
			continue
		}
		result["locked"] = true
		result["owner"] = owner.Name
		result["expires_in"] = owner.GetRemainingTimeinSeconds()
		break
	}

	return result, nil
}

func (s *Server) createVolumeLock(volumeName, ownerName string) (map[string]interface{}, error) {
	if s.s3Bucket == "" {
		return nil, errors.New("S3_LOCK_BUCKET, RESTIC_REPOSITORY, or S3_ENDPOINT must be configured")
	}

	rw, err := s.storeForVolume(volumeName)
	if err != nil {
		return nil, err
	}

	if ownerName == "" {
		ownerName = fmt.Sprintf("webadmin-%s-%d", mustHostname(), os.Getpid())
	}
	expiry := time.Now().Add(24 * time.Hour).Unix()
	folder := "volume-locks/" + volumeName + "/"
	myKey := fmt.Sprintf("%s%s-%d.json", folder, ownerName, time.Now().UnixNano())

	proposal := store.LockOwner{Name: ownerName, ExpiryTime: expiry}
	data, err := json.Marshal(proposal)
	if err != nil {
		return nil, fmt.Errorf("marshal proposal: %w", err)
	}
	if err := rw.PutObject(myKey, data); err != nil {
		return nil, fmt.Errorf("create proposal: %w", err)
	}

	objects, err := rw.ListObjects(folder)
	if err != nil {
		rw.DeleteObject(myKey)
		return nil, fmt.Errorf("list proposals: %w", err)
	}

	sort.Slice(objects, func(i, j int) bool {
		ti, tj := objects[i].LastModified, objects[j].LastModified
		if ti != nil && tj != nil && !ti.Equal(*tj) {
			return ti.Before(*tj)
		}
		return *objects[i].Key < *objects[j].Key
	})

	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		k := *obj.Key
		if k == myKey {
			continue
		}
		if !strings.Contains(k, ownerName) {
			continue
		}
		rw.DeleteObject(k)
	}

	first := true
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		raw, err := rw.ReadObject(*obj.Key)
		if err != nil || raw == nil {
			continue
		}
		var o store.LockOwner
		if err := json.Unmarshal(raw, &o); err != nil {
			continue
		}
		if o.GetRemainingTimeinSeconds() <= 0 {
			rw.DeleteObject(*obj.Key)
			continue
		}
		first = *obj.Key == myKey
		break
	}
	if !first {
		rw.DeleteObject(myKey)
		return nil, errors.New("another lock proposal was earlier")
	}

	return map[string]interface{}{
		"volume":     volumeName,
		"owner":      ownerName,
		"expires_at": expiry,
	}, nil
}

func (s *Server) storeForVolume(volumeName string) (*store.S3rw, error) {
	opts := store.S3StoreOpts{
		AwsBucketName: s.s3Bucket,
		AwsLockFolder: "volume-locks/" + volumeName + "/",
		S3Endpoint:    s.s3Endpoint,
		Region:        s.s3Region,
	}

	return store.NewS3Store(opts)
}

func (s *Server) deleteVolumeLocks(volumeName string) error {
	if s.s3Bucket == "" {
		return errors.New("S3_LOCK_BUCKET is not configured")
	}

	rw, err := s.storeForVolume(volumeName)
	if err != nil {
		return fmt.Errorf("create s3 store: %w", err)
	}

	return rw.DeleteLockObjects()
}
