package repo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func TestHandleRepoInit_MissingVolume(t *testing.T) {
	s := &server.Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/repo/init", nil)
	rec := httptest.NewRecorder()
	HandleRepoInit(s, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRepoInit_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/init?volume=test", nil)
	rec := httptest.NewRecorder()
	HandleRepoInit(&server.Server{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleRepoStatus_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/status", nil)
	rec := httptest.NewRecorder()
	HandleRepoStatus(&server.Server{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRepoStatus_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repo/status?volume=test", nil)
	rec := httptest.NewRecorder()
	HandleRepoStatus(&server.Server{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleStats_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	HandleStats(&server.Server{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleStats_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stats?volume=test", nil)
	rec := httptest.NewRecorder()
	HandleStats(&server.Server{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleStatsRefresh(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stats/refresh", nil)
	rec := httptest.NewRecorder()
	HandleStatsRefresh(&server.Server{}, rec, req)

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

func TestHandleStatsRefresh_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/stats/refresh", nil)
	rec := httptest.NewRecorder()
	HandleStatsRefresh(&server.Server{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleCheck_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repo/check", nil)
	rec := httptest.NewRecorder()
	HandleCheck(&server.Server{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCheck_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/check?volume=test", nil)
	rec := httptest.NewRecorder()
	HandleCheck(&server.Server{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleRepair_MissingVolume(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repo/repair", nil)
	rec := httptest.NewRecorder()
	HandleRepair(&server.Server{}, rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRepair_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repo/repair?volume=test", nil)
	rec := httptest.NewRecorder()
	HandleRepair(&server.Server{}, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
