package restic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Manager wraps restic operations. It expects restic in PATH and environment
// configured (RESTIC_REPOSITORY, RESTIC_PASSWORD, AWS envs for S3).

type Manager struct{}

type Snapshot struct {
	ID      string    `json:"id"`
	ShortID string    `json:"short_id"`
	Time    time.Time `json:"time"`
	Tags    []string  `json:"tags"`
	Paths   []string  `json:"paths"`
}

func NewManager() *Manager { return &Manager{} }

func (m *Manager) Backup(path, tag string) error {
	args := []string{"backup", path}
	if tag != "" {
		args = append(args, "--tag", tag)
	}
	compression := os.Getenv("RESTIC_COMPRESSION")
	if compression == "" {
		compression = "auto"
	}
	args = append(args, "--compression", compression)
	cmd, err := resticCommand(context.Background(), args...)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) ListSnapshots() ([]Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd, err := resticCommand(ctx, "snapshots", "--json")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		if isRepositoryMissing(string(out)) {
			return nil, nil
		}
		return nil, err
	}

	var snapshots []Snapshot
	if err := json.Unmarshal(out, &snapshots); err != nil {
		return nil, err
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Time.After(snapshots[j].Time)
	})
	return snapshots, nil
}

func (m *Manager) TagSnapshot(snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	cmd, err := resticCommand(context.Background(), "tag", "--add", tag, snapshotID)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) UntagSnapshot(snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	cmd, err := resticCommand(context.Background(), "tag", "--remove", tag, snapshotID)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) RestoreIfExists(path, preferred string) error {
	// preferred: "hot","cold","latest"
	// We look for snapshots and restore the most appropriate one.
	// Simplified: always restore latest snapshot. Users can control tags via options.
	// run: restic snapshots --json | parse -> restic restore <id> --target <path>
	// For simplicity, try `restic snapshots --last 1` and restore it.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Build snapshots command depending on preference
	args := []string{"snapshots", "--last", "1", "--json"}
	if preferred == "hot" || preferred == "cold" {
		args = []string{"snapshots", "--tag", preferred, "--last", "1", "--json"}
	}
	cmd, err := resticCommand(ctx, args...)
	if err != nil {
		return err
	}
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		// no snapshots found for this preference
		return nil
	}

	// Try to parse the snapshot id from JSON to restore that specific snapshot.
	var snaps []map[string]interface{}
	if err := json.Unmarshal(out, &snaps); err != nil || len(snaps) == 0 {
		// fallback: attempt to restore latest with tag if provided
		rargs := []string{"restore", "latest", "--target", path}
		if preferred == "hot" || preferred == "cold" {
			rargs = append(rargs, "--tag", preferred)
		}
		r, err := resticCommand(ctx, rargs...)
		if err != nil {
			return err
		}
		r.Stdout = os.Stdout
		r.Stderr = os.Stderr
		return r.Run()
	}
	// get snapshot ID key - try "short_id" or "id"
	id := ""
	if v, ok := snaps[0]["short_id"]; ok {
		if s, ok := v.(string); ok {
			id = s
		}
	}
	if id == "" {
		if v, ok := snaps[0]["id"]; ok {
			if s, ok := v.(string); ok {
				id = s
			}
		}
	}
	if id == "" {
		// couldn't find id; fallback to restore latest
		r, err := resticCommand(ctx, "restore", "latest", "--target", path)
		if err != nil {
			return err
		}
		r.Stdout = os.Stdout
		r.Stderr = os.Stderr
		return r.Run()
	}

	r, err := resticCommand(ctx, "restore", id, "--target", path)
	if err != nil {
		return err
	}
	r.Stdout = os.Stdout
	r.Stderr = os.Stderr
	return r.Run()
}

func (m *Manager) StartSchedule(ctx context.Context, name, path string, hotInterval, coldInterval time.Duration) {
	// Start two tickers: hot and cold. Hot tags snapshots as "hot"; cold as "cold".
	hotTicker := time.NewTicker(hotInterval)
	coldTicker := time.NewTicker(coldInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				hotTicker.Stop()
				coldTicker.Stop()
				return
			case <-hotTicker.C:
				log.Printf("restic hot backup for %s", name)
				if err := m.Backup(path, "hot"); err != nil {
					log.Printf("hot backup error: %v", err)
				}
			case <-coldTicker.C:
				log.Printf("restic cold backup for %s", name)
				if err := m.Backup(path, "cold"); err != nil {
					log.Printf("cold backup error: %v", err)
				}
			}
		}
	}()
}

func (m *Manager) RepoExists() (bool, error) {
	return m.repositoryExists()
}

func (m *Manager) Init() error {
	return m.initRepository()
}

type RepoStats struct {
	TotalSize           int64 `json:"total_size"`
	TotalFileCount      int64 `json:"total_file_count"`
	TotalBlobCount      int64 `json:"total_blob_count"`
	TotalUncompressedSize int64 `json:"total_uncompressed_size"`
	CompressedSize      int64 `json:"compressed_size"`
	UniqueBlobCount     int64 `json:"unique_blob_count"`
	UniqueBlobSize      int64 `json:"unique_blob_size"`
}

func (m *Manager) Stats() (*RepoStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd, err := resticCommand(ctx, "stats", "--json", "--mode", "raw-data")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("restic stats: %w", err)
	}

	var stats RepoStats
	if err := json.Unmarshal(out, &stats); err != nil {
		return nil, fmt.Errorf("parse restic stats: %w", err)
	}
	return &stats, nil
}

func resticRepository() (string, error) {
	repo := strings.TrimSpace(os.Getenv("RESTIC_REPOSITORY"))
	if repo == "" {
		return "", errors.New("RESTIC_REPOSITORY must be set for restic operations")
	}

	repo = strings.TrimSuffix(repo, "/")
	repo += "/restic"
	return repo, nil
}

func (m *Manager) repositoryExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd, err := resticCommand(ctx, "snapshots", "--json")
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

func isRepositoryMissing(output string) bool {
	out := strings.ToLower(output)
	return strings.Contains(out, "not found") || strings.Contains(out, "does not exist") || strings.Contains(out, "not initialized")
}

func (m *Manager) initRepository() error {
	cmd, err := resticCommand(context.Background(), "init")
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) Check() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd, err := resticCommand(ctx, "check")
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd, err := resticCommand(ctx, "unlock")
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resticCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	repo, err := resticRepository()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "restic", append([]string{"-r", repo, "--no-lock"}, args...)...)
	cmd.Env = os.Environ()
	return cmd, nil
}
