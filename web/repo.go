package web

import "net/http"

func (s *Server) handleRepoStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.volumeManager(volName)
	exists, err := rm.RepoExists()
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
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.volumeManager(volName)
	if err := rm.Init(); err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, map[string]string{"status": "repository initialized"})
}
