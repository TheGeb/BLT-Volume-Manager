package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func TestVersionRoute_ReturnsVersionInfo(t *testing.T) {
	s := &server.Service{
		Config: cfg.Config{
			MetadataBackend: "s3",
			S3Endpoint:      "https://s3.example.com",
			S3Bucket:        "my-bucket",
		},
	}
	mux := http.NewServeMux()
	registerVersionRoute(s, mux)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if resp["version"] != "dev" {
		t.Errorf("expected version 'dev', got %v", resp["version"])
	}
	if resp["metadata_backend"] != "s3" {
		t.Errorf("expected metadata_backend 's3', got %v", resp["metadata_backend"])
	}
	if resp["s3_endpoint"] != "https://s3.example.com" {
		t.Errorf("expected s3_endpoint 'https://s3.example.com', got %v", resp["s3_endpoint"])
	}
	if resp["s3_bucket"] != "my-bucket" {
		t.Errorf("expected s3_bucket 'my-bucket', got %v", resp["s3_bucket"])
	}
}

func TestVersionRoute_DefaultsToS3(t *testing.T) {
	s := &server.Service{
		Config: cfg.Config{
			S3Bucket: "auto-bucket",
		},
	}
	mux := http.NewServeMux()
	registerVersionRoute(s, mux)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if resp["metadata_backend"] != "s3" {
		t.Errorf("expected metadata_backend 's3' when empty but S3Bucket set, got %v", resp["metadata_backend"])
	}
}

func TestVersionRoute_NoneBackend(t *testing.T) {
	s := &server.Service{
		Config: cfg.Config{
			S3Bucket: "",
		},
	}
	mux := http.NewServeMux()
	registerVersionRoute(s, mux)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if resp["metadata_backend"] != "none" {
		t.Errorf("expected metadata_backend 'none', got %v", resp["metadata_backend"])
	}
}

func TestVersionRoute_EtcdEndpoints(t *testing.T) {
	s := &server.Service{
		Config: cfg.Config{
			MetadataBackend: "etcd",
			EtcdEndpoints:   []string{"http://10.0.0.1:2379", "http://10.0.0.2:2379"},
		},
	}
	mux := http.NewServeMux()
	registerVersionRoute(s, mux)

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if resp["metadata_backend"] != "etcd" {
		t.Errorf("expected metadata_backend 'etcd', got %v", resp["metadata_backend"])
	}

	endpoints, ok := resp["etcd_endpoints"].([]any)
	if !ok {
		t.Fatal("expected etcd_endpoints to be an array")
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
	if endpoints[0] != "http://10.0.0.1:2379" {
		t.Errorf("expected first endpoint 'http://10.0.0.1:2379', got %v", endpoints[0])
	}
	if endpoints[1] != "http://10.0.0.2:2379" {
		t.Errorf("expected second endpoint 'http://10.0.0.2:2379', got %v", endpoints[1])
	}
}
