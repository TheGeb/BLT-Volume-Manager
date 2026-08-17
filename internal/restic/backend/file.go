package backend

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// fileBackend deletes a restic repository that lives in a local directory.
type fileBackend struct{}

// NewFileBackend returns a Backend that deletes local restic repositories
// by removing the repository directory.
func NewFileBackend() Backend {
	return &fileBackend{}
}

func (b *fileBackend) DeleteRepo(ctx context.Context, repoPath string) error {
	if repoPath == "" {
		return fmt.Errorf("refusing to delete an empty local repo path")
	}
	if err := os.RemoveAll(repoPath); err != nil {
		return fmt.Errorf("remove local repo %q: %w", repoPath, err)
	}
	return nil
}

// DeleteBackendForRepo returns the Backend used to delete a restic
// repository stored at repo. S3 repositories delete objects through
// s3Deleter (which may be nil only when repo is not an s3 repository, in
// which case nil is returned and repo deletion is unsupported). Local file
// repositories (bare paths or "local:" URLs) use a file-based backend.
// Remote backends without a native delete mechanism (rest:, sftp:, rclone:,
// ...) also yield nil.
func DeleteBackendForRepo(repo string, s3Deleter PrefixDeleter) Backend {
	repo = strings.TrimSuffix(repo, "/")
	switch {
	case strings.HasPrefix(repo, "s3:"):
		if s3Deleter == nil {
			return nil
		}
		return NewS3Backend(s3Deleter)
	case strings.HasPrefix(repo, "local:"):
		return NewFileBackend()
	}
	if repoScheme(repo) != "" {
		return nil
	}
	return NewFileBackend()
}

// IsS3Repo reports whether repo points at an S3 repository.
func IsS3Repo(repo string) bool {
	return strings.HasPrefix(strings.TrimSuffix(repo, "/"), "s3:")
}

// repoScheme returns the scheme prefix of a restic repository URL (the part
// before the first colon), or "" when repo is a local path.
func repoScheme(repo string) string {
	repo = strings.TrimSuffix(repo, "/")
	i := strings.IndexByte(repo, ':')
	if i <= 0 {
		return ""
	}
	return repo[:i]
}
