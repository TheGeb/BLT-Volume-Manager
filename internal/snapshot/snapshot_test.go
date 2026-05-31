package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
)

func TestTypeString(t *testing.T) {
	cases := []struct {
		typ      Type
		expected string
	}{
		{TypeNone, ""},
		{TypeBtrfs, "btrfs"},
		{TypeZFS, "zfs"},
		{Type(100), ""},
	}

	for _, tc := range cases {
		got := tc.typ.String()
		if got != tc.expected {
			t.Errorf("Type(%d).String() = %q, want %q", tc.typ, got, tc.expected)
		}
	}
}

func TestListOrphaned_NoDir(t *testing.T) {
	snaps, err := ListOrphaned(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("ListOrphaned nonexistent dir: %v", err)
	}
	if snaps != nil {
		t.Errorf("expected nil for nonexistent dir, got %d items", len(snaps))
	}
}

func TestListOrphaned_EmptyDir(t *testing.T) {
	snaps, err := ListOrphaned(t.TempDir())
	if err != nil {
		t.Fatalf("ListOrphaned empty dir: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestListOrphaned_FindsColdSnaps(t *testing.T) {
	snapDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(snapDir, "vol1"+constants.ColdSnapSuffix), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(snapDir, "vol2"+constants.ColdSnapSuffix), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(snapDir, "regular-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "some-file.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	snaps, err := ListOrphaned(snapDir)
	if err != nil {
		t.Fatalf("ListOrphaned: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}

	found := map[string]bool{}
	for _, s := range snaps {
		found[s.VolName] = true
	}
	if !found["vol1"] {
		t.Error("expected vol1")
	}
	if !found["vol2"] {
		t.Error("expected vol2")
	}
}

func TestListOrphaned_AccessPath(t *testing.T) {
	snapDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(snapDir, "test-vol"+constants.ColdSnapSuffix), 0o755); err != nil {
		t.Fatal(err)
	}

	snaps, err := ListOrphaned(snapDir)
	if err != nil {
		t.Fatalf("ListOrphaned: %v", err)
	}

	want := filepath.Join(snapDir, "test-vol"+constants.ColdSnapSuffix)
	if snaps[0].AccessPath != want {
		t.Errorf("AccessPath = %q, want %q", snaps[0].AccessPath, want)
	}
	if snaps[0].SnapDir != snapDir {
		t.Errorf("SnapDir = %q, want %q", snaps[0].SnapDir, snapDir)
	}
}

func TestResolveType_DetectNonExistent(t *testing.T) {
	info := &SnapInfo{AccessPath: filepath.Join(t.TempDir(), "nonexistent")}
	err := ResolveType(info)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestSnapInfoZeroValue(t *testing.T) {
	info := &SnapInfo{}
	if info.Subtype != TypeNone {
		t.Errorf("expected TypeNone, got %d", info.Subtype)
	}
}

func TestRemove_NoneType(t *testing.T) {
	err := Remove(&SnapInfo{Subtype: TypeNone})
	if err != nil {
		t.Errorf("expected no error for TypeNone, got %v", err)
	}
}
