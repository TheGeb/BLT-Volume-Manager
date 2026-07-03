package repo

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type RepoStatusResponse struct {
	Initialized bool   `json:"initialized"`
	Hostname    string `json:"hostname"`
}

func InitRepo(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.ResticManager(volName)
	if err := rm.Init(); err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, server.StatusResponse{Status: "repository initialized"})
}

func RepoStatus(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.ResticManager(volName)
	exists, err := rm.RepoExists()
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, RepoStatusResponse{Initialized: exists, Hostname: metadata.Hostname()})
}
