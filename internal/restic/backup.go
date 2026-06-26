package restic

import (
	"context"
	"os"
	"time"
)

func (m *Manager) Backup(path string, tags ...string) error {
	return m.BackupInDir(path, tags, "")
}

func (m *Manager) BackupInDir(path string, tags []string, workDir string) error {
	args := m.backupArgs(path, tags)
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
	args := m.backupArgs(path, tags)
	if !t.IsZero() {
		args = append(args, "--time", t.Format(time.RFC3339))
	}
	return m.runSimple(context.Background(), args...)
}

func (m *Manager) backupArgs(path string, tags []string) []string {
	args := []string{"backup", path}
	for _, tag := range tags {
		if tag != "" {
			args = append(args, "--tag", tag)
		}
	}
	compression := os.Getenv("RESTIC_COMPRESSION")
	if compression == "" {
		compression = "auto"
	}
	args = append(args, "--compression", compression)
	return args
}
