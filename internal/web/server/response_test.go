package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireMethod_Allowed(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if !RequireMethod(w, r, http.MethodGet) {
		t.Error("expected RequireMethod to return true for matching method")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRequireMethod_NotAllowed(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if RequireMethod(w, r, http.MethodGet) {
		t.Error("expected RequireMethod to return false for mismatched method")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestRequireMethod_PUTvsPOST(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/", nil)
	if RequireMethod(w, r, http.MethodPost) {
		t.Error("expected RequireMethod to return false for PUT vs POST")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestRespondJSON(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	RespondJSON(w, StatusResponse{Status: "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestRespondJSON_Struct(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	type customResp struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	RespondJSON(w, customResp{Name: "test", Count: 42})

	var resp customResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp.Name != "test" || resp.Count != 42 {
		t.Errorf("got %+v, want {Name:test Count:42}", resp)
	}
}

func TestRespondJSON_Nil(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	RespondJSON(w, nil)

	var resp any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp != nil {
		t.Errorf("expected null, got %v", resp)
	}
}

func TestRequireVolumeParam_Present(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?volume=test-vol", nil)
	vol, ok := RequireVolumeParam(w, r)
	if !ok {
		t.Error("expected ok=true for present volume param")
	}
	if vol != "test-vol" {
		t.Errorf("expected 'test-vol', got %q", vol)
	}
}

func TestRequireVolumeParam_Missing(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := RequireVolumeParam(w, r)
	if ok {
		t.Error("expected ok=false for missing volume param")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRequireVolumeParam_EmptyValue(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?volume=", nil)
	_, ok := RequireVolumeParam(w, r)
	if ok {
		t.Error("expected ok=false for empty volume param")
	}
}

func TestRespondError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	RespondError(w, http.ErrNoLocation, http.StatusInternalServerError)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRespondError_NilError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	RespondError(w, nil, http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
