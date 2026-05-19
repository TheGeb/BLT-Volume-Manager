package web

import "net/http"

func (s *Server) handleRepoStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	exists, err := s.restic.RepoExists()
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]interface{}{"initialized": exists, "hostname": mustHostname()})
}

func (s *Server) handleRepoInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.restic.Init(); err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "repository initialized"})
}
