package web

import "net/http"

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := s.restic.Check(true)
	if err != nil {
		logInfo("check_failed")
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	logInfo("check_ok")
	respondJSON(w, map[string]string{"status": "Check completed, repository is healthy."})
}

func (s *Server) handleRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	err := s.restic.Repair()
	if err != nil {
		logInfo("repair_failed")
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	logInfo("repair_ok")
	respondJSON(w, map[string]string{"status": "Repair completed, index rebuilt and stale restic locks have been cleared."})
}
