package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/store"
)

func (s *Server) handleTestCreateVolume(w http.ResponseWriter, r *http.Request) {
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

	// Create volume directory
	volPath := filepath.Join(s.dataDir, "volumes", req.Name)
	if err := os.MkdirAll(volPath, 0755); err != nil {
		respondError(w, fmt.Errorf("create volume dir: %w", err), http.StatusInternalServerError)
		return
	}

	// Write volume config so the driver can discover it
	os.WriteFile(filepath.Join(volPath, "volume.json"), []byte(`{"fs_type":""}`), 0644)

	// Create dummy files and folders
	dummyContent := map[string]string{
		"readme.txt":        "This is a test volume created at " + time.Now().Format(time.RFC3339),
		"config/app.json":   `{"version": "1.0", "debug": true, "name": "test-app"}`,
		"config/db.yaml":    "host: localhost\nport: 5432\ndatabase: testdb",
		"data/users.csv":    "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n3,Charlie,charlie@example.com",
		"data/orders.csv":   "id,user_id,total\n1,1,99.99\n2,2,149.50\n3,3,75.00\n4,1,200.00",
		"logs/app.log":      "2024-01-01 10:00:00 INFO  Starting application\n2024-01-01 10:00:01 INFO  Connected to database\n2024-01-01 10:00:02 INFO  Server listening on port 8080",
		"scripts/setup.sh":  "#!/bin/bash\necho \"Setting up...\"\nmkdir -p /data\necho \"Done.\"",
	}

	for path, content := range dummyContent {
		fullPath := filepath.Join(volPath, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			respondError(w, fmt.Errorf("create dir %s: %w", path, err), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			respondError(w, fmt.Errorf("write file %s: %w", path, err), http.StatusInternalServerError)
			return
		}
	}

	// Initialize repo if needed
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

	// Run backup from within the volume dir so restic stores relative paths
	cwd, err := os.Getwd()
	if err != nil {
		respondError(w, fmt.Errorf("getwd: %w", err), http.StatusInternalServerError)
		return
	}
	if err := os.Chdir(volPath); err != nil {
		respondError(w, fmt.Errorf("chdir: %w", err), http.StatusInternalServerError)
		return
	}
	defer os.Chdir(cwd)
	if err := rm.Backup(".", "cold"); err != nil {
		respondError(w, fmt.Errorf("backup: %w", err), http.StatusInternalServerError)
		return
	}

	if s.s3Bucket != "" {
		rw, err := store.NewS3Store(store.S3StoreOpts{
			AWSBucketName:   s.s3Bucket,
			AWSVolumePrefix: store.VolumePrefix,
			S3Endpoint:      s.s3Endpoint,
			Region:          s.s3Region,
		})
		if err == nil {
			rw.WriteVolumeMarker(req.Name)
		}
	}

	respondJSON(w, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Created test volume %q with %d files and backed up", req.Name, len(dummyContent)),
	})
}
