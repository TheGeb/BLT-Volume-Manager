package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
)

func (s *Server) handleDummyVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		respondError(w, fmt.Errorf("name is required"), http.StatusBadRequest)
		return
	}
	if !strings.Contains(req.Name, "/") {
		respondError(w, fmt.Errorf("name must be in the format group/name"), http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(req.Name)
	volPath, err := os.MkdirTemp("", "blt-dummy-*")
	if err != nil {
		respondError(w, fmt.Errorf("create temp dir: %w", err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(volPath) }()

	if err := os.MkdirAll(volPath, 0o755); err != nil {
		respondError(w, fmt.Errorf("create volume dir: %w", err), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(filepath.Join(volPath, constants.VolumeConfigFile), []byte(`{"fs_type":""}`), 0o644); err != nil {
		respondError(w, fmt.Errorf("write volume config: %w", err), http.StatusInternalServerError)
		return
	}

	count := writeDummyFiles(volPath)

	exists, err := rm.RepoExists()
	if err != nil {
		respondError(w, fmt.Errorf("check repo: %w", err), http.StatusInternalServerError)
		return
	}
	if !exists {
		if err := rm.Init(); err != nil {
			respondError(w, fmt.Errorf("init repo: %w", err), http.StatusInternalServerError)
			return
		}
	}

	if err := rm.BackupInDir(".", constants.BackupTagCold, volPath); err != nil {
		respondError(w, fmt.Errorf("backup: %w", err), http.StatusInternalServerError)
		return
	}

	if s.s3Bucket != "" {
		s3, err := store.NewS3Store(store.S3StoreConfig{
			S3Bucket:       s.s3Bucket,
			S3VolumePrefix: store.VolumePrefix,
			S3Endpoint:     s.s3Endpoint,
			Region:         s.s3Region,
			Logger:         s3LogFn(),
		})
		if err == nil {
			if err := s3.WriteVolumeMarker(req.Name); err != nil {
				logError("write_volume_marker_failed", err)
			}
		}
	}

	respondJSON(w, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Created test volume %q with %d files and backed up", req.Name, count),
	})
}

func (s *Server) handleDummySnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Volume string `json:"volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if req.Volume == "" {
		respondError(w, fmt.Errorf("volume is required"), http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(req.Volume)
	volPath, err := os.MkdirTemp("", "blt-dummy-snap-*")
	if err != nil {
		respondError(w, fmt.Errorf("create temp dir: %w", err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(volPath) }()

	if err := os.MkdirAll(volPath, 0o755); err != nil {
		respondError(w, fmt.Errorf("create temp dir: %w", err), http.StatusInternalServerError)
		return
	}

	count := writeDummyFiles(volPath)

	if err := rm.BackupInDir(".", constants.BackupTagCold, volPath); err != nil {
		respondError(w, fmt.Errorf("backup: %w", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Created test snapshot with %d files on volume %q", count, req.Volume),
	})
}

func (s *Server) handleDevMode(w http.ResponseWriter, r *http.Request) {
	enabled := os.Getenv("BLT_TEST_MODE") != ""
	respondJSON(w, map[string]bool{"enabled": enabled})
}

func writeDummyFiles(dir string) int {
	dummyContent := map[string]string{
		"readme.txt":       "This is a test volume created at " + time.Now().Format(time.RFC3339),
		"config/app.json":  `{"version": "1.0", "debug": true, "name": "test-app"}`,
		"config/db.yaml":   "host: localhost\nport: 5432\ndatabase: testdb",
		"data/users.csv":   "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n3,Charlie,charlie@example.com",
		"data/orders.csv":  "id,user_id,total\n1,1,99.99\n2,2,149.50\n3,3,75.00\n4,1,200.00",
		"logs/app.log":     "2024-01-01 10:00:00 INFO  Starting application\n2024-01-01 10:00:01 INFO  Connected to database\n2024-01-01 10:00:02 INFO  Server listening on port 8080",
		"scripts/setup.sh": "#!/bin/bash\necho \"Setting up...\"\nmkdir -p /data\necho \"Done.\"",
	}
	for path, content := range dummyContent {
		fullPath := filepath.Join(dir, path)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
		_ = os.WriteFile(fullPath, []byte(content), 0o644)
	}
	return len(dummyContent)
}
