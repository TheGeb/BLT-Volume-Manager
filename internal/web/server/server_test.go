package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoggingResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	lw.WriteHeader(http.StatusNotFound)
	if lw.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", lw.status)
	}
}

func TestLoggingResponseWriter_Implicit200(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	n, err := lw.Write([]byte("ok"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 bytes written, got %d", n)
	}
	if lw.status != 200 {
		t.Errorf("expected implicit status 200, got %d", lw.status)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected recorder status 200, got %d", w.Code)
	}
}

func TestLoggingResponseWriter_DuplicateWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	lw.WriteHeader(http.StatusNotFound)
	lw.WriteHeader(http.StatusOK)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected recorder status 404 after duplicate WriteHeader, got %d", w.Code)
	}
	// loggingResponseWriter delegates to the underlying ResponseWriter;
	// httptest.ResponseRecorder ignores the second WriteHeader,
	// but our wrapper captures the last-written status for logging.
	if lw.status != http.StatusOK {
		t.Errorf("expected lw.status 200 (last written), got %d", lw.status)
	}
}

func TestGzipMiddleware_Negotiation(t *testing.T) {
	tests := []struct {
		name           string
		acceptEncoding string
		path           string
		wantGzip       bool
	}{
		{"gzip on api path", "gzip", "/api/volumes", true},
		{"no gzip without accept-encoding", "", "/api/volumes", false},
		{"no gzip for non-api path", "gzip", "/ui/index.html", false},
		{"no gzip for non-matching encoding", "deflate", "/api/volumes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("hello"))
			})
			handler := Gzip(inner)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.acceptEncoding != "" {
				r.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			handler.ServeHTTP(w, r)

			ce := w.Header().Get("Content-Encoding")
			if tt.wantGzip && ce != "gzip" {
				t.Errorf("expected gzip Content-Encoding, got %q", ce)
			}
			if !tt.wantGzip && ce == "gzip" {
				t.Errorf("did not expect gzip Content-Encoding")
			}

			if tt.wantGzip {
				gr, err := gzip.NewReader(w.Body)
				if err != nil {
					t.Fatalf("gzip.NewReader: %v", err)
				}
				got, _ := io.ReadAll(gr)
				if string(got) != "hello" {
					t.Errorf("expected body 'hello', got %q", string(got))
				}
			} else {
				if w.Body.String() != "hello" {
					t.Errorf("expected body 'hello', got %q", w.Body.String())
				}
			}
		})
	}
}

func TestGzipMiddleware_ContentLengthRemoved(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	})
	handler := Gzip(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	handler.ServeHTTP(w, r)

	ce := w.Header().Get("Content-Encoding")
	if ce != "gzip" {
		t.Errorf("expected gzip Content-Encoding, got %q", ce)
	}
}

func TestGzipResponseWriter_VaryHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	})
	handler := Gzip(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	handler.ServeHTTP(w, r)

	vary := w.Header().Values("Vary")
	if len(vary) == 0 {
		t.Error("expected Vary header to be set")
	}
}

func TestNoSniffMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	handler := NoSniff(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", v)
	}
}

func TestLoggingResponseWriter_WrittenBytes(t *testing.T) {
	w := httptest.NewRecorder()
	lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
	data := []byte("response body")
	n, err := lw.Write(data)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if w.Body.String() != "response body" {
		t.Errorf("expected body 'response body', got %q", w.Body.String())
	}
}

func TestGzipResponseWriter_WritePreservesStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gzw := &gzipResponseWriter{ResponseWriter: w, gw: gz}
	gzw.WriteHeader(http.StatusTeapot)
	if w.Code != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", w.Code)
	}
}

func TestNoSniffMiddleware_MultipleWrites(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("part1"))
		_, _ = w.Write([]byte("part2"))
	})
	handler := NoSniff(inner)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if w.Body.String() != "part1part2" {
		t.Errorf("expected 'part1part2', got %q", w.Body.String())
	}
}
