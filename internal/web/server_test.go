package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type mockS3Store struct {
	volumes     []string
	volumesErr  error
}

func (m *mockS3Store) PutObject(string, []byte) error                        { return nil }
func (m *mockS3Store) ReadObject(string) ([]byte, error)                     { return nil, nil }
func (m *mockS3Store) DeleteObject(string) error                             { return nil }
func (m *mockS3Store) ListObjects(string) ([]types.Object, error)            { return nil, nil }
func (m *mockS3Store) ListCommonPrefixes(string, string) ([]string, error)   { return nil, nil }
func (m *mockS3Store) DeleteObjectsWithPrefix(string) error                  { return nil }
func (m *mockS3Store) WriteVolumeMarker(string) error                        { return nil }
func (m *mockS3Store) DeleteVolumeMarker(string) error                       { return nil }
func (m *mockS3Store) ListVolumeMarkers() ([]string, error)                  { return m.volumes, m.volumesErr }
func (m *mockS3Store) DeleteLockObjects() error                              { return nil }
func (m *mockS3Store) WriteRestorePoint(string, store.RestorePoint) error    { return nil }
func (m *mockS3Store) ReadRestorePoint(string) (*store.RestorePoint, error)  { return nil, nil }
func (m *mockS3Store) DeleteRestorePoint(string) error                       { return nil }

var _ store.S3Store = (*mockS3Store)(nil)

func TestHandleVolumes(t *testing.T) {
	s := &Server{
		s3Bucket:     "test-bucket",
		s3StoreCache: map[string]store.S3Store{
			store.VolumePrefix: &mockS3Store{volumes: []string{"vol-a", "vol-b", "group/nested"}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	s.handleVolumes(rec, req)

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

func TestHandleVolumesEmpty(t *testing.T) {
	s := &Server{
		s3Bucket:     "test-bucket",
		s3StoreCache: map[string]store.S3Store{
			store.VolumePrefix: &mockS3Store{volumes: []string{}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	s.handleVolumes(rec, req)

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

func TestHandleVolumesMethodNotAllowed(t *testing.T) {
	s := &Server{
		s3Bucket:     "test-bucket",
		s3StoreCache: map[string]store.S3Store{
			store.VolumePrefix: &mockS3Store{volumes: []string{"vol-a"}},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	s.handleVolumes(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleVolumesNoS3(t *testing.T) {
	s := &Server{s3Bucket: ""}

	req := httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec := httptest.NewRecorder()
	s.handleVolumes(rec, req)

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
