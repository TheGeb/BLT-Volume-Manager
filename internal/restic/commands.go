package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

// TODO: Do not have other files use raw strings to specify commands, create a method for each
func (m *Manager) runSimple(ctx context.Context, args ...string) error {
	cmd, err := m.resticCommand(ctx, args...)
	if err != nil {
		return err
	}
	return m.runCommand(cmd)
}

func (m *Manager) runCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) runRestore(ctx context.Context, snapshotID, target string, tags ...string) error {
	args := []string{"restore", snapshotID, "--target", target}
	for _, t := range tags {
		args = append(args, "--tag", t)
	}
	cmd, err := m.resticCommand(ctx, args...)
	if err != nil {
		return err
	}
	return m.runCommand(cmd)
}

func (m *Manager) resticCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	return m.resticCommandForRepo(ctx, m.repo, args...)
}

func (m *Manager) resticCommandForRepo(ctx context.Context, repo string, args ...string) (*exec.Cmd, error) {
	global := []string{fmt.Sprintf("--repo=%s", repo)}
	if v := log.Verbosity(); v > 0 {
		global = append(global, fmt.Sprintf("--verbose=%d", v))
	}

	global = append(global, args...)
	log.Debugf("restic_command", "args=%s", strings.Join(global, " "))
	//nolint:gosec // all variable args are --flag=value pairs, repo is passed as --repo=<value>
	cmd := exec.CommandContext(ctx, "restic", global...)
	cmd.Env = os.Environ()
	return cmd, nil
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
	out := strings.ToLower(output)
	return strings.Contains(out, "not found") || strings.Contains(out, "does not exist") || strings.Contains(out, "not initialized")
}
