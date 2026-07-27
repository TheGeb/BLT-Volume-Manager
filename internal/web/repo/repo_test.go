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

func TestInitRepo_MissingVolume(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{}
	req := httptest.NewRequest(http.MethodPost, "/api/repo/init", nil)
	rec := httptest.NewRecorder()
	InitRepo(s, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestInitRepo_WrongMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/repo/init?volume=test", nil)
	rec := httptest.NewRecorder()
	InitRepo(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestRepoStatus_MissingVolume(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/repo/status", nil)
	rec := httptest.NewRecorder()
	RepoStatus(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRepoStatus_WrongMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/api/repo/status?volume=test", nil)
	rec := httptest.NewRecorder()
	RepoStatus(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestStats_MissingVolume(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	Stats(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestStats_WrongMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/api/stats?volume=test", nil)
	rec := httptest.NewRecorder()
	Stats(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
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

func TestCheckRepo_MissingVolume(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/api/repo/check", nil)
	rec := httptest.NewRecorder()
	CheckRepo(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCheckRepo_WrongMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/repo/check?volume=test", nil)
	rec := httptest.NewRecorder()
	CheckRepo(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestRepairRepo_MissingVolume(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/api/repo/repair", nil)
	rec := httptest.NewRecorder()
	RepairRepo(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRepairRepo_WrongMethod(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/repo/repair?volume=test", nil)
	rec := httptest.NewRecorder()
	RepairRepo(&server.BLTService{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
