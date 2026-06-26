package restic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
)

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

func (m *Manager) resticCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	global := []string{"-r", m.repo}
	if v := app.Verbosity(); v > 0 {
		global = append(global, fmt.Sprintf("--verbose=%d", v))
	}

	global = append(global, args...)
	app.Debugf("restic_command", "args=%s", strings.Join(global, " "))
	//nolint:gosec // args constructed from env vars and hardcoded strings only
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
