package driver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/appconfig"
	"github.com/TheGeb/BLT-Volume-Manager/internal/snapshot"
	"github.com/docker/go-plugins-helpers/volume"
)

func TestTypeFromString(t *testing.T) {
	cases := []struct {
		s        string
		expected snapshot.Type
	}{
		{"btrfs", snapshot.TypeBtrfs},
		{"zfs", snapshot.TypeZFS},
		{"", snapshot.TypeNone},
		{"ext4", snapshot.TypeNone},
		{"BTRFS", snapshot.TypeNone},
		{"ZFS", snapshot.TypeNone},
		{"btr", snapshot.TypeNone},
	}

	for _, tc := range cases {
		got := TypeFromString(tc.s)
		if got != tc.expected {
			t.Errorf("TypeFromString(%q) = %d, want %d", tc.s, got, tc.expected)
		}
	}
}

func TestVolumeConfigReadWrite(t *testing.T) {
	d := &Driver{root: t.TempDir()}
	volPath := filepath.Join(d.root, "volumes", "test-vol")
	if err := os.MkdirAll(volPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := &volumeConfig{FsType: "btrfs"}
	if err := d.writeVolumeConfig(volPath, cfg); err != nil {
		t.Fatalf("writeVolumeConfig: %v", err)
	}

	read := d.readVolumeConfig(volPath)
	if read == nil {
		t.Fatal("readVolumeConfig returned nil")
	}
	if read.FsType != "btrfs" {
		t.Errorf("FsType = %q, want %q", read.FsType, "btrfs")
	}
}

func TestVolumeConfigReadNonExistent(t *testing.T) {
	d := &Driver{root: t.TempDir()}
	missing := d.readVolumeConfig(filepath.Join(d.root, "volumes", "nonexistent"))
	if missing != nil {
		t.Error("expected nil for missing config")
	}
}

func TestVolumeConfigDefaultFsType(t *testing.T) {
	d := &Driver{root: t.TempDir()}
	volPath := filepath.Join(d.root, "volumes", "plain-vol")
	if err := os.MkdirAll(volPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := d.writeVolumeConfig(volPath, &volumeConfig{FsType: ""}); err != nil {
		t.Fatal(err)
	}
	read := d.readVolumeConfig(volPath)
	if read == nil {
		t.Fatal("expected non-nil config")
	}
	if read.FsType != "" {
		t.Errorf("expected empty FsType, got %q", read.FsType)
	}
}

func TestSnapVolumes(t *testing.T) {
	d := &Driver{
		vols: map[string]*VolumeInfo{
			"vol1": {Name: "vol1", FsType: "btrfs"},
			"vol2": {Name: "vol2", FsType: ""},
			"vol3": {Name: "vol3", FsType: "zfs"},
		},
	}

	snaps := d.SnapVolumes()
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snap volumes, got %d", len(snaps))
	}
	if snaps["vol1"] != "btrfs" {
		t.Errorf("vol1 = %q, want btrfs", snaps["vol1"])
	}
	if snaps["vol3"] != "zfs" {
		t.Errorf("vol3 = %q, want zfs", snaps["vol3"])
	}
	if _, ok := snaps["vol2"]; ok {
		t.Error("vol2 should not be in snap volumes (no fs_type)")
	}
}

func TestSnapVolumesEmpty(t *testing.T) {
	d := &Driver{vols: map[string]*VolumeInfo{}}
	snaps := d.SnapVolumes()
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestSnapVolumesNilMap(t *testing.T) {
	d := &Driver{vols: nil}
	snaps := d.SnapVolumes()
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestCollectVolumeNames(t *testing.T) {
	root := t.TempDir()
	d := &Driver{root: root}

	volumesDir := filepath.Join(root, "volumes")

	vol1 := filepath.Join(volumesDir, "my-vol")
	if err := os.MkdirAll(vol1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol1, "volume.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	vol2 := filepath.Join(volumesDir, "group", "nested-vol")
	if err := os.MkdirAll(vol2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol2, "volume.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(volumesDir, "no-config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(volumesDir, ".hidden-vol"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(volumesDir, ".hidden-vol", "volume.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var names []string
	d.collectVolumeNames(volumesDir, "", &names)

	found := make(map[string]bool)
	for _, n := range names {
		found[n] = true
	}

	if !found["my-vol"] {
		t.Error("expected 'my-vol'")
	}
	if !found["group/nested-vol"] {
		t.Error("expected 'group/nested-vol'")
	}
	if found["no-config"] {
		t.Error("did not expect 'no-config' without volume.json")
	}
	if found[".hidden-vol"] {
		t.Error("did not expect '.hidden-vol' (starts with dot)")
	}
}

func TestCollectVolumeNamesEmpty(t *testing.T) {
	root := t.TempDir()
	d := &Driver{root: root}

	var names []string
	d.collectVolumeNames(filepath.Join(root, "volumes"), "", &names)
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestVolumeNames(t *testing.T) {
	root := t.TempDir()
	d := &Driver{root: root}

	for _, name := range []string{"vol-a", "vol-b"} {
		p := filepath.Join(root, "volumes", name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "volume.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	names := d.VolumeNames()
	if len(names) != 2 {
		t.Errorf("expected 2, got %d: %v", len(names), names)
	}
}

func TestResticManager(t *testing.T) {
	d := &Driver{resticBase: "/data"}
	rm := d.ResticManager("test-vol")
	if rm == nil {
		t.Fatal("expected non-nil restic manager")
	}
	if rm.Repo() != "/data/restic/test-vol" {
		t.Errorf("repo = %q, want '/data/restic/test-vol'", rm.Repo())
	}
}

func TestNewDriverDefaults(t *testing.T) {
	d := NewDriver(appconfig.Config{DataDir: t.TempDir(), ResticBase: "/tmp/restic"})
	if d == nil {
		t.Fatal("expected non-nil driver")
	}
	if d.root == "" {
		t.Error("expected non-empty root")
	}
	if d.vols == nil {
		t.Error("expected non-nil vols map")
	}
	if d.locker == nil {
		t.Error("expected non-nil locker")
	}
}

func TestCapabilities(t *testing.T) {
	d := &Driver{}
	cap := d.Capabilities()
	if cap == nil {
		t.Fatal("expected non-nil capabilities")
	}
	if cap.Capabilities.Scope != "local" {
		t.Errorf("expected 'local', got %q", cap.Capabilities.Scope)
	}
}

func TestPath(t *testing.T) {
	root := t.TempDir()
	d := &Driver{root: root}

	resp, err := d.Path(&volume.PathRequest{Name: "test-vol"})
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if resp.Mountpoint != filepath.Join(root, "volumes", "test-vol") {
		t.Errorf("Mountpoint = %q", resp.Mountpoint)
	}
}
