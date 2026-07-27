package backend

import "context"

type Backend interface {
	DeleteRepo(ctx context.Context, repoPath string) error
}
