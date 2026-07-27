package owner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func TestOwnerRouter_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{}
	req := httptest.NewRequest(http.MethodPut, "/api/volume/test/owner", nil)
	rec := httptest.NewRecorder()
	OwnerRouter(s, rec, req, "test")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestOwnerRouter_PostInvalidJSON(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{}
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/volume/test/owner", body)
	rec := httptest.NewRecorder()
	OwnerRouter(s, rec, req, "test")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp["error"] == "" {
		t.Error("expected error message")
	}
}

func TestListVolumeOwners_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{}
	req := httptest.NewRequest(http.MethodPost, "/api/owners", nil)
	rec := httptest.NewRecorder()
	ListVolumeOwners(s, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
