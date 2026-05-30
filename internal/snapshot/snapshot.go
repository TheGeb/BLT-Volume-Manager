package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/example/blt-volume-manager/internal/applog"
	"github.com/example/blt-volume-manager/internal/constants"
)

type Type int

const (
	TypeNone Type = iota
	TypeBtrfs
	TypeZFS
)

func (t Type) String() string {
	switch t {
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

func Detect(path string) Type {
	applog.Debugf("detect_fs", "path=%s", path)
	cmd := exec.Command("stat", "-f", "-c", "%T", path)
	out, err := cmd.Output()
	if err != nil {
		return TypeNone
	}
	switch strings.TrimSpace(string(out)) {
	case "btrfs":
		return TypeBtrfs
	case "zfs":
		return TypeZFS
	}
	return TypeNone
}

func InitBtrfs(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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

	switch t {
	case TypeBtrfs:
		if !IsSubvolume(volPath) {
			return nil, fmt.Errorf("%s is not a btrfs subvolume; init with btrfs first", volPath)
		}
		if err := os.MkdirAll(snapDir, 0755); err != nil {
			return nil, fmt.Errorf("create snap dir: %w", err)
		}
		if err := btrfsCreate(volPath, accessPath); err != nil {
			return nil, err
		}
	case TypeZFS:
		dataset, err := zfsDataset(volPath)
		if err != nil {
			return nil, fmt.Errorf("resolve zfs dataset: %w", err)
		}
		snapName := volName + constants.ColdSnapSuffix
		fullSnap := dataset + "@" + snapName
		if err := zfsCreateSnapshot(fullSnap, accessPath); err != nil {
			return nil, err
		}
		info.zfsSnap = fullSnap
	}

	return info, nil
}

func Remove(info *SnapInfo) error {
	switch info.Subtype {
	case TypeBtrfs:
		return btrfsRemove(info.AccessPath)
	case TypeZFS:
		return zfsRemoveSnapshot(info.zfsSnap, info.AccessPath)
	}
	return nil
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
