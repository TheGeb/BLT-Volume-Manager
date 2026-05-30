package restic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/internal/applog"
	"github.com/example/blt-volume-manager/internal/constants"
	"github.com/example/blt-volume-manager/internal/store"
)

// FindSnapshotByHash searches for a snapshot matching the criteria derived from host + time + paths.
func (m *Manager) FindSnapshotByHash(hash string) (*Snapshot, error) {
	snapshots, err := m.ListSnapshots()
	if err != nil {
		return nil, err
	}

	for _, s := range snapshots {
		// Use same length as shortID
		fullHash := m.generateHash(s)
		shortHash := fullHash[:len(s.ShortID)]
		applog.Debugf("comparing_hash", "hash=%x snapshot=%s", hash, s.ID)
		if shortHash == hash {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found for hash %s", hash)
}

func (m *Manager) generateHash(s Snapshot) string {
	// Sort paths to ensure consistency
	paths := make([]string, len(s.Paths))
	copy(paths, s.Paths)
	sort.Strings(paths)

	data := s.Hostname + s.Time.Format(time.RFC3339Nano) + s.Tree + strings.Join(paths, ",")
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// Manager wraps restic operations for a single repository.
type Manager struct {
	repo string
	s3Store store.S3Store
}

func (m *Manager) SetS3Store(s3Store store.S3Store) {
	m.s3Store = s3Store
}

type Snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Tree     string    `json:"tree"`
	Tags     []string  `json:"tags"`
	Paths    []string  `json:"paths"`
	Hostname string    `json:"hostname"`
}

// NewManager creates a Manager for the given restic repository URL/path.
func NewManager(repo string) *Manager { return &Manager{repo: repo} }

// Repo returns the repository path/URL.
func (m *Manager) Repo() string { return m.repo }

func (m *Manager) Backup(path, tag string) error {
	return m.BackupInDir(path, tag, "")
}

func (m *Manager) BackupInDir(path, tag, workDir string) error {
	args := m.backupArgs(path, tag, "")
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

func (m *Manager) BackupAt(path, tag string, t time.Time) error {
	args := m.backupArgs(path, tag, "")
	if !t.IsZero() {
		args = append(args, "--time", t.Format(time.RFC3339))
	}
	return m.runSimple(context.Background(), args...)
}

func (m *Manager) backupArgs(path, tag, extra string) []string {
	args := []string{"backup", path}
	if tag != "" {
		args = append(args, "--tag", tag)
	}
	compression := os.Getenv("RESTIC_COMPRESSION")
	if compression == "" {
		compression = "auto"
	}
	args = append(args, "--compression", compression)
	if extra != "" {
		args = append(args, extra)
	}
	return args
}

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

func (m *Manager) ListSnapshots() ([]Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "snapshots", "--no-lock", "--json")
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

func (m *Manager) ForgetSnapshot(snapshotID string) error {
	if snapshotID == "" {
		return errors.New("snapshot ID is required")
	}
	return m.runSimple(context.Background(), "forget", snapshotID)
}

func (m *Manager) TagSnapshot(snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	return m.runSimple(context.Background(), "tag", "--add", tag, snapshotID)
}

func (m *Manager) UntagSnapshot(snapshotID, tag string) error {
	if snapshotID == "" || tag == "" {
		return errors.New("snapshot ID and tag are required")
	}
	return m.runSimple(context.Background(), "tag", "--remove", tag, snapshotID)
}

func (m *Manager) RestoreIfExists(path, preferred string) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
	defer cancel()

	args := []string{"snapshots", "--no-lock", "--last", "1", "--json"}
	if preferred == constants.BackupTagHot || preferred == constants.BackupTagCold {
		args = []string{"snapshots", "--no-lock", "--tag", preferred, "--last", "1", "--json"}
	}
	cmd, err := m.resticCommand(ctx, args...)
	if err != nil {
		return err
	}
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}

	var snaps []map[string]interface{}
	if err := json.Unmarshal(out, &snaps); err != nil || len(snaps) == 0 {
		rargs := []string{"restore", "latest", "--target", path}
		if preferred == constants.BackupTagHot || preferred == constants.BackupTagCold {
			rargs = append(rargs, "--tag", preferred)
		}
		r, err := m.resticCommand(ctx, rargs...)
		if err != nil {
			return err
		}
		r.Stdout = os.Stdout
		r.Stderr = os.Stderr
		return r.Run()
	}
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
		r, err := m.resticCommand(ctx, "restore", "latest", "--target", path)
		if err != nil {
			return err
		}
		r.Stdout = os.Stdout
		r.Stderr = os.Stderr
		return r.Run()
	}

	r, err := m.resticCommand(ctx, "restore", id, "--target", path)
	if err != nil {
		return err
	}
	r.Stdout = os.Stdout
	r.Stderr = os.Stderr
	return r.Run()
}

func (m *Manager) RepoExists() (bool, error) {
	return m.repositoryExists()
}

func (m *Manager) Init() error {
	return m.initRepository()
}

type RepoStats struct {
	TotalSize             int64 `json:"total_size"`
	TotalFileCount        int64 `json:"total_file_count"`
	TotalBlobCount        int64 `json:"total_blob_count"`
	TotalUncompressedSize int64 `json:"total_uncompressed_size"`
	UniqueBlobCount       int64 `json:"unique_blob_count"`
	UniqueBlobSize        int64 `json:"unique_blob_size"`
}

func (m *Manager) Stats() (*RepoStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "stats", "--no-lock", "--json", "--mode", "raw-data")
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		applog.Errorf("restic_stats_failed", err, "stderr=%s", string(out))
		return nil, fmt.Errorf("restic stats: %w", err)
	}

	var stats RepoStats
	if err := json.Unmarshal(out, &stats); err != nil {
		applog.Errorf("parse_restic_stats_failed", err, "raw=%s", string(out))
		return nil, fmt.Errorf("parse restic stats: %w", err)
	}
	return &stats, nil
}

type SnapshotSizeResult struct {
	TotalSize      int64 `json:"total_size"`
	TotalFileCount int64 `json:"total_file_count"`
}

func (m *Manager) SnapshotStats(snapshotID string) (*SnapshotSizeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
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

func (m *Manager) RestoreSnapshot(snapshotID, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutMedium)
	defer cancel()
	return m.runSimple(ctx, "restore", snapshotID, "--target", target)
}

// SetRestorePoint stores the snapshot as the restore-point for its volume in S3.
func (m *Manager) SetRestorePoint(snapshotID, volume string) error {
	if snapshotID == "" {
		return errors.New("snapshot ID is required")
	}
	if m.s3Store == nil {
		return errors.New("S3 store not configured for restore points")
	}

	var fallbackHash string
	if snap, err := m.findSnapshotByID(snapshotID); err == nil {
		fullHash := m.generateHash(*snap)
		fallbackHash = fullHash[:len(snap.ShortID)]
	}

	rp := store.RestorePoint{
		SnapshotID:   snapshotID,
		FallbackHash: fallbackHash,
	}
	if err := m.s3Store.WriteRestorePoint(volume, rp); err != nil {
		return fmt.Errorf("write restore point: %w", err)
	}

	return nil
}

// FindRestorePoint reads the restore-point from S3 for the given volume path.
// Returns the snapshot ID string, or empty string if none found.
func (m *Manager) FindRestorePoint(volPath string) (string, error) {
	if m.s3Store == nil {
		return "", nil
	}

	marker := "/volumes/"
	idx := strings.Index(volPath, marker)
	if idx < 0 {
		return "", nil
	}
	targetVolume := strings.TrimPrefix(volPath[idx+len(marker):], "/")
	if targetVolume == "" {
		return "", nil
	}

	rp, err := m.s3Store.ReadRestorePoint(targetVolume)
	if err != nil {
		return "", fmt.Errorf("read restore point: %w", err)
	}
	if rp == nil || rp.SnapshotID == "" {
		return "", nil
	}
	return rp.SnapshotID, nil
}

// DeleteRestorePoint removes the restore-point from S3 for the given volume.
func (m *Manager) DeleteRestorePoint(volume string) error {
	if m.s3Store == nil {
		return nil
	}
	return m.s3Store.DeleteRestorePoint(volume)
}

func (m *Manager) findSnapshotByID(snapshotID string) (*Snapshot, error) {
	snapshots, err := m.ListSnapshots()
	if err != nil {
		return nil, err
	}
	for i := range snapshots {
		if snapshots[i].ID == snapshotID || snapshots[i].ShortID == snapshotID {
			return &snapshots[i], nil
		}
	}
	return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

// FileNode represents a single file/directory entry from restic ls.
type FileNode struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	FullPath    string `json:"full_path,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Permissions string `json:"permissions,omitempty"`
	ModTime     string `json:"mtime,omitempty"`
}

// DiffResult represents a diff between two snapshots.
type DiffResult struct {
	ChangeSets []DiffChange `json:"change_sets"`
}

type DiffChange struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

// ListSnapshotFiles lists files in a snapshot at the given path.
func (m *Manager) ListSnapshotFiles(snapshotID, path string) ([]FileNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
	defer cancel()

	args := []string{"ls", "--no-lock", snapshotID}
	if path != "" && path != "/" {
		args = append(args, path)
	}
	cmd, err := m.resticCommand(ctx, args...)
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("restic ls: %w", err)
	}

	var rawPaths []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "snapshot ") || strings.HasPrefix(line, "restic ") {
			continue
		}
		rawPaths = append(rawPaths, line)
	}
	if len(rawPaths) == 0 {
		return nil, nil
	}

	dirSet := map[string]bool{}
	for _, p := range rawPaths {
		prefix := p + "/"
		for _, q := range rawPaths {
			if strings.HasPrefix(q, prefix) && q != p {
				dirSet[p] = true
				break
			}
		}
	}

	// Strip the common root prefix so display paths are relative
	common := commonPathPrefix(rawPaths)

	var nodes []FileNode
	for _, p := range rawPaths {
		if p == common {
			continue
		}
		rel := strings.TrimPrefix(p, common)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || rel == "." {
			continue
		}
		nodes = append(nodes, FileNode{
			Name:     filepath.Base(rel),
			Type:     map[bool]string{true: "dir", false: "file"}[dirSet[p]],
			Path:     "/" + rel,
			FullPath: p,
		})
	}

	// If stripping the common root left us with nothing (e.g. a single path),
	// strip one more level so the root folder itself still shows
	if len(nodes) == 0 {
		common = filepath.Dir(common)
		for _, p := range rawPaths {
			if p == common {
				continue
			}
			rel := strings.TrimPrefix(p, common)
			rel = strings.TrimPrefix(rel, "/")
			if rel == "" || rel == "." {
				continue
			}
			nodes = append(nodes, FileNode{
				Name:     filepath.Base(rel),
				Type:     map[bool]string{true: "dir", false: "file"}[dirSet[p]],
				Path:     "/" + rel,
				FullPath: p,
			})
		}
	}
	return nodes, nil
}

func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	prefix := paths[0]
	for _, p := range paths[1:] {
		for !strings.HasPrefix(p, prefix) {
			prefix = filepath.Dir(prefix)
			if prefix == "/" || prefix == "." {
				return ""
			}
		}
	}
	return prefix
}

// DumpFile returns the contents of a file within a snapshot.
func (m *Manager) DumpFile(snapshotID, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "dump", "--no-lock", snapshotID, path)
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("restic dump: %w", err)
	}
	return out, nil
}

// DiffSnapshots returns the diff between two snapshots.
func (m *Manager) DiffSnapshots(snapID1, snapID2 string) (*DiffResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "diff", "--no-lock", snapID1, snapID2)
	if err != nil {
		return nil, err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("restic diff: %w\n%s", err, string(out))
	}

	var changes []DiffChange
	groups := map[string][]string{}
	order := []string{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 2 || line[1] != ' ' {
			continue
		}
		prefix := string(line[0])
		path := strings.TrimSpace(line[2:])
		if path == "" {
			continue
		}
		var typ string
		switch prefix {
		case "+":
			typ = "added"
		case "-":
			typ = "removed"
		case "M":
			typ = "modified"
		case "U":
			typ = "metadata"
		case "T":
			typ = "type-changed"
		default:
			continue
		}
		if _, ok := groups[typ]; !ok {
			order = append(order, typ)
		}
		groups[typ] = append(groups[typ], path)
	}

	for _, typ := range order {
		changes = append(changes, DiffChange{Type: typ, Paths: groups[typ]})
	}

	return &DiffResult{ChangeSets: changes}, nil
}

func (m *Manager) repositoryExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutShort)
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

func isRepositoryMissing(output string) bool {
	out := strings.ToLower(output)
	return strings.Contains(out, "not found") || strings.Contains(out, "does not exist") || strings.Contains(out, "not initialized")
}

func (m *Manager) initRepository() error {
	return m.runSimple(context.Background(), "init")
}

func (m *Manager) Check(noLock bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutMedium)
	defer cancel()
	args := []string{"check"}
	if noLock {
		args = append(args, "--no-lock")
	}
	return m.runSimple(ctx, args...)
}

func (m *Manager) Repair() error {
	if err := m.Unlock(); err != nil {
		applog.Warnf("repair_unlock_failed_continuing", "error=%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.ResticTimeoutLong)
	defer cancel()
	return m.runSimple(ctx, "repair", "index")
}

func (m *Manager) Unlock() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return m.runSimple(ctx, "unlock")
}

func (m *Manager) resticCommand(ctx context.Context, args ...string) (*exec.Cmd, error) {
	global := []string{"-r", m.repo}
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "trace":
		global = append(global, "--verbose=2")
	case "debug":
		global = append(global, "--verbose=1")
	}

	fullArgs := append(global, args...)
	applog.Debugf("restic_command", "args=%s", strings.Join(fullArgs, " "))
	cmd := exec.CommandContext(ctx, "restic", fullArgs...)
	cmd.Env = os.Environ()
	return cmd, nil
}
