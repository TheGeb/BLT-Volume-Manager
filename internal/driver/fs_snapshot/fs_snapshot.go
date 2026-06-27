package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
)

type Type int

const (
	TypeNone Type = iota
	TypeBtrfs
	TypeZFS
)

const (
	ColdSuffix       = "-cold-snapshot"
	PreRestoreSuffix = "-pre-restore"
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

type Info struct {
	VolName    string
	SnapDir    string
	AccessPath string
	Subtype    Type
	ZfsSnap    string
}

type Provider interface {
	Type() Type
	MatchFSType(fsType string) bool
	CreateSnapshot(volPath, accessPath, volName string, info *Info) error
	RemoveSnapshot(info *Info) error
	Init(path string, opts map[string]string) error
	Destroy(path string) error
}

var rootDataset string

func InitRoot(root string) {
	t := Detect(root)
	if t == TypeZFS {
		if p, err := ZFSDataset(root); err == nil {
			rootDataset = p
		}
	}
}

func RootDataset() string {
	return rootDataset
}

var (
	providers = map[Type]Provider{}
	typeOrder []Type
)

func RegisteredTypes() []Type {
	return typeOrder
}

func Register(s Provider) {
	t := s.Type()
	providers[t] = s
	typeOrder = append(typeOrder, t)
}

func Detect(path string) Type {
	log.Debugf("detect_fs", "path=%s", path)
	cmd := exec.Command("stat", "-f", "-c", "%T", path)
	out, err := cmd.Output()
	if err != nil {
		return TypeNone
	}
	fsType := strings.TrimSpace(string(out))
	for _, t := range typeOrder {
		if s := providers[t]; s != nil && s.MatchFSType(fsType) {
			return t
		}
	}
	return TypeNone
}

func Create(volPath, snapDir, volName string) (*Info, error) {
	t := Detect(volPath)
	if t == TypeNone {
		return nil, fmt.Errorf("no supported snapshot filesystem at %s", volPath)
	}
	accessPath := filepath.Join(snapDir, volName+ColdSuffix)
	info := &Info{
		VolName:    volName,
		SnapDir:    snapDir,
		AccessPath: accessPath,
		Subtype:    t,
	}

	s, ok := providers[t]
	if !ok {
		return nil, fmt.Errorf("unsupported filesystem type for %s", volPath)
	}
	if err := s.CreateSnapshot(volPath, accessPath, volName, info); err != nil {
		return nil, err
	}
	return info, nil
}

func Remove(info *Info) error {
	if info.Subtype == TypeNone {
		return nil
	}
	s, ok := providers[info.Subtype]
	if !ok {
		return nil
	}
	return s.RemoveSnapshot(info)
}

func ListOrphaned(snapDir string) ([]*Info, error) {
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Info
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ColdSuffix) {
			continue
		}
		volName := strings.TrimSuffix(name, ColdSuffix)
		out = append(out, &Info{
			VolName:    volName,
			SnapDir:    snapDir,
			AccessPath: filepath.Join(snapDir, name),
		})
	}
	return out, nil
}

func FromString(s string) Type {
	switch s {
	case "btrfs":
		return TypeBtrfs
	case "zfs":
		return TypeZFS
	}
	return TypeNone
}

func ResolveType(info *Info) error {
	t := Detect(info.AccessPath)
	if t == TypeNone {
		return fmt.Errorf("unsupported filesystem type at %s", info.AccessPath)
	}
	info.Subtype = t
	return nil
}

func ZFSDataset(path string) (string, error) {
	log.Debugf("findmnt_lookup", "path=%s", path)
	cmd := exec.Command("findmnt", "-T", path, "-o", "SOURCE", "-n")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("findmnt for %s: %w", path, err)
	}
	source := strings.TrimSpace(string(out))
	if source == "" {
		return "", fmt.Errorf("no mount source for %s", path)
	}
	if !strings.Contains(source, "/") {
		return "", fmt.Errorf("mount source %q does not look like a ZFS dataset", source)
	}
	return source, nil
}

func InitFs(volPath string, t Type, opts map[string]string) error {
	s, ok := providers[t]
	if !ok {
		return fmt.Errorf("unsupported snapshot filesystem type: %s", t)
	}
	return s.Init(volPath, opts)
}

func DestroyVolume(volPath string, t Type) error {
	s, ok := providers[t]
	if !ok {
		return fmt.Errorf("unsupported snapshot filesystem type: %s", t)
	}
	return s.Destroy(volPath)
}
