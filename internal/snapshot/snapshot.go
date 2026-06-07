package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
)

type Type int

const (
	TypeNone Type = iota
	TypeBtrfs
	TypeZFS
)

func (t Type) String() string {
	switch t {
	case TypeNone:
		return ""
	case TypeBtrfs:
		return "btrfs"
	case TypeZFS:
		return "zfs"
	}
	return ""
}

type SnapInfo struct {
	VolName    string
	SnapDir    string
	AccessPath string
	Subtype    Type
	zfsSnap    string
}

// Snapshotter defines filesystem-specific snapshot operations.
// Implementations register themselves via init().
type Snapshotter interface {
	Type() Type
	MatchFSType(fsType string) bool
	CreateSnapshot(volPath, accessPath, volName string, info *SnapInfo) error
	RemoveSnapshot(info *SnapInfo) error
}

var (
	snapshotters = map[Type]Snapshotter{}
	typeOrder    []Type
)

func register(s Snapshotter) {
	t := s.Type()
	snapshotters[t] = s
	typeOrder = append(typeOrder, t)
}

// Detect determines the snapshot-capable filesystem type at path.
func Detect(path string) Type {
	applog.Debugf("detect_fs", "path=%s", path)
	cmd := exec.Command("stat", "-f", "-c", "%T", path)
	out, err := cmd.Output()
	if err != nil {
		return TypeNone
	}
	fsType := strings.TrimSpace(string(out))
	for _, t := range typeOrder {
		if s := snapshotters[t]; s != nil && s.MatchFSType(fsType) {
			return t
		}
	}
	return TypeNone
}

func InitBtrfs(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	applog.Debugf("btrfs_create_subvolume", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "create", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("btrfs subvolume create: %w\n%s", err, string(out))
	}
	return nil
}

func ParentDataset(path string) (string, error) {
	return zfsDataset(path)
}

func InitZFS(path, parentDataset string) (string, error) {
	name := filepath.Base(path)
	full := parentDataset + "/" + name

	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("remove path: %w", err)
	}

	applog.Debugf("zfs_create", "path=%s dataset=%s", path, full)
	cmd := exec.Command("zfs", "create", "-o", "mountpoint="+path, full)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("zfs create: %w\n%s", err, string(out))
	}
	return full, nil
}

func IsSubvolume(path string) bool {
	applog.Debugf("btrfs_check_subvolume", "path=%s", path)
	cmd := exec.Command("btrfs", "subvolume", "show", path)
	return cmd.Run() == nil
}

func Create(volPath, snapDir, volName string) (*SnapInfo, error) {
	t := Detect(volPath)
	if t == TypeNone {
		return nil, fmt.Errorf("no supported snapshot filesystem at %s", volPath)
	}
	accessPath := filepath.Join(snapDir, volName+constants.ColdSnapSuffix)
	info := &SnapInfo{
		VolName:    volName,
		SnapDir:    snapDir,
		AccessPath: accessPath,
		Subtype:    t,
	}

	s, ok := snapshotters[t]
	if !ok {
		return nil, fmt.Errorf("unsupported filesystem type for %s", volPath)
	}
	if err := s.CreateSnapshot(volPath, accessPath, volName, info); err != nil {
		return nil, err
	}
	return info, nil
}

func Remove(info *SnapInfo) error {
	if info.Subtype == TypeNone {
		return nil
	}
	s, ok := snapshotters[info.Subtype]
	if !ok {
		return nil
	}
	return s.RemoveSnapshot(info)
}

func ListOrphaned(snapDir string) ([]*SnapInfo, error) {
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*SnapInfo
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, constants.ColdSnapSuffix) {
			continue
		}
		volName := strings.TrimSuffix(name, constants.ColdSnapSuffix)
		out = append(out, &SnapInfo{
			VolName:    volName,
			SnapDir:    snapDir,
			AccessPath: filepath.Join(snapDir, name),
		})
	}
	return out, nil
}

func ResolveType(info *SnapInfo) error {
	t := Detect(info.AccessPath)
	if t == TypeNone {
		return fmt.Errorf("unsupported filesystem type at %s", info.AccessPath)
	}
	info.Subtype = t
	return nil
}
