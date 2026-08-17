package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFileBackend_DeleteRepo(t *testing.T) {
	t.Parallel()
	repo := filepath.Join(t.TempDir(), "restic", "vol-a")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(repo, "snapshots")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}

	b := NewFileBackend()
	if err := b.DeleteRepo(context.Background(), repo); err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}
	if _, err := os.Stat(repo); !os.IsNotExist(err) {
		t.Fatalf("expected repo dir to be removed, stat err = %v", err)
	}
}

func TestFileBackend_DeleteRepoEmptyPath(t *testing.T) {
	t.Parallel()
	b := NewFileBackend()
	if err := b.DeleteRepo(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty repo path")
	}
}

func TestFileBackend_DeleteRepoMissingDirIsNoop(t *testing.T) {
	t.Parallel()
	b := NewFileBackend()
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := b.DeleteRepo(context.Background(), missing); err != nil {
		t.Fatalf("DeleteRepo on missing dir: %v", err)
	}
}

type mockPrefixDeleter struct{}

func (m *mockPrefixDeleter) DeleteObjectsWithPrefix(_ context.Context, _ string) error {
	return nil
}

func TestDeleteBackendForRepo(t *testing.T) {
	t.Parallel()
	deleter := &mockPrefixDeleter{}

	tests := []struct {
		name     string
		repo     string
		deleter  PrefixDeleter
		wantNil  bool
		wantType string
	}{
		{"local absolute", "/backups", deleter, false, "*backend.fileBackend"},
		{"local relative", "backups/repo", deleter, false, "*backend.fileBackend"},
		{"local explicit scheme", "local:/backups", deleter, false, "*backend.fileBackend"},
		{"local trailing slash", "/backups/", deleter, false, "*backend.fileBackend"},
		{"s3 with deleter", "s3:https://s3.example/bucket/repo", deleter, false, "*backend.s3Backend"},
		{"s3 without deleter", "s3:https://s3.example/bucket/repo", nil, true, ""},
		{"s3 endpoint no scheme", "s3:minio:9000/bucket/repo", deleter, false, "*backend.s3Backend"},
		{"rest server", "rest:http://localhost:8000/repo", deleter, true, ""},
		{"sftp", "sftp:user@host:/backups", deleter, true, ""},
		{"rclone", "rclone:remote:path", deleter, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeleteBackendForRepo(tt.repo, tt.deleter)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("DeleteBackendForRepo(%q) = %T, want nil", tt.repo, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("DeleteBackendForRepo(%q) = nil, want %s", tt.repo, tt.wantType)
			}
			if gotType := fmt.Sprintf("%T", got); gotType != tt.wantType {
				t.Errorf("DeleteBackendForRepo(%q) = %s, want %s", tt.repo, gotType, tt.wantType)
			}
		})
	}
}

func TestIsS3Repo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		repo string
		want bool
	}{
		{"s3:https://s3.example/bucket/repo", true},
		{"s3:bucket/repo", true},
		{"s3:minio:9000/bucket", true},
		{"s3:https://s3.example/bucket/repo/", true},
		{"/backups", false},
		{"local:/backups", false},
		{"rest:http://localhost:8000/repo", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsS3Repo(tt.repo); got != tt.want {
			t.Errorf("IsS3Repo(%q) = %v, want %v", tt.repo, got, tt.want)
		}
	}
}
