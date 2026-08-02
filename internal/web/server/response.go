package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

type StatusResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

var (
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrMissingVolume    = errors.New("missing volume query parameter")
	ErrNotFound         = errors.New("not found")
)

func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		RespondError(w, ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func RequireVolumeParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	vol := r.URL.Query().Get("volume")
	if vol == "" {
		RespondError(w, ErrMissingVolume, http.StatusBadRequest)
		return "", false
	}
	return vol, true
}

// errInternalServerError is the client-facing body for 5xx responses. The
// underlying error is logged server-side but not exposed, since it can
// contain backend internals (repo paths, S3/restic error strings).
const errInternalServerError = "internal server error"

func RespondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "unknown error"
	if err != nil {
		if status >= 500 {
			log.Error("request_error", err)
			msg = errInternalServerError
		} else {
			slog.Warn("request_error", "error", err)
			msg = err.Error()
		}
	}
	if encodeErr := json.NewEncoder(w).Encode(ErrorResponse{Error: msg}); encodeErr != nil {
		log.Error("encode_error_response_failed", encodeErr)
	}
}

func RespondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error("encode_response_failed", err)
	}
}
