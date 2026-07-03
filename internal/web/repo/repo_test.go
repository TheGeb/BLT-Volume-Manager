package repo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func TestInitRepo_MissingVolume(t *testing.T) {
	s := &server.Service{}
	req := httptest.NewRequest(http.MethodPost, "/api/repo/init", nil)
	rec := httptest.NewRecorder()
	InitRepo(s, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestInitRepo_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/init?volume=test", nil)
	rec := httptest.NewRecorder()
	InitRepo(&server.Service{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestRepoStatus_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/status", nil)
	rec := httptest.NewRecorder()
	RepoStatus(&server.Service{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRepoStatus_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repo/status?volume=test", nil)
	rec := httptest.NewRecorder()
	RepoStatus(&server.Service{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestStats_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	Stats(&server.Service{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestStats_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stats?volume=test", nil)
	rec := httptest.NewRecorder()
	Stats(&server.Service{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestRefreshStats(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stats/refresh", nil)
	rec := httptest.NewRecorder()
	RefreshStats(&server.Service{}, rec, req)

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
	req := httptest.NewRequest(http.MethodGet, "/api/stats/refresh", nil)
	rec := httptest.NewRecorder()
	RefreshStats(&server.Service{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestCheckRepo_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repo/check", nil)
	rec := httptest.NewRecorder()
	CheckRepo(&server.Service{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCheckRepo_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/check?volume=test", nil)
	rec := httptest.NewRecorder()
	CheckRepo(&server.Service{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestRepairRepo_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repo/repair", nil)
	rec := httptest.NewRecorder()
	RepairRepo(&server.Service{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRepairRepo_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/repair?volume=test", nil)
	rec := httptest.NewRecorder()
	RepairRepo(&server.Service{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
