package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ListSnapshotsOpts struct {
	Hosts  []string
	Latest int
	Tags   []string
}

func (m *Manager) ListSnapshots() ([]Snapshot, error) {
	return m.ListSnapshotsWithOpts(nil)
}

func (m *Manager) ListSnapshotsWithOpts(opts *ListSnapshotsOpts) ([]Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

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

	cmd, err := m.resticCommand(ctx, args...)
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

type HostSnapshots struct {
	Host      string     `json:"host"`
	Snapshots []Snapshot `json:"snapshots"`
}

func (m *Manager) SnapshotHosts(latest int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	args := []string{"snapshots", "--no-lock", "--json", "--group-by", "host", "--latest", "1"}
	cmd, err := m.resticCommand(ctx, args...)
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

	var raw []json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	hostSet := make(map[string]bool)
	for _, item := range raw {
		var group struct {
			GroupKey struct {
				Hostname string `json:"hostname"`
			} `json:"group_key"`
		}
		if json.Unmarshal(item, &group) == nil && group.GroupKey.Hostname != "" {
			hostSet[group.GroupKey.Hostname] = true
		}
	}

	hosts := make([]string, 0, len(hostSet))
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts, nil
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
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	args := []string{"snapshots", "--no-lock", "--last", "1", "--json"}
	if preferred == BackupTagHot || preferred == BackupTagCold {
		args = []string{"snapshots", "--no-lock", "--tag", preferred, "--last", "1", "--json"}
	}
	cmd, err := m.resticCommand(ctx, args...)
	if err != nil {
		return err
	}
	out, err := cmd.Output()
	if err != nil {
		if len(out) == 0 {
			return nil
		}
		return err
	}
	if len(out) == 0 {
		return nil
	}

	var snaps []map[string]any
	if err := json.Unmarshal(out, &snaps); err != nil {
		return fmt.Errorf("parse snapshot list: %w", err)
	}
	if len(snaps) == 0 {
		rargs := []string{"restore", "latest", "--target", path}
		if preferred == BackupTagHot || preferred == BackupTagCold {
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

func (m *Manager) RestoreSnapshot(snapshotID, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutMedium)
	defer cancel()
	return m.runSimple(ctx, "restore", snapshotID, "--target", target)
}

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

type DiffResult struct {
	ChangeSets []DiffChange `json:"change_sets"`
}

type DiffChange struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func (m *Manager) ListSnapshotFiles(snapshotID, path string) ([]FileNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
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

func (m *Manager) DumpFile(snapshotID, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
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

func (m *Manager) DiffSnapshots(snapID1, snapID2 string) (*DiffResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	cmd, err := m.resticCommand(ctx, "diff", "--no-lock", "--json", snapID1, snapID2)
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("restic diff: %w\n%s", err, string(out))
	}

	groups := map[string][]string{}
	order := []string{}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg struct {
			Type     string `json:"message_type"`
			Path     string `json:"path"`
			Modifier string `json:"modifier"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "change" || msg.Path == "" {
			continue
		}
		typ := modifierToChangeType(msg.Modifier)
		if typ == "" {
			continue
		}
		if _, ok := groups[typ]; !ok {
			order = append(order, typ)
		}
		groups[typ] = append(groups[typ], msg.Path)
	}

	changes := make([]DiffChange, 0, len(order))
	for _, typ := range order {
		changes = append(changes, DiffChange{Type: typ, Paths: groups[typ]})
	}
	return &DiffResult{ChangeSets: changes}, nil
}

func modifierToChangeType(modifier string) string {
	for _, ch := range modifier {
		switch ch {
		case '+':
			return "added"
		case '-':
			return "removed"
		case 'M':
			return "modified"
		case 'U':
			return "metadata"
		case 'T':
			return "type-changed"
		}
	}
	return ""
}
