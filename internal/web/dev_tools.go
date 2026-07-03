package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func TestMode(s *server.Server, w http.ResponseWriter, r *http.Request) {
	enabled := os.Getenv("BLT_DEV_MODE") != ""
	server.RespondJSON(w, map[string]bool{"enabled": enabled})
}

func CreateDummyVolume(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		server.RespondError(w, fmt.Errorf("name is required"), http.StatusBadRequest)
		return
	}
	rm := s.VolumeManager(req.Name)
	volPath, err := os.MkdirTemp("", "blt-dummy-volume-")
	if err != nil {
		server.RespondError(w, fmt.Errorf("create temp dir: %w", err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(volPath) }()

	if err := os.WriteFile(filepath.Join(volPath, VolumeConfigFile), []byte(`{"fs_type":""}`), 0o644); err != nil { // TODO: get rid of magic number across app
		server.RespondError(w, fmt.Errorf("write volume config: %w", err), http.StatusInternalServerError)
		return
	}

	count := writeDummyFiles(volPath)

	exists, err := rm.RepoExists()
	if err != nil {
		server.RespondError(w, fmt.Errorf("check repo: %w", err), http.StatusInternalServerError)
		return
	}
	if !exists {
		if err := rm.Init(); err != nil {
			server.RespondError(w, fmt.Errorf("init repo: %w", err), http.StatusInternalServerError)
			return
		}
	}

	vt := s.NextVersionTags(req.Name, true)
	if err := rm.BackupInDir(".", restic.WithTags(restic.BackupTagCold, vt...), volPath); err != nil {
		server.RespondError(w, fmt.Errorf("backup: %w", err), http.StatusInternalServerError)
		return
	}

	if ms, err := s.MetadataStore(); err == nil && ms != nil {
		if err := ms.WriteRegisteredVolume(req.Name); err != nil {
			log.Error("write_registered_volume_failed", err)
		}
	}

	server.RespondJSON(w, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Created test volume %q with %d files and backed up", req.Name, count),
	})
}

func CreateDummySnapshot(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Volume string `json:"volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid request: %w", err), http.StatusBadRequest)
		return
	}
	if req.Volume == "" {
		server.RespondError(w, fmt.Errorf("volume is required"), http.StatusBadRequest)
		return
	}

	rm := s.VolumeManager(req.Volume)
	volPath, err := os.MkdirTemp("", "blt-dummy-snapshot-")
	if err != nil {
		server.RespondError(w, fmt.Errorf("create temp dir: %w", err), http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(volPath) }()

	count := writeDummyFiles(volPath)

	vt := s.NextVersionTags(req.Volume, false)
	if err := rm.BackupInDir(".", restic.WithTags(restic.BackupTagCold, vt...), volPath); err != nil {
		server.RespondError(w, fmt.Errorf("backup: %w", err), http.StatusInternalServerError)
		return
	}

	server.RespondJSON(w, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Created test snapshot with %d files on volume %q", count, req.Volume),
	})
}

func writeDummyFiles(dir string) int {
	dummyContent := map[string]string{
		"readme.txt":       "This is a test volume created at " + time.Now().Format(time.RFC3339),
		"config/log.json":  `{"version": "1.0", "debug": true, "name": "test-app"}`,
		"config/db.yaml":   "host: localhost\nport: 5432\ndatabase: testdb",
		"data/users.csv":   "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n3,Charlie,charlie@example.com",
		"data/orders.csv":  "id,user_id,total\n1,1,99.99\n2,2,149.50\n3,3,75.00\n4,1,200.00",
		"logs/log.log":     "2024-01-01 10:00:00 INFO  Starting application\n2024-01-01 10:00:01 INFO  Connected to database\n2024-01-01 10:00:02 INFO  Server listening on port 8080",
		"scripts/setup.sh": "#!/bin/bash\necho \"Setting up...\"\nmkdir -p /data\necho \"Done.\"",
	}
	for path, content := range dummyContent {
		fullPath := filepath.Join(dir, path)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0o755) // TODO: get rid of magic number across app
		_ = os.WriteFile(fullPath, []byte(content), 0o644)
	}
	return len(dummyContent)
}
