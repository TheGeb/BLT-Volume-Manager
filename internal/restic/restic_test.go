package restic

import (
	"context"
	"testing"
	"time"
)

func TestCommonPathPrefix(t *testing.T) {
	t.Parallel()
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

func TestGenerateHash(t *testing.T) {
	t.Parallel()
	m := NewManager("/tmp/repo")

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := Snapshot{
		ID:       "abc123def456",
		ShortID:  "abc123",
		Time:     now,
		Tree:     "treehash",
		Tags:     []string{BackupTagCold},
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
	t.Parallel()
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
	t.Parallel()
	m := NewManager("s3:https://bucket.s3.amazonaws.com/repo")
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.Repo() != "s3:https://bucket.s3.amazonaws.com/repo" {
		t.Errorf("unexpected repo: %q", m.Repo())
	}
}

func TestNewManagerLocalPath(t *testing.T) {
	t.Parallel()
	m := NewManager("/data/restic/vol1")
	if m.Repo() != "/data/restic/vol1" {
		t.Errorf("expected /data/restic/vol1, got %q", m.Repo())
	}
}

func TestFindSnapshotByHashEmpty(t *testing.T) {
	t.Parallel()
	m := NewManager("/nonexistent/repo")
	_, err := m.FindSnapshotByHash(context.Background(), "somehash")
	if err == nil {
		t.Error("expected error for nonexistent repo")
	}
}

func TestSnapshotSortLogic(t *testing.T) {
	t.Parallel()
	now := time.Now()
	snaps := []Snapshot{
		{ShortID: "s1", Time: now.Add(-2 * time.Hour), Tags: []string{"restore-point"}, Paths: []string{"/volumes/my-vol/data"}},
		{ShortID: "s2", Time: now.Add(-1 * time.Hour), Tags: []string{"restore-point"}, Paths: []string{"/volumes/my-vol/data"}},
		{ShortID: "s3", Time: now, Tags: []string{BackupTagHot}, Paths: []string{"/volumes/my-vol/data"}},
	}

	tagFound := func(tags []string, target string) bool {
		for _, t := range tags {
			if t == target {
				return true
			}
		}
		return false
	}
	if !tagFound(snaps[0].Tags, "restore-point") {
		t.Error("expected s1 to have restore-point tag")
	}
	if tagFound(snaps[2].Tags, "restore-point") {
		t.Error("expected s3 to not have restore-point tag")
	}
	if !snaps[2].Time.After(snaps[1].Time) {
		t.Error("expected s3 to be newest")
	}
}

func TestCommonPathPrefixSingle(t *testing.T) {
	t.Parallel()
	got := commonPathPrefix([]string{"/a/b/c"})
	if got != "/a/b/c" {
		t.Errorf("expected '/a/b/c', got %q", got)
	}
}

func TestCommonPathPrefixIdentical(t *testing.T) {
	t.Parallel()
	got := commonPathPrefix([]string{"/a/b/c", "/a/b/c", "/a/b/c"})
	if got != "/a/b/c" {
		t.Errorf("expected '/a/b/c', got %q", got)
	}
}
