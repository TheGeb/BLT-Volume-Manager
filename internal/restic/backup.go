package restic

import (
	"context"
	"time"
)

func (m *Manager) Backup(ctx context.Context, path string, tags ...string) error {
	return m.BackupInDir(ctx, path, tags, "")
}

func (m *Manager) BackupInDir(ctx context.Context, path string, tags []string, workDir string) error {
	m.ensureRepo(ctx)
	if workDir != "" {
		return m.runner.BackupInDir(ctx, path, tags, workDir)
	}
	return m.runner.Backup(ctx, path, tags)
}

func (m *Manager) BackupAt(ctx context.Context, path string, tags []string, t time.Time) error {
	m.ensureRepo(ctx)
	return m.runner.BackupAt(ctx, path, tags, t)
}

func (m *Manager) ensureRepo(ctx context.Context) {
	// Best-effort init: if it fails, the repo likely already exists.
	// The backup call will surface any real errors.
	_ = m.Init(ctx)
}
