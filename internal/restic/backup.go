package restic

import (
	"context"
	"time"
)

func (m *Manager) Backup(path string, tags ...string) error {
	return m.BackupInDir(path, tags, "")
}

func (m *Manager) BackupInDir(path string, tags []string, workDir string) error {
	args := m.backupCommand(path, tags)
	if workDir != "" {
		cmd, err := m.resticCommand(context.Background(), args...)
		if err != nil {
			return err
		}
		cmd.Dir = workDir
		return m.runCommand(cmd)
	}
	return m.runSimple(context.Background(), args...)
}

func (m *Manager) BackupAt(path string, tags []string, t time.Time) error {
	args := m.backupCommand(path, tags)
	if !t.IsZero() {
		args = append(args, "--time", t.Format(time.RFC3339))
	}
	return m.runSimple(context.Background(), args...)
}
