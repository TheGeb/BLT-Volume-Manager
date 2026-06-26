package repo

import (
	"net/http"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func HandleCheck(s *server.Server, w http.ResponseWriter, r *http.Request) {
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
	err := rm.Check(true)
	if err != nil {
		app.Error("check_failed", err)
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	app.Info("check_ok")
	server.RespondJSON(w, map[string]string{"status": "Check completed, repository is healthy."})
}

func HandleRepair(s *server.Server, w http.ResponseWriter, r *http.Request) {
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
	err := rm.Repair()
	if err != nil {
		app.Error("repair_failed", err)
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	app.Info("repair_ok")
	server.RespondJSON(w, map[string]string{"status": "Repair completed, index rebuilt and stale restic locks have been cleared."})
}
