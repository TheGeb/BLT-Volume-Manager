package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/internal/applog"
	"github.com/example/blt-volume-manager/internal/store"
)

func init() {
	store.LogS3 = applog.S3Call
}

func logInfo(event string) {
	applog.Info(event)
}

func logError(event string, err error) {
	applog.Error(event, err)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(lw, r)
		dur := time.Since(start)
		level := "debug"
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, ".js") {
			level = "trace"
		}
		applog.Log(applog.Entry{
			Level:      level,
			Event:      "http_request",
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     lw.status,
			DurationMs: dur.Milliseconds(),
		})
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}
