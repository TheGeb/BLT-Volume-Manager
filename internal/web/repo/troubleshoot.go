package repo

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func CheckRepo(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.VolumeManager(volName)
	err := rm.Check(true)
	if err != nil {
		log.Error("check_failed", err)
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	log.Info("check_ok")
	server.RespondJSON(w, map[string]string{"status": "Check completed, repository is healthy."})
}

func RepairRepo(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.VolumeManager(volName)
	err := rm.Repair()
	if err != nil {
		log.Error("repair_failed", err)
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	log.Info("repair_ok")
	server.RespondJSON(w, map[string]string{"status": "Repair completed, index rebuilt and stale restic locks have been cleared."})
}
