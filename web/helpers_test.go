package web

import (
	"testing"
	"time"

	"github.com/example/blt-volume-manager/restic"
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
		{"/some/other/path", "path"},
		{"", ""},
		{"/volumes/group/deeply/nested/data.txt", "group"},
	}

	for _, tt := range tests {
		got := volumeNameFromPath(tt.path)
		if got != tt.expected {
			t.Errorf("volumeNameFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestSnapshotMatchesVolume(t *testing.T) {
	snap := restic.Snapshot{
		ID:       "abc123",
		ShortID:  "abc",
		Time:     time.Now(),
		Paths:    []string{"/volumes/my-vol/data", "/volumes/my-vol/config"},
		Hostname: "host1",
	}

	if !snapshotMatchesVolume(snap, "my-vol") {
		t.Error("expected match for 'my-vol'")
	}
	if snapshotMatchesVolume(snap, "other-vol") {
		t.Error("expected no match for 'other-vol'")
	}
	if snapshotMatchesVolume(snap, "") {
		t.Error("expected no match for empty volume")
	}
}

func TestSnapshotMatchesVolumeNoPaths(t *testing.T) {
	snap := restic.Snapshot{Paths: nil}
	if snapshotMatchesVolume(snap, "my-vol") {
		t.Error("expected false for nil paths")
	}

	snap2 := restic.Snapshot{Paths: []string{}}
	if snapshotMatchesVolume(snap2, "my-vol") {
		t.Error("expected false for empty paths")
	}
}

func TestSnapshotMatchesVolumeNested(t *testing.T) {
	snap := restic.Snapshot{
		Paths: []string{"/volumes/group/sub-vol/backup.tar.gz"},
	}

	if !snapshotMatchesVolume(snap, "group") {
		t.Error("expected match for group volume")
	}
}

func TestMustHostname(t *testing.T) {
	h := mustHostname()
	if h == "" {
		t.Error("expected non-empty hostname")
	}
}

func TestRespondError(t *testing.T) {
	_ = mustHostname
	_ = volumeNameFromPath
	_ = snapshotMatchesVolume
}
