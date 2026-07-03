package snapshot

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func handleRestoreSnapshot(s *server.Server, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	target := r.URL.Query().Get("path")
	if target == "" {
		http.Error(w, "missing path query parameter", http.StatusBadRequest)
		return
	}
	s.AddWork()
	go func() {
		defer s.DoneWork()
		if err := rm.RestoreSnapshot(snapshotID, target); err != nil {
			log.Errorf("restore_failed", err, "snapshot=%s target=%s", snapshotID, target)
		} else {
			log.Info("restore_ok")
		}
	}()
	server.RespondJSON(w, map[string]string{"status": "restore started – see server logs for results"})
}
