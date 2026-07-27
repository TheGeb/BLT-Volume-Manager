package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

type ListSnapshotsOpts struct {
	Hosts  []string
	Latest int
	Tags   []string
}

func (r *Runner) Snapshots(ctx context.Context, opts *ListSnapshotsOpts) ([]byte, error) {
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
	return r.capture(ctx, args...)
}

func (r *Runner) SnapshotsLast(ctx context.Context, tagFilter string) ([]byte, error) {
	args := []string{"snapshots", "--no-lock", "--last", "1", "--json"}
	if tagFilter != "" {
		args = append(args, "--tag", tagFilter)
	}
	return r.capture(ctx, args...)
}

func (r *Runner) Forget(ctx context.Context, ids ...string) error {
	return r.run(ctx, append([]string{"forget"}, ids...)...)
}

func (r *Runner) ForgetAll(ctx context.Context) error {
	return r.run(ctx, "forget", "--keep-last", "0", "--prune")
}

func (r *Runner) TagAdd(ctx context.Context, snapshotID, tag string) error {
	return r.run(ctx, "tag", "--add", tag, snapshotID)
}

func (r *Runner) TagRemove(ctx context.Context, snapshotID, tag string) error {
	return r.run(ctx, "tag", "--remove", tag, snapshotID)
}

func (r *Runner) Restore(ctx context.Context, snapshotID, target string, tags ...string) error {
	args := []string{"restore", snapshotID, "--target", target}
	for _, t := range tags {
		args = append(args, "--tag", t)
	}
	return r.run(ctx, args...)
}

func (r *Runner) Backup(ctx context.Context, path string, tags []string) error {
	return r.run(ctx, backupArgs(path, tags)...)
}

func (r *Runner) BackupAt(ctx context.Context, path string, tags []string, t time.Time) error {
	args := backupArgs(path, tags)
	if !t.IsZero() {
		args = append(args, "--time", t.Format(time.RFC3339))
	}
	return r.run(ctx, args...)
}

func (r *Runner) BackupInDir(ctx context.Context, path string, tags []string, workDir string) error {
	cmd, err := r.command(ctx, backupArgs(path, tags)...)
	if err != nil {
		return err
	}
	cmd.Dir = workDir
	return r.runCommand(cmd)
}

func (r *Runner) Ls(ctx context.Context, snapshotID, path string) ([]byte, error) {
	args := []string{"ls", "--no-lock", snapshotID}
	if path != "" && path != "/" {
		args = append(args, path)
	}
	return r.capture(ctx, args...)
}

func (r *Runner) Dump(ctx context.Context, snapshotID, path string) ([]byte, error) {
	return r.capture(ctx, "dump", "--no-lock", snapshotID, path)
}

func (r *Runner) Diff(ctx context.Context, snapID1, snapID2 string) ([]byte, error) {
	return r.capture(ctx, "diff", "--no-lock", "--json", snapID1, snapID2)
}

func (r *Runner) Stats(ctx context.Context, mode string) ([]byte, error) {
	return r.capture(ctx, "stats", "--no-lock", "--json", "--mode", mode)
}

func (r *Runner) StatsSnapshot(ctx context.Context, snapshotID string) ([]byte, error) {
	return r.capture(ctx, "stats", "--no-lock", snapshotID, "--json")
}

func (r *Runner) Check(ctx context.Context, noLock bool) error {
	args := []string{"check"}
	if noLock {
		args = append(args, "--no-lock")
	}
	return r.run(ctx, args...)
}

func (r *Runner) Repair(ctx context.Context) error {
	return r.run(ctx, "repair", "index")
}

func (r *Runner) Init(ctx context.Context) error {
	_, err := r.combinedCapture(ctx, "init")
	return err
}

func (r *Runner) Copy(ctx context.Context, destRepo string, snapshotIDs ...string) error {
	args := []string{fmt.Sprintf("--from-repo=%s", r.Repo), "copy"}
	args = append(args, snapshotIDs...)
	if v := log.Verbosity(); v > 0 {
		args = append(args, fmt.Sprintf("--verbose=%d", v))
	}
	cmd, err := r.commandForRepo(ctx, destRepo, args...)
	if err != nil {
		return err
	}
	return r.runCommand(cmd)
}

func (r *Runner) Unlock(ctx context.Context) error {
	return r.run(ctx, "unlock")
}

func (r *Runner) HostSnapshots(ctx context.Context) ([]byte, error) {
	return r.capture(ctx, "snapshots", "--no-lock", "--json", "--group-by", "host", "--latest", "1")
}

func (r *Runner) RepoExists(ctx context.Context) ([]byte, error) {
	return r.combinedCapture(ctx, "snapshots", "--no-lock", "--json")
}

func backupArgs(path string, tags []string) []string {
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
