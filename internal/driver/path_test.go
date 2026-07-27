package driver

import (
	"testing"
)

func TestVolumeNameFromPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		expected string
	}{
		{"/var/lib/docker-volumes/volumes/my-volume/data/file.txt", "my-volume"},
		{"/volumes/my-volume/data", "my-volume"},
		{"/volumes/group/sub/data", "group"},
		{"/some/path/vol1-cold-snapshot", "vol1"},
		{"/some/path/vol1-pre-restore", "vol1"},
		{"/some/path", ""},
		{"", ""},
		{"/volumes/", ""},
	}

	for _, tt := range tests {
		got := VolumeNameFromPath(tt.path)
		if got != tt.expected {
			t.Errorf("VolumeNameFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestPathBelongsToVolume(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path   string
		volume string
		want   bool
	}{
		{"/var/lib/docker-volumes/volumes/my-vol/data/file.txt", "my-vol", true},
		{"/volumes/my-vol/data", "my-vol", true},
		{"/volumes/group/sub-vol/data", "group", true},
		{"/volumes/group/sub-vol/data", "group/sub-vol", true},
		{"/volumes/", "my-vol", false},
		{"/some/path/vol1-cold-snapshot", "vol1", true},
		{"/some/path/vol1-pre-restore", "vol1", true},
		{"/snaps/group/sub-vol-cold-snapshot", "group/sub-vol", true},
		{"/some/path", "vol1", false},
		{"", "vol1", false},
		{"/snaps/my-vol-cold-snap", "vol", false},
		{"/volumes/other-vol", "my-vol", false},
	}

	for _, tt := range tests {
		got := PathBelongsToVolume(tt.path, tt.volume)
		if got != tt.want {
			t.Errorf("PathBelongsToVolume(%q, %q) = %v, want %v", tt.path, tt.volume, got, tt.want)
		}
	}
}

func TestPathBelongsToVolumeColdSnapEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path   string
		volume string
		want   bool
	}{
		{"/snaps/my-vol-cold-snapshot", "my-vol", true},
		{"/snaps/group/sub-vol-cold-snapshot", "group/sub-vol", true},
		{"/snaps/vol-cold-snapshot", "vol", true},
		{"/snaps/other-vol-cold-snapshot", "vol", false},
	}
	for _, tt := range tests {
		got := PathBelongsToVolume(tt.path, tt.volume)
		if got != tt.want {
			t.Errorf("PathBelongsToVolume(%q, %q) = %v, want %v", tt.path, tt.volume, got, tt.want)
		}
	}
}
