package repo

import (
	"net/http"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func HandleRepoInit(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.VolumeManager(volName)
	if err := rm.Init(); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, map[string]string{"status": "repository initialized"})
}

func HandleRepoStatus(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.VolumeManager(volName)
	exists, err := rm.RepoExists()
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, map[string]any{"initialized": exists, "hostname": metadata.Hostname()})
}
