package volume

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type mockObjectStore struct {
	objects    []metadata.Object
	objectsErr error
}

func (m *mockObjectStore) PutObject(string, []byte) error                      { return nil }
func (m *mockObjectStore) ReadObject(string) ([]byte, error)                   { return nil, metadata.ErrKeyNotFound }
func (m *mockObjectStore) DeleteObject(string) error                           { return nil }
func (m *mockObjectStore) ListObjects(string) ([]metadata.Object, error)       { return m.objects, m.objectsErr }
func (m *mockObjectStore) ListCommonPrefixes(string, string) ([]string, error) { return nil, nil }
func (m *mockObjectStore) DeleteObjectsWithPrefix(string) error                { return nil }

func mockStore(objects []metadata.Object, objectsErr error) *metadata.Store {
	return metadata.New(&mockObjectStore{objects: objects, objectsErr: objectsErr})
}

func volumeObjects(names ...string) []metadata.Object {
	var objs []metadata.Object
	for _, n := range names {
		key := metadata.RegisteredVolumesPrefix + n + ".json"
		objs = append(objs, metadata.Object{Key: &key})
	}
	return objs
}

func TestListVolumes(t *testing.T) {
	s := &server.Server{
		Config: cfg.Config{S3Bucket: "test-bucket"},
	}
	s.SetMetadataStore(mockStore(volumeObjects("vol-a", "vol-b", "group/nested"), nil))

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
	s := &server.Server{
		Config: cfg.Config{S3Bucket: "test-bucket"},
	}
	s.SetMetadataStore(mockStore(nil, nil))

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
	s := &server.Server{
		Config: cfg.Config{S3Bucket: "test-bucket"},
	}
	s.SetMetadataStore(mockStore(volumeObjects("vol-a"), nil))

	req := httptest.NewRequest(http.MethodPost, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	ListVolumes(s, rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestListVolumesNoS3(t *testing.T) {
	s := &server.Server{}

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
		t.Errorf("expected 0 volumes (no S3), got %d", len(resp.Volumes))
	}
}
