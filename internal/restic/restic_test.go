package restic

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic/cli"
)

// exitStatusError returns an *exec.ExitError with the given exit code,
// matching the error restic returns when it fails.
func exitStatusError(code int) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// mockRunner implements runner for testing without real restic binary.
type mockRunner struct {
	initFn       func(ctx context.Context) ([]byte, error)
	repoExistsFn func(ctx context.Context) ([]byte, error)
}

func (m *mockRunner) InitOutput(ctx context.Context) ([]byte, error) {
	if m.initFn != nil {
		return m.initFn(ctx)
	}
	return nil, nil
}

func (m *mockRunner) RepoExists(ctx context.Context) ([]byte, error) {
	if m.repoExistsFn != nil {
		return m.repoExistsFn(ctx)
	}
	return nil, nil
}

func (m *mockRunner) Snapshots(ctx context.Context, opts *cli.ListSnapshotsOpts) ([]byte, error) {
	return nil, nil
}
func (m *mockRunner) Forget(ctx context.Context, ids ...string) error        { return nil }
func (m *mockRunner) Stats(ctx context.Context, mode string) ([]byte, error) { return nil, nil }
func (m *mockRunner) StatsSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	return nil, nil
}
func (m *mockRunner) Check(ctx context.Context, noLock bool) error { return nil }
func (m *mockRunner) Repair(ctx context.Context) error             { return nil }
func (m *mockRunner) Copy(ctx context.Context, destRepo string, snapshotIDs ...string) error {
	return nil
}
func (m *mockRunner) Unlock(ctx context.Context) error                             { return nil }
func (m *mockRunner) HostSnapshots(ctx context.Context) ([]byte, error)            { return nil, nil }
func (m *mockRunner) Backup(ctx context.Context, path string, tags []string) error { return nil }
func (m *mockRunner) BackupInDir(ctx context.Context, path string, tags []string, workDir string) error {
	return nil
}

func (m *mockRunner) BackupAt(ctx context.Context, path string, tags []string, t time.Time) error {
	return nil
}

func (m *mockRunner) Ls(ctx context.Context, snapshotID, path string) ([]byte, error) {
	return nil, nil
}

func (m *mockRunner) Dump(ctx context.Context, snapshotID, path string) ([]byte, error) {
	return nil, nil
}

func (m *mockRunner) Diff(ctx context.Context, snapID1, snapID2 string) ([]byte, error) {
	return nil, nil
}
func (m *mockRunner) TagAdd(ctx context.Context, snapshotID, tag string) error    { return nil }
func (m *mockRunner) TagRemove(ctx context.Context, snapshotID, tag string) error { return nil }
func (m *mockRunner) Restore(ctx context.Context, snapshotID, target string, tags ...string) error {
	return nil
}

func TestRepoExists_NotFound(t *testing.T) {
	t.Parallel()
	m := NewManager("/test/repo")
	m.runner = &mockRunner{
		repoExistsFn: func(ctx context.Context) ([]byte, error) {
			return nil, exitStatusError(exitCodeRepoDoesNotExist)
		},
	}
	exists, err := m.RepoExists(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for not-found, got: %v", err)
	}
	if exists {
		t.Error("expected exists=false for unknown repository")
	}
}

func TestRepoExists_ExecError(t *testing.T) {
	t.Parallel()
	m := NewManager("/test/repo")
	m.runner = &mockRunner{
		repoExistsFn: func(ctx context.Context) ([]byte, error) {
			return nil, &exec.Error{Name: "restic", Err: errors.New("not found")}
		},
	}
	_, err := m.RepoExists(context.Background())
	if err == nil {
		t.Fatal("expected error for exec failure")
	}
}

func TestRepoExists_OtherFailure(t *testing.T) {
	t.Parallel()
	m := NewManager("/test/repo")
	m.runner = &mockRunner{
		repoExistsFn: func(ctx context.Context) ([]byte, error) {
			return nil, exitStatusError(1)
		},
	}
	_, err := m.RepoExists(context.Background())
	if err == nil {
		t.Fatal("expected error for non-zero exit code other than 10, got nil")
	}
}

func TestInitRepo_AlreadyInitialized(t *testing.T) {
	t.Parallel()
	m := NewManager("/test/repo")
	m.runner = &mockRunner{
		initFn: func(ctx context.Context) ([]byte, error) {
			return []byte("Fatal: create repository at /test/repo failed: config file already exists\n"), errors.New("exit status 1")
		},
	}
	err := m.Init(context.Background())
	if err != nil {
		t.Fatalf("expected nil for already initialized, got: %v", err)
	}
}

func TestInitRepo_RealError(t *testing.T) {
	t.Parallel()
	m := NewManager("/test/repo")
	m.runner = &mockRunner{
		initFn: func(ctx context.Context) ([]byte, error) {
			return []byte("Fatal: create repository at /test/repo failed: PermissionDenied\n"), errors.New("exit status 1")
		},
	}
	err := m.Init(context.Background())
	if err == nil {
		t.Fatal("expected error for init failure, got nil")
	}
}

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
