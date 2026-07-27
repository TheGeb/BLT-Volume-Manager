package volume

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type mockBackend struct {
	objects    []backend.Entry
	objectsErr error
}

func (m *mockBackend) PutObject(context.Context, string, []byte) error { return nil }
func (m *mockBackend) ReadObject(context.Context, string) ([]byte, error) {
	return nil, backend.ErrKeyNotFound
}
func (m *mockBackend) DeleteObject(context.Context, string) error { return nil }
func (m *mockBackend) ListObjects(context.Context, string) ([]backend.Entry, error) {
	return m.objects, m.objectsErr
}
func (m *mockBackend) DeleteObjectsWithPrefix(context.Context, string) error { return nil }

func mockStores(objects []backend.Entry, objectsErr error) func(*server.BLTService) {
	return func(s *server.BLTService) {
		b := &mockBackend{objects: objects, objectsErr: objectsErr}
		s.SetStores(nil, store.NewRegisteredVolumeStore(b), nil, nil)
	}
}

func volumeObjects(names ...string) []backend.Entry {
	var objs []backend.Entry
	for _, n := range names {
		key := store.RegisteredVolumeKeyspace + n + ".json"
		objs = append(objs, backend.Entry{Key: &key})
	}
	return objs
}

func TestListVolumes(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{
		Config: cfg.Config{S3Bucket: "test-bucket"},
	}
	mockStores(volumeObjects("vol-a", "vol-b", "group/nested"), nil)(s)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	ListVolumes(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Volumes []string `json:"volumes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(resp.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d: %v", len(resp.Volumes), resp.Volumes)
	}

	seen := make(map[string]bool)
	for _, v := range resp.Volumes {
		seen[v] = true
	}
	for _, name := range []string{"vol-a", "vol-b", "group/nested"} {
		if !seen[name] {
			t.Errorf("expected volume %q in response", name)
		}
	}
}

func TestListVolumesEmpty(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{
		Config: cfg.Config{S3Bucket: "test-bucket"},
	}
	mockStores(nil, nil)(s)

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	ListVolumes(s, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Volumes []string `json:"volumes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if resp.Volumes == nil {
		t.Error("expected non-nil volumes slice")
	}
	if len(resp.Volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(resp.Volumes))
	}
}

func TestListVolumesMethodNotAllowed(t *testing.T) {
	t.Parallel()
	s := &server.BLTService{
		Config: cfg.Config{S3Bucket: "test-bucket"},
	}
	mockStores(volumeObjects("vol-a"), nil)(s)

	req := httptest.NewRequest(http.MethodPost, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	ListVolumes(s, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestValidVolumeName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", false},
		{"simple", "my-volume", true},
		{"with group", "group/my-volume", true},
		{"nested group", "a/b/c", true},
		{"backslash", "bad\\name", false},
		{"dotdot", "../etc", false},
		{"dotdot middle", "a/../b", false},
		{"just dotdot", "..", false},
		{"single char", "a", true},
		{"hyphens", "my-volume-name", true},
		{"underscores", "my_volume", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validVolumeName(tt.input)
			if got != tt.want {
				t.Errorf("validVolumeName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}


