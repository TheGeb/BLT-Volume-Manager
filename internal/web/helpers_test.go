package web

import (
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/volumepath"
)

func TestVolumeNameFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/var/lib/docker-volumes/volumes/my-volume/data/file.txt", "my-volume"},
		{"/volumes/my-volume/data", "my-volume"},
		{"/volumes/group/sub/data", "group"},
		{"/volumes/", ""},
		{"/some/other/path", ""},
		{"", ""},
		{"/volumes/group/deeply/nested/data.txt", "group"},
	}

	for _, tt := range tests {
		got := volumepath.VolumeNameFromPath(tt.path)
		if got != tt.expected {
			t.Errorf("VolumeNameFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestSnapshotMatchesVolume(t *testing.T) {
	paths := []string{"/volumes/my-vol/data", "/volumes/my-vol/config"}

	for _, path := range paths {
		if !volumepath.PathBelongsToVolume(path, "my-vol") {
			t.Errorf("expected match for 'my-vol' in path %q", path)
		}
	}
	if volumepath.PathBelongsToVolume("/volumes/other-vol/data", "my-vol") {
		t.Error("expected no match for 'other-vol'")
	}
	if volumepath.PathBelongsToVolume("", "my-vol") {
		t.Error("expected no match for empty volume")
	}
}

func TestSnapshotMatchesVolumeNoPaths(t *testing.T) {
	if volumepath.PathBelongsToVolume("", "my-vol") {
		t.Error("expected false for empty path")
	}
}

func TestSnapshotMatchesVolumeNested(t *testing.T) {
	if !volumepath.PathBelongsToVolume("/volumes/group/sub-vol/backup.tar.gz", "group") {
		t.Error("expected match for group volume")
	}
	if !volumepath.PathBelongsToVolume("/volumes/group/sub-vol/backup.tar.gz", "group/sub-vol") {
		t.Error("expected match for nested volume group/sub-vol")
	}
	if volumepath.PathBelongsToVolume("/volumes/group/sub-vol/backup.tar.gz", "other") {
		t.Error("expected no match for other volume")
	}
}

func TestHostname(t *testing.T) {
	h := store.Hostname()
	if h == "" {
		t.Error("expected non-empty hostname")
	}
}
