package web

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/example/docker-s3-volume-plugin/restic"
	"github.com/example/docker-s3-volume-plugin/store"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	restic     *restic.Manager
	s3Bucket   string
	s3Endpoint string
	s3Region   string
}

func NewServer(r *restic.Manager, s3Bucket string, s3Endpoint string, s3Region string) *Server {
	return &Server{restic: r, s3Bucket: s3Bucket, s3Endpoint: s3Endpoint, s3Region: s3Region}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/repo/init", s.handleRepoInit)
	mux.HandleFunc("/api/repo/status", s.handleRepoStatus)
	mux.HandleFunc("/api/volumes", s.handleVolumes)
	mux.HandleFunc("/api/snapshots", s.handleSnapshots)
	mux.HandleFunc("/api/snapshot/", s.handleSnapshotAction)
	mux.HandleFunc("/api/volume/", s.handleVolumeAction)

	uiFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(uiFS)))
}

func (s *Server) handleVolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshots, err := s.restic.ListSnapshots()
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	volumes := uniqueVolumes(snapshots)
	respondJSON(w, volumes)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	volumeFilter := r.URL.Query().Get("volume")

	snapshots, err := s.restic.ListSnapshots()
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}

	if volumeFilter != "" {
		filtered := make([]restic.Snapshot, 0, len(snapshots))
		for _, snap := range snapshots {
			if snapshotMatchesVolume(snap, volumeFilter) {
				filtered = append(filtered, snap)
			}
		}
		snapshots = filtered
	}

	respondJSON(w, snapshots)
}

func (s *Server) handleSnapshotAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/snapshot/"), "/")
	if len(parts) != 2 || parts[1] != "tag" {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if err := s.restic.TagSnapshot(snapshotID, tag); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "tag added"})
	case http.MethodDelete:
		if err := s.restic.UntagSnapshot(snapshotID, tag); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "tag removed"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

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

	for _, obj := range objects {
		if obj.Key == nil {
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
	expiry := time.Now().Add(12 * time.Hour).Unix()
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

func (s *Server) handleRepoStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	exists, err := s.restic.RepoExists()
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]interface{}{"initialized": exists, "hostname": mustHostname()})
}

func (s *Server) handleRepoInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.restic.Init(); err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "repository initialized"})
}

func mustHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

func uniqueVolumes(snapshots []restic.Snapshot) []string {
	seen := map[string]struct{}{}
	for _, snapshot := range snapshots {
		for _, path := range snapshot.Paths {
			if volume := volumeNameFromPath(path); volume != "" {
				seen[volume] = struct{}{}
			}
		}
	}

	volumes := make([]string, 0, len(seen))
	for v := range seen {
		volumes = append(volumes, v)
	}
	sort.Strings(volumes)
	return volumes
}

func volumeNameFromPath(path string) string {
	marker := "/volumes/"
	idx := strings.Index(path, marker)
	if idx >= 0 {
		subpath := strings.TrimPrefix(path[idx+len(marker):], "/")
		parts := strings.Split(subpath, "/")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func snapshotMatchesVolume(snapshot restic.Snapshot, volume string) bool {
	for _, path := range snapshot.Paths {
		if volumeNameFromPath(path) == volume {
			return true
		}
	}
	return false
}

func respondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
