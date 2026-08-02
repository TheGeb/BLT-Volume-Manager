package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type noopBackend struct{}

func (noopBackend) PutObject(context.Context, string, []byte) error { return nil }
func (noopBackend) ReadObject(context.Context, string) ([]byte, error) {
	return nil, store.ErrKeyNotFound
}
func (noopBackend) DeleteObject(context.Context, string) error { return nil }
func (noopBackend) ListObjects(context.Context, string) ([]s3.Object, error) {
	return nil, nil
}
func (noopBackend) DeleteObjectsWithPrefix(context.Context, string) error { return nil }

func TestRepoRouteGuards(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{}
	// Each handler rejects a wrong HTTP method (405) and a missing volume
	// parameter (400) independent of other input, so we table-drive both.
	tests := []struct {
		name     string
		handler  func(*server.BLTService, http.ResponseWriter, *http.Request)
		method   string
		path     string
		wantCode int
	}{
		{"InitRepo-wrong-method", InitRepo, "GET", "/api/repo/init?volume=test", http.StatusMethodNotAllowed},
		{"InitRepo-missing-volume", InitRepo, "POST", "/api/repo/init", http.StatusBadRequest},
		{"RepoStatus-wrong-method", RepoStatus, "POST", "/api/repo/status?volume=test", http.StatusMethodNotAllowed},
		{"RepoStatus-missing-volume", RepoStatus, "GET", "/api/repo/status", http.StatusBadRequest},
		{"Stats-wrong-method", Stats, "POST", "/api/stats?volume=test", http.StatusMethodNotAllowed},
		{"Stats-missing-volume", Stats, "GET", "/api/stats", http.StatusBadRequest},
		{"CheckRepo-wrong-method", CheckRepo, "GET", "/api/repo/check?volume=test", http.StatusMethodNotAllowed},
		{"CheckRepo-missing-volume", CheckRepo, "POST", "/api/repo/check", http.StatusBadRequest},
		{"RepairRepo-wrong-method", RepairRepo, "GET", "/api/repo/repair?volume=test", http.StatusMethodNotAllowed},
		{"RepairRepo-missing-volume", RepairRepo, "POST", "/api/repo/repair", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.handler(s, rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("handler = %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestRefreshStats(t *testing.T) {
	t.Parallel()
	var b noopBackend
	s := server.New(cfg.Config{}, b)
	req := httptest.NewRequest(http.MethodPost, "/api/stats/refresh", nil)
	rec := httptest.NewRecorder()
	RefreshStats(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp["total_volumes"] == nil {
		t.Error("expected total_volumes in response")
	}
}

func TestRefreshStats_WrongMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/stats/refresh", nil)
	rec := httptest.NewRecorder()
	RefreshStats(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
