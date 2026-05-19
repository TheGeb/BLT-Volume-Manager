package web

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/store"
)

func init() {
	store.LogS3 = logS3Call
}

type logEntry struct {
	Level     string `json:"level"`
	Event     string `json:"event"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
	Status    int    `json:"status,omitempty"`
	DurationMs int64 `json:"duration_ms,omitempty"`
	Error     string `json:"error,omitempty"`
	Op        string `json:"op,omitempty"`
	Bucket    string `json:"bucket,omitempty"`
	Key       string `json:"key,omitempty"`
	Time      string `json:"time"`
}

func logJSON(e logEntry) {
	e.Time = time.Now().UTC().Format(time.RFC3339Nano)
	json.NewEncoder(os.Stdout).Encode(e)
}

func logS3Call(op, bucket, key string, dur time.Duration, err error) {
	e := logEntry{Level: "debug", Event: "s3_call", Op: op, Bucket: bucket, Key: key, DurationMs: dur.Milliseconds()}
	if err != nil {
		e.Error = err.Error()
	}
	logJSON(e)
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
		e := logEntry{
			Level:      level,
			Event:      "http_request",
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     lw.status,
			DurationMs: dur.Milliseconds(),
		}
		logJSON(e)
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
