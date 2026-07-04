package restic

import (
	"context"
	"time"
)

func (m *Manager) Backup(path string, tags ...string) error {
	return m.BackupInDir(path, tags, "")
}

func (m *Manager) BackupInDir(path string, tags []string, workDir string) error {
	if workDir != "" {
		return m.runner.BackupInDir(context.Background(), path, tags, workDir)
	}
	return m.runner.Backup(context.Background(), path, tags)
}

func (m *Manager) BackupAt(path string, tags []string, t time.Time) error {
	return m.runner.BackupAt(context.Background(), path, tags, t)
}
