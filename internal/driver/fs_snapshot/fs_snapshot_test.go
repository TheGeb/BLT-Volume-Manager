package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
)

func TestTypeString(t *testing.T) {
	t.Parallel()
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

func TestFromString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s        string
		expected Type
	}{
		{"btrfs", TypeBtrfs},
		{"zfs", TypeZFS},
		{"", TypeNone},
		{"ext4", TypeNone},
		{"BTRFS", TypeNone},
		{"ZFS", TypeNone},
		{"btr", TypeNone},
	}

	for _, tc := range cases {
		got := FromString(tc.s)
		if got != tc.expected {
			t.Errorf("FromString(%q) = %d, want %d", tc.s, got, tc.expected)
		}
	}
}

func TestListOrphaned_NoDir(t *testing.T) {
	t.Parallel()
	snaps, err := ListOrphaned(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("ListOrphaned nonexistent dir: %v", err)
	}
	if snaps != nil {
		t.Errorf("expected nil for nonexistent dir, got %d items", len(snaps))
	}
}

func TestListOrphaned_EmptyDir(t *testing.T) {
	t.Parallel()
	snaps, err := ListOrphaned(t.TempDir())
	if err != nil {
		t.Fatalf("ListOrphaned empty dir: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestListOrphaned_FindsColdSnaps(t *testing.T) {
	t.Parallel()
	snapDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(snapDir, "vol1"+ColdSuffix), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(snapDir, "vol2"+ColdSuffix), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(snapDir, "regular-dir"), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "some-file.txt"), nil, app.DefaultFilePerm); err != nil {
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
	t.Parallel()
	snapDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(snapDir, "test-vol"+ColdSuffix), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}

	snaps, err := ListOrphaned(snapDir)
	if err != nil {
		t.Fatalf("ListOrphaned: %v", err)
	}

	want := filepath.Join(snapDir, "test-vol"+ColdSuffix)
	if snaps[0].AccessPath != want {
		t.Errorf("AccessPath = %q, want %q", snaps[0].AccessPath, want)
	}
	if snaps[0].SnapDir != snapDir {
		t.Errorf("SnapDir = %q, want %q", snaps[0].SnapDir, snapDir)
	}
}

func TestResolveType_DetectNonExistent(t *testing.T) {
	t.Parallel()
	info := &Info{AccessPath: filepath.Join(t.TempDir(), "nonexistent")}
	err := ResolveType(info)
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestInfoZeroValue(t *testing.T) {
	t.Parallel()
	info := &Info{}
	if info.Subtype != TypeNone {
		t.Errorf("expected TypeNone, got %d", info.Subtype)
	}
}

func TestRemove_NoneType(t *testing.T) {
	t.Parallel()
	err := Remove(&Info{Subtype: TypeNone})
	if err != nil {
		t.Errorf("expected no error for TypeNone, got %v", err)
	}
}
