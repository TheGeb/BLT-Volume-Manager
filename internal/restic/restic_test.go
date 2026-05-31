package restic

import (
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
	"github.com/TheGeb/BLT-Volume-Manager/internal/volumepath"
)

func TestHasTag(t *testing.T) {
	tags := []string{constants.BackupTagHot, constants.BackupTagCold, constants.BackupTagRestore}

	if !hasTag(tags, constants.BackupTagHot) {
		t.Error("expected 'hot' found")
	}
	if !hasTag(tags, constants.BackupTagRestore) {
		t.Error("expected 'restore-point' found")
	}
	if hasTag(tags, "nonexistent") {
		t.Error("expected 'nonexistent' not found")
	}
	if hasTag(nil, "hot") {
		t.Error("expected false for nil tags")
	}
	if hasTag([]string{}, "hot") {
		t.Error("expected false for empty tags")
	}
	if hasTag([]string{constants.BackupTagHot, constants.BackupTagCold}, "") {
		t.Error("expected false for empty target")
	}
}

func TestCommonPathPrefix(t *testing.T) {
	tests := []struct {
		paths    []string
		expected string
	}{
		{[]string{"/a/b/c", "/a/b/d", "/a/b/e"}, "/a/b"},
		{[]string{"/a/b/c", "/a/c/d", "/a/e/f"}, "/a"},
		{[]string{"/a/b/c"}, "/a/b/c"},
		{[]string{"/a/b/c", "/a/b/c/d", "/a/b/c/e"}, "/a/b/c"},
		{[]string{"/a/b/c", "/d/e/f"}, ""},
		{[]string{}, ""},
		{[]string{"/", "/a"}, "/"},
		{[]string{"/a", "/a/b", "/a/b/c"}, "/a"},
	}

	for _, tt := range tests {
		got := commonPathPrefix(tt.paths)
		if got != tt.expected {
			t.Errorf("commonPathPrefix(%v) = %q, want %q", tt.paths, got, tt.expected)
		}
	}
}

func TestVolumeNameFromPath(t *testing.T) {
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
		got := volumepath.VolumeNameFromPath(tt.path)
		if got != tt.expected {
			t.Errorf("VolumeNameFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
		}
	}
}

func TestIsRepositoryMissing(t *testing.T) {
	tests := []struct {
		output   string
		expected bool
	}{
		{"repository not found", true},
		{"Fatal: repository does not exist", true},
		{"is not initialized yet", true},
		{"snapshot count: 5", false},
		{"repository abc123 opened successfully", false},
		{"", false},
		{"NOT FOUND", true},
		{"Does Not Exist", true},
		{"Not Initialized", true},
	}

	for _, tt := range tests {
		got := isRepositoryMissing(tt.output)
		if got != tt.expected {
			t.Errorf("isRepositoryMissing(%q) = %v, want %v", tt.output, got, tt.expected)
		}
	}
}

func TestGenerateHash(t *testing.T) {
	m := NewManager("/tmp/repo")

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := Snapshot{
		ID:       "abc123def456",
		ShortID:  "abc123",
		Time:     now,
		Tree:     "treehash",
		Tags:     []string{constants.BackupTagCold},
		Paths:    []string{"/volumes/vol1", "/volumes/vol2"},
		Hostname: "host1",
	}

	hash := m.GenerateHash(snap)
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex, got %d: %s", len(hash), hash)
	}

	hash2 := m.GenerateHash(snap)
	if hash != hash2 {
		t.Error("expected deterministic hash for same input")
	}

	snap2 := snap
	snap2.Paths = []string{"/volumes/vol2", "/volumes/vol1"}
	hash3 := m.GenerateHash(snap2)
	if hash != hash3 {
		t.Error("expected same hash regardless of path order")
	}
}

func TestGenerateHashDiffers(t *testing.T) {
	m := NewManager("/tmp/repo")
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	snap1 := Snapshot{Hostname: "host-a", Time: now, Tree: "t1", Paths: []string{"/volumes/v1"}}
	snap2 := Snapshot{Hostname: "host-b", Time: now, Tree: "t1", Paths: []string{"/volumes/v1"}}

	h1 := m.GenerateHash(snap1)
	h2 := m.GenerateHash(snap2)
	if h1 == h2 {
		t.Error("expected different hashes for different hostnames")
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager("s3:https://bucket.s3.amazonaws.com/repo")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.Repo() != "s3:https://bucket.s3.amazonaws.com/repo" {
		t.Errorf("unexpected repo: %q", m.Repo())
	}
}

func TestNewManagerLocalPath(t *testing.T) {
	m := NewManager("/data/restic/vol1")
	if m.Repo() != "/data/restic/vol1" {
		t.Errorf("expected /data/restic/vol1, got %q", m.Repo())
	}
}

func TestFindSnapshotByHashEmpty(t *testing.T) {
	m := NewManager("/nonexistent/repo")
	_, err := m.FindSnapshotByHash("somehash")
	if err == nil {
		t.Error("expected error for nonexistent repo")
	}
}

func TestSnapshotSortLogic(t *testing.T) {
	now := time.Now()
	snaps := []Snapshot{
		{ShortID: "s1", Time: now.Add(-2 * time.Hour), Tags: []string{constants.BackupTagRestore}, Paths: []string{"/volumes/my-vol/data"}},
		{ShortID: "s2", Time: now.Add(-1 * time.Hour), Tags: []string{constants.BackupTagRestore}, Paths: []string{"/volumes/my-vol/data"}},
		{ShortID: "s3", Time: now, Tags: []string{constants.BackupTagHot}, Paths: []string{"/volumes/my-vol/data"}},
	}

	if !hasTag(snaps[0].Tags, constants.BackupTagRestore) {
		t.Error("expected s1 to have restore-point tag")
	}
	if hasTag(snaps[2].Tags, constants.BackupTagRestore) {
		t.Error("expected s3 to not have restore-point tag")
	}
	if !snaps[2].Time.After(snaps[1].Time) {
		t.Error("expected s3 to be newest")
	}
}

func TestCommonPathPrefixSingle(t *testing.T) {
	got := commonPathPrefix([]string{"/a/b/c"})
	if got != "/a/b/c" {
		t.Errorf("expected '/a/b/c', got %q", got)
	}
}

func TestCommonPathPrefixIdentical(t *testing.T) {
	got := commonPathPrefix([]string{"/a/b/c", "/a/b/c", "/a/b/c"})
	if got != "/a/b/c" {
		t.Errorf("expected '/a/b/c', got %q", got)
	}
}

func TestPathBelongsToVolume(t *testing.T) {
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
		got := volumepath.PathBelongsToVolume(tt.path, tt.volume)
		if got != tt.want {
			t.Errorf("PathBelongsToVolume(%q, %q) = %v, want %v", tt.path, tt.volume, got, tt.want)
		}
	}
}

func TestPathBelongsToVolumeColdSnapEdgeCases(t *testing.T) {
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
		got := volumepath.PathBelongsToVolume(tt.path, tt.volume)
		if got != tt.want {
			t.Errorf("PathBelongsToVolume(%q, %q) = %v, want %v", tt.path, tt.volume, got, tt.want)
		}
	}
}
