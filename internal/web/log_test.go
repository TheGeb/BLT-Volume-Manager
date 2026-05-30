package web

import (
	"net/http/httptest"
	"testing"
)

func TestLoggingResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	lw.WriteHeader(404)
	if lw.status != 404 {
		t.Errorf("expected status 404, got %d", lw.status)
	}
}
