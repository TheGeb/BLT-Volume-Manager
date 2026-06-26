package restic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
)

type RepoStats struct {
	TotalSize             int64 `json:"total_size"`
	TotalFileCount        int64 `json:"total_file_count"`
	TotalBlobCount        int64 `json:"total_blob_count"`
	TotalUncompressedSize int64 `json:"total_uncompressed_size"`
	UniqueBlobCount       int64 `json:"unique_blob_count"`
	UniqueBlobSize        int64 `json:"unique_blob_size"`
}

type SnapshotSizeResult struct {
	TotalSize      int64 `json:"total_size"`
	TotalFileCount int64 `json:"total_file_count"`
}

func (m *Manager) RepoExists() (bool, error) {
	return m.repositoryExists()
}

func (m *Manager) Init() error {
	return m.initRepository()
}

func (m *Manager) Stats() (*RepoStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "stats", "--no-lock", "--json", "--mode", "raw-data")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		log.Errorf("restic_stats_failed", err, "stderr=%s", string(out))
		return nil, fmt.Errorf("restic stats: %w", err)
	}

	var stats RepoStats
	if err := json.Unmarshal(out, &stats); err != nil {
		log.Errorf("parse_restic_stats_failed", err, "raw=%s", string(out))
		return nil, fmt.Errorf("parse restic stats: %w", err)
	}
	return &stats, nil
}

func (m *Manager) SnapshotStats(snapshotID string) (*SnapshotSizeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "stats", "--no-lock", snapshotID, "--json")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("restic snapshot stats: %w", err)
	}

	var result SnapshotSizeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse restic snapshot stats: %w", err)
	}
	return &result, nil
}

func (m *Manager) repositoryExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "snapshots", "--no-lock", "--json")
	if err != nil {
		return false, err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isRepositoryMissing(string(out)) {
			return false, nil
		}
		return false, fmt.Errorf("restic snapshots failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func (m *Manager) initRepository() error {
	return m.runSimple(context.Background(), "init")
}

func (m *Manager) Check(noLock bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), ResticTimeoutMedium)
	defer cancel()
	args := []string{"check"}
	if noLock {
		args = append(args, "--no-lock")
	}
	return m.runSimple(ctx, args...)
}

func (m *Manager) Repair() error {
	if err := m.Unlock(); err != nil {
		log.Warnf("repair_unlock_failed_continuing", "error=%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), ResticTimeoutLong)
	defer cancel()
	return m.runSimple(ctx, "repair", "index")
}

func (m *Manager) CopyTo(destRepo string, snapshotIDs ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), ResticTimeoutLong)
	defer cancel()

	args := []string{"--from-repo", m.repo, "-r", destRepo, "copy"}
	args = append(args, snapshotIDs...)

	if pwFile := os.Getenv("RESTIC_PASSWORD_FILE"); pwFile != "" {
		args = append(args, "--from-password-file", pwFile, "--password-file", pwFile)
	} else if pw := os.Getenv("RESTIC_PASSWORD"); pw != "" {
		tmpFile, err := os.CreateTemp("", "restic-pw-*")
		if err != nil {
			return fmt.Errorf("create temp password file: %w", err)
		}
		tmpName := tmpFile.Name()
		defer func() { _ = os.Remove(tmpName) }()
		if _, err := tmpFile.WriteString(pw + "\n"); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("write temp password file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("close temp password file: %w", err)
		}
		args = append(args, "--from-password-file", tmpName, "--password-file", tmpName)
	}

	if v := log.Verbosity(); v > 0 {
		args = append(args, fmt.Sprintf("--verbose=%d", v))
	}

	log.Debugf("restic_command", "args=%s", strings.Join(args, " "))
	//nolint:gosec // args constructed from env vars and hardcoded strings only
	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = os.Environ()
	return m.runCommand(cmd)
}

func (m *Manager) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.runSimple(ctx, "unlock")
}
