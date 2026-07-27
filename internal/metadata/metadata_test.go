package metadata_test

import (
	"context"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
)

type MockBackend struct {
	Objects    []backend.Entry
	ObjectsErr error
	ListFunc   func(ctx context.Context, prefix string) ([]backend.Entry, error)
}

func (m *MockBackend) PutObject(context.Context, string, []byte) error {
	return nil
}

func (m *MockBackend) ReadObject(context.Context, string) ([]byte, error) {
	return nil, backend.ErrKeyNotFound
}

func (m *MockBackend) DeleteObject(context.Context, string) error {
	return nil
}

func (m *MockBackend) ListObjects(ctx context.Context, prefix string) ([]backend.Entry, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, prefix)
	}
	return m.Objects, m.ObjectsErr
}

func (m *MockBackend) DeleteObjectsWithPrefix(context.Context, string) error {
	return nil
}

func sPtr(s string) *string { return &s }

func TestListRegisteredVolumes(t *testing.T) {
	t.Parallel()
	st := store.NewRegisteredVolumeStore(&MockBackend{
		ListFunc: func(ctx context.Context, prefix string) ([]backend.Entry, error) {
			return nil, nil
		},
	})
	names, err := st.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestListRegisteredVolumesWithVolumes(t *testing.T) {
	t.Parallel()
	st := store.NewRegisteredVolumeStore(&MockBackend{
		ListFunc: func(ctx context.Context, prefix string) ([]backend.Entry, error) {
			return []backend.Entry{
				{Key: sPtr(store.RegisteredVolumeKeyspace + "vol-a.json")},
				{Key: sPtr(store.RegisteredVolumeKeyspace + "vol-b.json")},
				{Key: sPtr(store.RegisteredVolumeKeyspace + "deep/nested-vol.json")},
			}, nil
		},
	})
	names, err := st.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3, got %d: %v", len(names), names)
	}
	expected := map[string]bool{"vol-a": true, "vol-b": true, "deep/nested-vol": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected volume %q", n)
		}
	}
}

func TestListRegisteredVolumesSkipsNilKey(t *testing.T) {
	t.Parallel()
	st := store.NewRegisteredVolumeStore(&MockBackend{
		ListFunc: func(ctx context.Context, prefix string) ([]backend.Entry, error) {
			return []backend.Entry{
				{Key: nil},
				{Key: sPtr(store.RegisteredVolumeKeyspace + "valid.json")},
			}, nil
		},
	})
	names, err := st.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "valid" {
		t.Errorf("expected [valid], got %v", names)
	}
}

func TestListRegisteredVolumesSkipsEmptyName(t *testing.T) {
	t.Parallel()
	st := store.NewRegisteredVolumeStore(&MockBackend{
		ListFunc: func(ctx context.Context, prefix string) ([]backend.Entry, error) {
			return []backend.Entry{
				{Key: sPtr(store.RegisteredVolumeKeyspace + ".json")},
			}, nil
		},
	})
	names, err := st.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestHostname(t *testing.T) {
	t.Parallel()
	h := metadata.Hostname()
	if h == "" {
		t.Error("expected non-empty hostname")
	}
}
