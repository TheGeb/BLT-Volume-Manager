package owner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func TestHandleOwnerAction_GetNoBackend(t *testing.T) {
	s := &server.Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/volume/test/owner", nil)
	rec := httptest.NewRecorder()
	HandleOwnerAction(s, rec, req, "test")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleOwnerAction_DeleteNoBackend(t *testing.T) {
	s := &server.Server{}
	req := httptest.NewRequest(http.MethodDelete, "/api/volume/test/owner", nil)
	rec := httptest.NewRecorder()
	HandleOwnerAction(s, rec, req, "test")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleOwnerAction_MethodNotAllowed(t *testing.T) {
	s := &server.Server{}
	req := httptest.NewRequest(http.MethodPut, "/api/volume/test/owner", nil)
	rec := httptest.NewRecorder()
	HandleOwnerAction(s, rec, req, "test")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleOwnerAction_PostInvalidJSON(t *testing.T) {
	s := &server.Server{}
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/volume/test/owner", body)
	rec := httptest.NewRecorder()
	HandleOwnerAction(s, rec, req, "test")

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

func TestHandleVolumeOwners_NoBackend(t *testing.T) {
	s := &server.Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/owners", nil)
	rec := httptest.NewRecorder()
	HandleVolumeOwners(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Owners map[string]any `json:"owners"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp.Owners == nil {
		t.Error("expected non-nil owners map")
	}
	if len(resp.Owners) != 0 {
		t.Errorf("expected empty owners, got %d", len(resp.Owners))
	}
}

func TestHandleVolumeOwners_MethodNotAllowed(t *testing.T) {
	s := &server.Server{}
	req := httptest.NewRequest(http.MethodPost, "/api/owners", nil)
	rec := httptest.NewRecorder()
	HandleVolumeOwners(s, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
