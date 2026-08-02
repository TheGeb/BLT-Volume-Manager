package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

type Runner struct {
	Repo string
}

func (r *Runner) command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	return r.commandForRepo(ctx, r.Repo, args...)
}

func (r *Runner) commandForRepo(ctx context.Context, repo string, args ...string) (*exec.Cmd, error) {
	global := []string{fmt.Sprintf("--repo=%s", repo)}
	if v := log.Verbosity(); v > 0 {
		global = append(global, fmt.Sprintf("--verbose=%d", v))
	}

	global = append(global, args...)
	log.Debugf("restic_command", "args=%s", strings.Join(global, " "))
	// gosec G204: all args are --flag=value pairs passed via exec.CommandContext
	// (no shell invocation). repo is passed as --repo=<value>. Each arg is treated
	// atomically by the restic binary.
	//nolint:gosec
	cmd := exec.CommandContext(ctx, "restic", global...)
	cmd.Env = os.Environ()
	if _, ok := os.LookupEnv("RESTIC_FROM_PASSWORD"); !ok {
		// restic copy reads the source repo password from RESTIC_FROM_PASSWORD.
		// For copies between the manager's own repos the passwords are the
		// same, so default it to RESTIC_PASSWORD when the user did not set it.
		if pw := os.Getenv("RESTIC_PASSWORD"); pw != "" {
			cmd.Env = append(cmd.Env, "RESTIC_FROM_PASSWORD="+pw)
		}
	}
	return cmd, nil
}

func (r *Runner) run(ctx context.Context, args ...string) error {
	cmd, err := r.command(ctx, args...)
	if err != nil {
		return err
	}
	return r.runCommand(cmd)
}

func (r *Runner) capture(ctx context.Context, args ...string) ([]byte, error) {
	cmd, err := r.command(ctx, args...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

func (r *Runner) combinedCapture(ctx context.Context, args ...string) ([]byte, error) {
	cmd, err := r.command(ctx, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

func (r *Runner) runCommand(cmd *exec.Cmd) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("restic_failed", err, "output=%s", strings.TrimSpace(string(output)))
	} else if len(output) > 0 {
		log.Debugf("restic_output", "output=%s", strings.TrimSpace(string(output)))
	}
	return err
}
