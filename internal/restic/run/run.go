// Package run provides primitives for executing restic CLI commands.
//
// The Runner type offers several execution methods that differ in how they
// handle stdout and who controls the underlying exec.Cmd:
//
//   - Command / CommandForRepo — build the exec.Cmd without running it
//   - Run — fire-and-forget, stdout→os.Stdout
//   - Capture — run and return stdout as []byte
//   - CombinedCapture — run and return both stdout and stderr as []byte
//   - RunCommand — execute a pre-built exec.Cmd
package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

// Runner executes restic CLI commands.
type Runner struct {
	Repo string
}

// Command builds a restic command targeting the runner's repo and returns the
// exec.Cmd without running it. Use this when the caller needs to customize
// the command's stdin, working directory, or wait pattern.
func (r *Runner) Command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	return r.CommandForRepo(ctx, r.Repo, args...)
}

// CommandForRepo is like Command but sets --repo=<repo> instead of r.Repo.
// Used for cross-repo operations such as restic copy.
func (r *Runner) CommandForRepo(ctx context.Context, repo string, args ...string) (*exec.Cmd, error) {
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

// Run executes a fire-and-forget restic command. Its stdout and stderr are
// written to the process's stdout and stderr. Use for commands like forget,
// tag, init, check, repair, unlock, and backup where the output is not parsed.
func (r *Runner) Run(ctx context.Context, args ...string) error {
	cmd, err := r.Command(ctx, args...)
	if err != nil {
		return err
	}
	return r.RunCommand(cmd)
}

// Capture executes a restic command and returns its stdout as bytes.
// Stderr is not captured and goes to os.Stderr. Use for commands like
// snapshots and stats whose JSON output needs to be parsed.
func (r *Runner) Capture(ctx context.Context, args ...string) ([]byte, error) {
	cmd, err := r.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

// CombinedCapture is like Capture but returns both stdout and stderr combined.
// Use for commands where stderr may contain informative messages (e.g.
// repository existence checks).
func (r *Runner) CombinedCapture(ctx context.Context, args ...string) ([]byte, error) {
	cmd, err := r.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

// RunCommand executes a pre-built exec.Cmd and writes its stdout and stderr
// to the process's stdout and stderr. Use this when you already have a
// prepared exec.Cmd (e.g. with a custom working directory set).
func (r *Runner) RunCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsRepositoryMissing checks whether a restic error output indicates
// the repository does not exist or has not been initialized.
func IsRepositoryMissing(output string) bool {
	out := strings.ToLower(output)
	return strings.Contains(out, "not found") || strings.Contains(out, "does not exist") || strings.Contains(out, "not initialized")
}
