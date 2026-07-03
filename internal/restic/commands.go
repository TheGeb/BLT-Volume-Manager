package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic/run"
)

// --- Execution wrappers (delegate to run.Runner) ---

func (m *Manager) runSimple(ctx context.Context, args ...string) error {
	return m.runner.Run(ctx, args...)
}

func (m *Manager) captureOutput(ctx context.Context, args ...string) ([]byte, error) {
	return m.runner.Capture(ctx, args...)
}

func (m *Manager) combinedCapture(ctx context.Context, args ...string) ([]byte, error) {
	return m.runner.CombinedCapture(ctx, args...)
}

func (m *Manager) runCommand(cmd *exec.Cmd) error {
	return m.runner.RunCommand(cmd)
}

func (m *Manager) resticCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	return m.runner.Command(ctx, args...)
}

func (m *Manager) resticCommandForRepo(ctx context.Context, repo string, args ...string) (*exec.Cmd, error) {
	return m.runner.CommandForRepo(ctx, repo, args...)
}

func (m *Manager) runRestore(ctx context.Context, snapshotID, target string, tags ...string) error {
	return m.runner.Run(ctx, m.restoreCommand(snapshotID, target, tags...)...)
}

// --- Command builders ---

func (m *Manager) snapshotsCommand(opts *ListSnapshotsOpts) []string {
	args := []string{"snapshots", "--no-lock", "--json"}
	if opts != nil {
		for _, h := range opts.Hosts {
			args = append(args, "--host", h)
		}
		if opts.Latest > 0 {
			args = append(args, "--latest", strconv.Itoa(opts.Latest))
		}
		for _, t := range opts.Tags {
			args = append(args, "--tag", t)
		}
	}
	return args
}

func (m *Manager) snapshotsLastCommand(tag string) []string {
	if tag == BackupTagHot || tag == BackupTagCold {
		return []string{"snapshots", "--no-lock", "--tag", tag, "--last", "1", "--json"}
	}
	return []string{"snapshots", "--no-lock", "--last", "1", "--json"}
}

func (m *Manager) forgetCommand(ids ...string) []string {
	return append([]string{"forget"}, ids...)
}

func (m *Manager) tagAddCommand(snapshotID, tag string) []string {
	return []string{"tag", "--add", tag, snapshotID}
}

func (m *Manager) tagRemoveCommand(snapshotID, tag string) []string {
	return []string{"tag", "--remove", tag, snapshotID}
}

func (m *Manager) restoreCommand(snapshotID, target string, tags ...string) []string {
	args := []string{"restore", snapshotID, "--target", target}
	for _, t := range tags {
		args = append(args, "--tag", t)
	}
	return args
}

func (m *Manager) backupCommand(path string, tags []string) []string {
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

func (m *Manager) lsCommand(snapshotID, path string) []string {
	args := []string{"ls", "--no-lock", snapshotID}
	if path != "" && path != "/" {
		args = append(args, path)
	}
	return args
}

func (m *Manager) dumpCommand(snapshotID, path string) []string {
	return []string{"dump", "--no-lock", snapshotID, path}
}

func (m *Manager) diffCommand(snapID1, snapID2 string) []string {
	return []string{"diff", "--no-lock", "--json", snapID1, snapID2}
}

func (m *Manager) statsCommand(mode string) []string {
	return []string{"stats", "--no-lock", "--json", "--mode", mode}
}

func (m *Manager) statsSnapshotCommand(snapshotID string) []string {
	return []string{"stats", "--no-lock", snapshotID, "--json"}
}

func (m *Manager) checkCommand(noLock bool) []string {
	args := []string{"check"}
	if noLock {
		args = append(args, "--no-lock")
	}
	return args
}

func (m *Manager) repairCommand() []string {
	return []string{"repair", "index"}
}

func (m *Manager) initCommand() []string {
	return []string{"init"}
}

func (m *Manager) copyCommand(destRepo string, snapshotIDs ...string) []string {
	args := []string{fmt.Sprintf("--from-repo=%s", m.repo), "copy"}
	args = append(args, snapshotIDs...)
	if v := log.Verbosity(); v > 0 {
		args = append(args, fmt.Sprintf("--verbose=%d", v))
	}
	return args
}

func (m *Manager) unlockCommand() []string {
	return []string{"unlock"}
}

func (m *Manager) hostSnapshotsCommand() []string {
	return []string{"snapshots", "--no-lock", "--json", "--group-by", "host", "--latest", "1"}
}

func (m *Manager) repoExistsCommand() []string {
	return []string{"snapshots", "--no-lock", "--json"}
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

func isRepositoryMissing(output string) bool {
	return run.IsRepositoryMissing(output)
}
