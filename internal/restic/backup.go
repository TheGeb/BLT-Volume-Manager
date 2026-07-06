package restic

import (
	"context"
	"time"
)

func (m *Manager) Backup(path string, tags ...string) error {
	return m.BackupInDir(path, tags, "")
}

func (m *Manager) BackupInDir(path string, tags []string, workDir string) error {
	m.ensureRepo()
	if workDir != "" {
		return m.runner.BackupInDir(context.Background(), path, tags, workDir)
	}
	return m.runner.Backup(context.Background(), path, tags)
}

func (m *Manager) BackupAt(path string, tags []string, t time.Time) error {
	m.ensureRepo()
	return m.runner.BackupAt(context.Background(), path, tags, t)
}

func (m *Manager) ensureRepo() {
	// Best-effort init: if it fails, the repo likely already exists.
	// The backup call will surface any real errors.
	_ = m.Init()
}
