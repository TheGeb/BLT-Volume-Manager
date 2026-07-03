package metadata

import (
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

func sPtr(s string) *string { return &s }

func TestListRegisteredVolumes(t *testing.T) {
	st := NewVolumeStore(&MockKeyValueStore{
		ListFunc: func(prefix string) ([]backend.Entry, error) {
			return nil, nil
		},
	})
	names, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestListRegisteredVolumesWithVolumes(t *testing.T) {
	st := NewVolumeStore(&MockKeyValueStore{
		ListFunc: func(prefix string) ([]backend.Entry, error) {
			return []backend.Entry{
				{Key: sPtr(RegisteredVolumesPrefix + "vol-a.json")},
				{Key: sPtr(RegisteredVolumesPrefix + "vol-b.json")},
				{Key: sPtr(RegisteredVolumesPrefix + "deep/nested-vol.json")},
			}, nil
		},
	})
	names, err := st.List()
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
	st := NewVolumeStore(&MockKeyValueStore{
		ListFunc: func(prefix string) ([]backend.Entry, error) {
			return []backend.Entry{
				{Key: nil},
				{Key: sPtr(RegisteredVolumesPrefix + "valid.json")},
			}, nil
		},
	})
	names, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "valid" {
		t.Errorf("expected [valid], got %v", names)
	}
}

func TestListRegisteredVolumesSkipsEmptyName(t *testing.T) {
	st := NewVolumeStore(&MockKeyValueStore{
		ListFunc: func(prefix string) ([]backend.Entry, error) {
			return []backend.Entry{
				{Key: sPtr(RegisteredVolumesPrefix + ".json")},
			}, nil
		},
	})
	names, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestHostname(t *testing.T) {
	h := Hostname()
	if h == "" {
		t.Error("expected non-empty hostname")
	}
}
