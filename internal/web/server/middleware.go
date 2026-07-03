package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
)

const gzipMinSize = 1024

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(lw, r)
		dur := time.Since(start)
		level := log.LevelDebug
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, ".js") {
			level = log.LevelTrace
		}
		log.LogEvent(level, log.Event{
			Event:      "http_request",
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     lw.status,
			DurationMs: dur.Milliseconds(),
		})
	})
}

func NoSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") { //TODO: should gzip be limited to API?
			next.ServeHTTP(w, r)
			return
		}
		buf := &bufferResponseWriter{ResponseWriter: w, buf: &bytes.Buffer{}, code: http.StatusOK}
		next.ServeHTTP(buf, r)
		if buf.code >= 300 || buf.buf.Len() < gzipMinSize {
			w.WriteHeader(buf.code)
			if _, err := w.Write(buf.buf.Bytes()); err != nil {
				log.Error("write_gzip_passthrough_failed", err)
			}
			return
		}
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(buf.code)
		gz := gzip.NewWriter(w)
		if _, err := gz.Write(buf.buf.Bytes()); err != nil {
			log.Error("write_gzip_failed", err)
		}
		if err := gz.Close(); err != nil {
			log.Error("close_gzip_writer_failed", err)
		}
	})
}

type bufferResponseWriter struct {
	http.ResponseWriter
	buf  *bytes.Buffer
	code int
}

func (b *bufferResponseWriter) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

func (b *bufferResponseWriter) WriteHeader(code int) {
	b.code = code
}

func (b *bufferResponseWriter) Flush() {}

func (b *bufferResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, fmt.Errorf("gzip middleware does not support hijacking")
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}
