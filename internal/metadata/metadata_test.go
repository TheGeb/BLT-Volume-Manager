package metadata

import "testing"

func sPtr(s string) *string { return &s }

func TestListRegisteredVolumes(t *testing.T) {
	st := New(&MockObjectStore{
		ListFunc: func(prefix string) ([]Object, error) {
			return nil, nil
		},
	})
	names, err := st.ListRegisteredVolumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestListRegisteredVolumesWithVolumes(t *testing.T) {
	st := New(&MockObjectStore{
		ListFunc: func(prefix string) ([]Object, error) {
			return []Object{
				{Key: sPtr(RegisteredVolumesPrefix + "vol-a.json")},
				{Key: sPtr(RegisteredVolumesPrefix + "vol-b.json")},
				{Key: sPtr(RegisteredVolumesPrefix + "deep/nested-vol.json")},
			}, nil
		},
	})
	names, err := st.ListRegisteredVolumes()
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
	st := New(&MockObjectStore{
		ListFunc: func(prefix string) ([]Object, error) {
			return []Object{
				{Key: nil},
				{Key: sPtr(RegisteredVolumesPrefix + "valid.json")},
			}, nil
		},
	})
	names, err := st.ListRegisteredVolumes()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "valid" {
		t.Errorf("expected [valid], got %v", names)
	}
}

func TestListRegisteredVolumesSkipsEmptyName(t *testing.T) {
	st := New(&MockObjectStore{
		ListFunc: func(prefix string) ([]Object, error) {
			return []Object{
				{Key: sPtr(RegisteredVolumesPrefix + ".json")},
			}, nil
		},
	})
	names, err := st.ListRegisteredVolumes()
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
