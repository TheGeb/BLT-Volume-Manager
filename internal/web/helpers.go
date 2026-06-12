package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

func validVolumeName(name string) bool {
	return name != "" && !strings.Contains(name, "..") && !strings.Contains(name, "\\")
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func requireVolumeParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	vol := r.URL.Query().Get("volume")
	if vol == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return "", false
	}
	return vol, true
}

func respondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
		logError("request_error", err)
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logError("encode_error_response_failed", err)
	}
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logError("encode_response_failed", err)
	}
}
