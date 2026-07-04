package restic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

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

	out, err := m.runner.Ls(ctx, snapshotID, path)
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

	buildNodes := func(prefix string) []FileNode {
		var nodes []FileNode
		for _, p := range rawPaths {
			if p == prefix {
				continue
			}
			rel := strings.TrimPrefix(p, prefix)
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
		return nodes
	}

	nodes := buildNodes(common)
	if len(nodes) == 0 {
		nodes = buildNodes(filepath.Dir(common))
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

	out, err := m.runner.Dump(ctx, snapshotID, path)
	if err != nil {
		return nil, fmt.Errorf("restic dump: %w", err)
	}
	return out, nil
}

func (m *Manager) DiffSnapshots(snapID1, snapID2 string) (*DiffResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	out, err := m.runner.Diff(ctx, snapID1, snapID2)
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
