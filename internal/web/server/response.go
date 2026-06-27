package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
)

var (
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrMissingVolume    = errors.New("missing volume query parameter")
)

func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func RequireVolumeParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	vol := r.URL.Query().Get("volume")
	if vol == "" {
		http.Error(w, ErrMissingVolume.Error(), http.StatusBadRequest)
		return "", false
	}
	return vol, true
}

func RespondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
		log.Error("request_error", err)
	}
	if encodeErr := json.NewEncoder(w).Encode(map[string]string{"error": msg}); encodeErr != nil {
		log.Error("encode_error_response_failed", encodeErr)
	}
}

func RespondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("encode_response_failed", err)
	}
}
