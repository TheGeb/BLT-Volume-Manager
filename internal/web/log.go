package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/internal/applog"
)

func s3LogFn() func(op, bucket, key string, dur time.Duration, err error) {
	return applog.S3Call
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
		level := applog.LevelDebug
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, ".js") {
			level = applog.LevelTrace
		}
		applog.LogEvent(level, applog.Event{
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
