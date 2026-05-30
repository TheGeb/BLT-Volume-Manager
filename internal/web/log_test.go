package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	lw.WriteHeader(http.StatusNotFound)
	if lw.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", lw.status)
	}
}
