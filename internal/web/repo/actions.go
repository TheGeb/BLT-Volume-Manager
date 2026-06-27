package repo

import (
	"net/http"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func InitRepo(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.VolumeManager(volName)
	if err := rm.Init(); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, map[string]string{"status": "repository initialized"})
}

func RepoStatus(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
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
