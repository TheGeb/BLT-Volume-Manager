package driver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/docker/go-plugins-helpers/volume"
)

func TestVolumeConfigReadWrite(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	volPath := filepath.Join(d.volumePath, "volumes", "test-vol")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cfg := &volumeConfig{FsType: "btrfs"}
	if err := d.writeVolumeConfig(volPath, cfg); err != nil {
		t.Fatalf("writeVolumeConfig: %v", err)
	}

	read := d.readVolumeConfig(volPath)
	if read == nil {
		t.Fatal("readVolumeConfig returned nil")
		return
	}
	if read.FsType != "btrfs" {
		t.Errorf("FsType = %q, want %q", read.FsType, "btrfs")
	}
}

func TestVolumeConfigReadNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	missing := d.readVolumeConfig(filepath.Join(d.volumePath, "volumes", "nonexistent"))
	if missing != nil {
		t.Error("expected nil for missing config")
	}
}

func TestVolumeConfigDefaultFsType(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	volPath := filepath.Join(d.volumePath, "volumes", "plain-vol")
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}

	if err := d.writeVolumeConfig(volPath, &volumeConfig{FsType: ""}); err != nil {
		t.Fatal(err)
	}
	read := d.readVolumeConfig(volPath)
	if read == nil {
		t.Fatal("expected non-nil config")
		return
	}
	if read.FsType != "" {
		t.Errorf("expected empty FsType, got %q", read.FsType)
	}
}

func TestSnapVolumes(t *testing.T) {
	t.Parallel()
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
	m := make(map[string]string)
	for _, sv := range snaps {
		m[sv.Name] = sv.FsType
	}
	if m["vol1"] != "btrfs" {
		t.Errorf("vol1 = %q, want btrfs", m["vol1"])
	}
	if m["vol3"] != "zfs" {
		t.Errorf("vol3 = %q, want zfs", m["vol3"])
	}
	if _, ok := m["vol2"]; ok {
		t.Error("vol2 should not be in snap volumes (no fs_type)")
	}
}

func TestSnapVolumesEmpty(t *testing.T) {
	t.Parallel()
	d := &Driver{vols: map[string]*VolumeInfo{}}
	snaps := d.SnapVolumes()
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestSnapVolumesNilMap(t *testing.T) {
	t.Parallel()
	d := &Driver{vols: nil}
	snaps := d.SnapVolumes()
	if len(snaps) != 0 {
		t.Errorf("expected 0, got %d", len(snaps))
	}
}

func TestCollectVolumeNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	volumesDir := filepath.Join(root, "volumes")

	vol1 := filepath.Join(volumesDir, "my-vol")
	if err := os.MkdirAll(vol1, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol1, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	vol2 := filepath.Join(volumesDir, "group", "nested-vol")
	if err := os.MkdirAll(vol2, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vol2, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(volumesDir, "no-config"), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(volumesDir, ".hidden-vol"), app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(volumesDir, ".hidden-vol", "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
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
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	var names []string
	d.collectVolumeNames(filepath.Join(root, "volumes"), "", &names)
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestVolumeNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	for _, name := range []string{"vol-a", "vol-b"} {
		p := filepath.Join(root, "volumes", name)
		if err := os.MkdirAll(p, app.DefaultDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
			t.Fatal(err)
		}
	}

	names := d.VolumeNames()
	if len(names) != 2 {
		t.Errorf("expected 2, got %d: %v", len(names), names)
	}
}

func TestResticManager(t *testing.T) {
	t.Parallel()
	d := &Driver{resticPath: "/data"}
	rm := d.ResticManager("test-vol")
	if rm == nil {
		t.Fatal("expected non-nil restic manager")
	}
	if rm.Repo() != "/data/restic/test-vol" {
		t.Errorf("repo = %q, want '/data/restic/test-vol'", rm.Repo())
	}
}

func TestNewDriverDefaults(t *testing.T) {
	t.Parallel()
	d := New(cfg.Config{DataDir: t.TempDir(), ResticBase: "/tmp/restic"}, context.Background())
	if d == nil {
		t.Fatal("expected non-nil driver")
		return
	}
	if d.volumePath == "" {
		t.Error("expected non-empty root")
	}
	if d.vols == nil {
		t.Error("expected non-nil vols map")
	}
	if d.ownerStore != nil {
		t.Error("expected nil ownerStore (no S3)")
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	for _, name := range []string{"vol-a", "vol-b", "group/nested-vol"} {
		p := filepath.Join(root, "volumes", name)
		if err := os.MkdirAll(p, app.DefaultDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "volume.json"), []byte(`{}`), app.DefaultFilePerm); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := d.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
		return
	}
	if len(resp.Volumes) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(resp.Volumes))
	}

	seen := make(map[string]bool)
	for _, v := range resp.Volumes {
		seen[v.Name] = true
		if v.Name != "" && v.Mountpoint == "" {
			t.Errorf("volume %q has empty Mountpoint", v.Name)
		}
	}
	for _, name := range []string{"vol-a", "vol-b", "group/nested-vol"} {
		if !seen[name] {
			t.Errorf("expected volume %q in list", name)
		}
	}
}

func TestListEmpty(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	resp, err := d.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
		return
	}
	if len(resp.Volumes) != 0 {
		t.Errorf("expected 0 volumes, got %d", len(resp.Volumes))
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	cap := d.Capabilities()
	if cap == nil {
		t.Fatal("expected non-nil capabilities")
		return
	}
	if cap.Capabilities.Scope != "local" {
		t.Errorf("expected 'local', got %q", cap.Capabilities.Scope)
	}
}

func TestPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	d := &Driver{volumePath: root}

	resp, err := d.Path(&volume.PathRequest{Name: "test-vol"})
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if resp.Mountpoint != filepath.Join(root, "volumes", "test-vol") {
		t.Errorf("Mountpoint = %q", resp.Mountpoint)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	err := d.Remove(&volume.RemoveRequest{Name: "no-such-vol"})
	if err != nil {
		t.Fatalf("Remove on non-existent volume: %v", err)
	}
}

func TestGetNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{volumePath: t.TempDir()}
	resp, err := d.Get(&volume.GetRequest{Name: "no-such-vol"})
	if err != nil {
		t.Fatalf("Get on non-existent volume: %v", err)
	}
	if resp == nil || resp.Volume == nil {
		t.Fatal("expected non-nil response with Volume")
		return
	}
	if resp.Volume.Name != "no-such-vol" {
		t.Errorf("Name = %q, want %q", resp.Volume.Name, "no-such-vol")
	}
	if resp.Volume.Mountpoint != filepath.Join(d.volumePath, "volumes", "no-such-vol") {
		t.Errorf("Mountpoint = %q", resp.Volume.Mountpoint)
	}
	if state, ok := resp.Volume.Status["state"].(string); !ok || state != "unclaimed" {
		t.Errorf("state = %q, want %q", state, "unclaimed")
	}
}

func TestUnmountNonExistent(t *testing.T) {
	t.Parallel()
	d := &Driver{}
	err := d.Unmount(&volume.UnmountRequest{Name: "no-such-vol", ID: "test"})
	if err != nil {
		t.Fatalf("Unmount on non-existent volume: %v", err)
	}
}
