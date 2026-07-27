package backend

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type PrefixDeleter interface {
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}

func NewS3Backend(d PrefixDeleter) Backend {
	return &s3Backend{deleter: d}
}

type s3Backend struct {
	deleter PrefixDeleter
}

func (b *s3Backend) DeleteRepo(ctx context.Context, repoPath string) error {
	prefix := s3RepoPrefix(repoPath)
	if prefix == "" {
		return fmt.Errorf("cannot derive S3 prefix from repo path %q", repoPath)
	}
	return b.deleter.DeleteObjectsWithPrefix(ctx, prefix)
}

func s3RepoPrefix(repoPath string) string {
	path := strings.TrimPrefix(repoPath, "s3:")
	u, err := url.Parse(path)
	if err != nil {
		return ""
	}
	cleaned := strings.TrimPrefix(u.Path, "/")
	parts := strings.SplitN(cleaned, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1] + "/"
}
