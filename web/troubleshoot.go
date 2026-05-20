package web

import "net/http"

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	go func() {
		if err := s.restic.Check(); err != nil {
			logInfo("check_failed")
		} else {
			logInfo("check_ok")
		}
	}()
	respondJSON(w, map[string]string{"status": "check started"})
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.restic.Unlock(); err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "locks removed"})
}
