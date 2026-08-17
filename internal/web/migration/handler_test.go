package migration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func TestBackendSpecConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		spec    backendSpec
		wantErr bool
	}{
		{"s3 minimal", backendSpec{Type: "s3", Bucket: "meta"}, false},
		{"s3 full", backendSpec{Type: "s3", Bucket: "meta", Endpoint: "https://s3.example.com", Region: "us-east-1", ForcePathStyle: boolPtr(false)}, false},
		{"s3 no bucket", backendSpec{Type: "s3"}, true},
		{"etcd", backendSpec{Type: "etcd", EtcdEndpoints: []string{"http://127.0.0.1:2379"}}, false},
		{"etcd no endpoints", backendSpec{Type: "etcd"}, true},
		{"unknown type", backendSpec{Type: "redis"}, true},
		{"empty", backendSpec{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.spec.config()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.MetadataBackend != tt.spec.Type {
				t.Errorf("MetadataBackend = %q, want %q", got.MetadataBackend, tt.spec.Type)
			}
		})
	}
}

func TestBackendSpecConfigForcePathDefault(t *testing.T) {
	t.Parallel()
	c, err := backendSpec{Type: "s3", Bucket: "meta"}.config()
	if err != nil {
		t.Fatal(err)
	}
	if !c.S3ForcePathStyle {
		t.Error("force_path_style should default to true")
	}
	c, err = backendSpec{Type: "s3", Bucket: "meta", ForcePathStyle: boolPtr(false)}.config()
	if err != nil {
		t.Fatal(err)
	}
	if c.S3ForcePathStyle {
		t.Error("force_path_style should honor the provided value")
	}
}

func TestRunMigrationValidation(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{Config: cfg.Config{}}

	tests := []struct {
		name string
		body string
		code int
	}{
		{"invalid json", `{`, http.StatusBadRequest},
		{"unknown backend type", `{"from":{"type":"redis"},"to":{"type":"s3","bucket":"meta"}}`, http.StatusBadRequest},
		{"s3 missing bucket", `{"from":{"type":"s3"},"to":{"type":"s3","bucket":"meta"}}`, http.StatusBadRequest},
		{"etcd missing endpoints", `{"from":{"type":"s3","bucket":"meta"},"to":{"type":"etcd"}}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/migrate", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			RunMigration(s, rec, req)
			if rec.Code != tt.code {
				t.Errorf("RunMigration(%q) = %d, want %d", tt.body, rec.Code, tt.code)
			}
		})
	}
}

func TestRunMigrationMethodNotAllowed(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{Config: cfg.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/api/migrate", nil)
	rec := httptest.NewRecorder()
	RunMigration(s, rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func boolPtr(b bool) *bool { return &b }
