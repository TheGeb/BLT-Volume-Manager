package metadata

import "testing"

func sPtr(s string) *string { return &s }

type mockObjectStore struct {
	listFunc func(prefix string) ([]Object, error)
}

func (m *mockObjectStore) PutObject(string, []byte) error                      { return nil }
func (m *mockObjectStore) ReadObject(string) ([]byte, error)                   { return nil, nil }
func (m *mockObjectStore) DeleteObject(string) error                           { return nil }
func (m *mockObjectStore) ListObjects(prefix string) ([]Object, error)         { return m.listFunc(prefix) }
func (m *mockObjectStore) ListCommonPrefixes(string, string) ([]string, error) { return nil, nil }
func (m *mockObjectStore) DeleteObjectsWithPrefix(string) error                { return nil }

func TestListVolumeMarkers(t *testing.T) {
	st := New(&mockObjectStore{
		listFunc: func(prefix string) ([]Object, error) {
			return nil, nil
		},
	})
	names, err := st.ListVolumeMarkers("prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestListVolumeMarkersWithVolumes(t *testing.T) {
	st := New(&mockObjectStore{
		listFunc: func(prefix string) ([]Object, error) {
			return []Object{
				{Key: sPtr("prefix/vol-a.json")},
				{Key: sPtr("prefix/vol-b.json")},
				{Key: sPtr("prefix/deep/nested-vol.json")},
			}, nil
		},
	})
	names, err := st.ListVolumeMarkers("prefix/")
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

func TestListVolumeMarkersSkipsNilKey(t *testing.T) {
	st := New(&mockObjectStore{
		listFunc: func(prefix string) ([]Object, error) {
			return []Object{
				{Key: nil},
				{Key: sPtr("prefix/valid.json")},
			}, nil
		},
	})
	names, err := st.ListVolumeMarkers("prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "valid" {
		t.Errorf("expected [valid], got %v", names)
	}
}

func TestListVolumeMarkersSkipsEmptyName(t *testing.T) {
	st := New(&mockObjectStore{
		listFunc: func(prefix string) ([]Object, error) {
			return []Object{
				{Key: sPtr("prefix/.json")},
			}, nil
		},
	})
	names, err := st.ListVolumeMarkers("prefix/")
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
