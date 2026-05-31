package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/appconfig"
	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
	"github.com/TheGeb/BLT-Volume-Manager/internal/locker"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/snapshot"
	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/volumepath"
	"github.com/docker/go-plugins-helpers/volume"
)

type volumeConfig struct {
	FsType string `json:"fs_type"`
}

type VolumeInfo struct {
	Name     string
	Path     string
	Lock     locker.Lock
	FsType   string
	attached int
	cancel   context.CancelFunc
}

type Driver struct {
	root       string
	resticBase string
	locker     locker.Locker
	vols       map[string]*VolumeInfo
	mu         sync.Mutex
	zfsParent  string
	s3Store    store.S3Store
}

func NewDriver(cfg appconfig.Config) *Driver {
	root := cfg.DataDir
	var l locker.Locker
	if cfg.S3Bucket != "" {
		if sl, err := locker.NewS3Locker(cfg.S3Bucket, cfg.S3Endpoint, cfg.S3Region, cfg.S3LockMaxMins); err == nil {
			l = sl
		} else {
			applog.Errorf("s3_locker_init_failed", err, "falling back to file locker")
			l = locker.NewFileLocker(filepath.Join(root, constants.LocksDir))
		}
	} else {
		l = locker.NewFileLocker(filepath.Join(root, constants.LocksDir))
	}

	zfsParent := ""
	if p, err := snapshot.ParentDataset(root); err == nil {
		zfsParent = p
	}

	drv := &Driver{
		root:       root,
		resticBase: cfg.ResticBase,
		locker:     l,
		vols:       make(map[string]*VolumeInfo),
		zfsParent:  zfsParent,
	}

	if cfg.S3Bucket != "" {
		if rw, err := store.NewS3Store(store.S3StoreConfig{
			S3Bucket:   cfg.S3Bucket,
			S3Endpoint: cfg.S3Endpoint,
			Region:     cfg.S3Region,
		}); err == nil {
			drv.s3Store = rw
		} else {
			applog.Errorf("s3_store_init_failed", err, "restore points will not be available")
		}
	}

	go drv.monitorOrphanedSnapshots(context.Background())
	return drv
}

func (d *Driver) ResticManager(volName string) *restic.Manager {
	m := restic.NewManager(d.resticBase + "/restic/" + volName)
	if d.s3Store != nil {
		m.SetS3Store(d.s3Store)
	}
	return m
}

func (d *Driver) Create(r *volume.CreateRequest) error {
	name := r.Name
	volPath := volumepath.VolumePath(d.root, name)

	fsType := ""
	if r.Options != nil {
		fsType = d.initFsType(r.Options, name, volPath)
	}
	if fsType == "" {
		if err := os.MkdirAll(volPath, 0o755); err != nil {
			return err
		}
	}
	if err := d.writeVolumeConfig(volPath, &volumeConfig{FsType: fsType}); err != nil {
		return fmt.Errorf("write volume config: %w", err)
	}

	ctx := context.Background()
	lock, err := d.locker.Acquire(ctx, name)
	if err == nil {
		// FIXME: Should it take latest even if it's a hot backup? Maybe send an alert
		restoreMode := "latest"
		if r.Options != nil {
			restoreMode = r.Options["restore"]
		}
		if restoreMode == "" {
			restoreMode = "latest"
		}
		rm := d.ResticManager(name)
		if err := rm.RestoreIfExists(volPath, restoreMode); err != nil {
			applog.Errorf("restore_failed", err, "volume=%s", name)
			if lock != nil {
				if rerr := lock.Release(); rerr != nil {
					applog.Errorf("release_lock_failed", rerr, "volume=%s", name)
				}
			}
		}
	} else {
		applog.Warnf("create_lock_acquire_failed", "volume=%s error=%v", name, err)
	}

	return nil
}

func (d *Driver) initFsType(opts map[string]string, name, volPath string) string {
	for _, candidate := range []string{"btrfs", "zfs"} {
		v, ok := opts[candidate]
		if !ok || !strings.EqualFold(v, "true") {
			continue
		}
		detected := snapshot.Detect(volPath)
		if detected.String() == candidate {
			return candidate
		}
		parent := filepath.Dir(volPath)
		if snapshot.Detect(parent) != TypeFromString(candidate) {
			applog.Warnf("volume_fs_mismatch", "volume=%s fs=%s parent=%s", name, candidate, parent)
			return ""
		}
		switch candidate {
		case "btrfs":
			if err := snapshot.InitBtrfs(volPath); err != nil {
				applog.Errorf("btrfs_init_failed", err, "volume=%s", name)
				return ""
			}
			applog.Infof("btrfs_initialized", "volume=%s", name)
		case "zfs":
			parentDS := d.zfsParent
			if p, ok := opts["zfs-pool"]; ok && p != "" {
				parentDS = p
			}
			if parentDS == "" {
				applog.Warnf("zfs_no_parent_dataset", "volume=%s", name)
				return ""
			}
			if _, err := snapshot.InitZFS(volPath, parentDS); err != nil {
				applog.Errorf("zfs_init_failed", err, "volume=%s parent=%s", name, parentDS)
				return ""
			}
			applog.Infof("zfs_initialized", "volume=%s dataset=%s", name, parentDS)
		}
		return candidate
	}
	return ""
}

func TypeFromString(s string) snapshot.Type {
	switch s {
	case "btrfs":
		return snapshot.TypeBtrfs
	case "zfs":
		return snapshot.TypeZFS
	}
	return snapshot.TypeNone
}

func (d *Driver) Remove(r *volume.RemoveRequest) error {
	name := r.Name
	d.mu.Lock()
	vi, ok := d.vols[name]
	var viLock locker.Lock
	if ok && vi != nil {
		viLock = vi.Lock
	}
	d.mu.Unlock()

	volPath := volumepath.VolumePath(d.root, name)
	cfg := d.readVolumeConfig(volPath)
	fsType := ""
	if cfg != nil {
		fsType = cfg.FsType
	}

	rm := d.ResticManager(name)
	if err := d.coldBackup(name, volPath, fsType, rm); err != nil {
		applog.Errorf("final_backup_failed", err, "volume=%s", name)
	}
	switch fsType {
	case "btrfs":
		applog.Debugf("btrfs_delete", "path=%s", volPath)
		if err := exec.Command("btrfs", "subvolume", "delete", volPath).Run(); err != nil {
			applog.Errorf("btrfs_delete_failed", err, "path=%s", volPath)
		}
	case "zfs":
		if ds, err := snapshot.ParentDataset(volPath); err == nil {
			applog.Debugf("zfs_destroy", "dataset=%s", ds)
			if err := exec.Command("zfs", "destroy", "-r", ds).Run(); err != nil {
				applog.Errorf("zfs_destroy_failed", err, "dataset=%s", ds)
			}
		}
	default:
		if err := os.RemoveAll(volPath); err != nil {
			applog.Errorf("remove_volume_dir_failed", err, "path=%s", volPath)
		}
	}
	if viLock != nil {
		if err := viLock.Release(); err != nil {
			applog.Errorf("release_lock_failed", err, "volume=%s", name)
		}
	}
	return nil
}

func (d *Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	name := r.Name
	volPath := volumepath.VolumePath(d.root, name)
	if err := os.MkdirAll(volPath, 0o755); err != nil {
		return nil, fmt.Errorf("create volume dir: %w", err)
	}

	d.mu.Lock()
	vi, ok := d.vols[name]
	if !ok {
		cfg := d.readVolumeConfig(volPath)
		fsType := ""
		if cfg != nil {
			fsType = cfg.FsType
		}
		vi = &VolumeInfo{Name: name, Path: volPath, FsType: fsType}
		d.vols[name] = vi
	}
	vi.attached++
	d.mu.Unlock()

	rm := d.ResticManager(name)

	ctx := context.Background()
	lock, err := d.locker.Acquire(ctx, name)
	if err != nil {
		applog.Errorf("mount_lock_acquire_failed", err, "volume=%s", name)
	} else {
		vi.Lock = lock

		if vi.FsType != "" {
			snapID, err := rm.FindRestorePointByName(name)
			if err != nil {
				applog.Errorf("check_restore_point_failed", err, "volume=%s", name)
			} else if snapID != "" {
				valid, verr := vi.Lock.IsValid()
				switch {
				case verr != nil:
					applog.Errorf("lock_check_failed", verr, "volume=%s", name)
				case !valid:
					applog.Warnf("lock_expired_skipping_restore", "volume=%s", name)
				default:
					applog.Infof("restore_point_found", "volume=%s snapshot=%s", name, snapID)

					applog.Infof("rollback_backup", "volume=%s", name)
					if err := rm.Backup(volPath, constants.BackupTagRollback); err != nil {
						applog.Errorf("rollback_backup_failed", err, "volume=%s", name)
					}

					snapDir := filepath.Join(d.root, constants.SnapshotsDir)
					preSnap, snapErr := snapshot.Create(volPath, snapDir, name+constants.PreRestoreSuffix)
					if snapErr != nil {
						applog.Errorf("pre_restore_snapshot_failed", snapErr, "volume=%s", name)
					}
					if err := rm.RestoreSnapshot(snapID, volPath); err != nil {
						applog.Errorf("restore_failed", err, "volume=%s snapshot=%s", name, snapID)
					} else {
						applog.Infof("restore_complete_removing_point", "volume=%s", name)
						if err := rm.DeleteRestorePoint(name); err != nil {
							applog.Errorf("remove_restore_point_failed", err, "volume=%s", name)
						}
					}
					if preSnap != nil {
						if err := snapshot.Remove(preSnap); err != nil {
							applog.Errorf("cleanup_pre_restore_snapshot_failed", err, "volume=%s", name)
						}
					}
				}
			}
		}

		ctx2, cancel := context.WithCancel(context.Background())
		vi.cancel = cancel
		if vi.attached == 1 {
			d.startHotSchedule(ctx2, name, volPath)
		}
	}

	return &volume.MountResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Unmount(r *volume.UnmountRequest) error {
	name := r.Name
	d.mu.Lock()
	vi, ok := d.vols[name]
	if ok {
		vi.attached--
		if vi.attached <= 0 {
			rm := d.ResticManager(name)
			if err := d.coldBackup(name, vi.Path, vi.FsType, rm); err != nil {
				applog.Errorf("unmount_cold_backup_failed", err, "volume=%s", name)
			}
			if vi.cancel != nil {
				vi.cancel()
			}
		}
	}
	d.mu.Unlock()
	return nil
}

func (d *Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	volPath := volumepath.VolumePath(d.root, r.Name)
	return &volume.PathResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	volPath := volumepath.VolumePath(d.root, r.Name)
	d.mu.Lock()
	vi, ok := d.vols[r.Name]
	d.mu.Unlock()
	state := "unlocked"
	attached := 0
	if ok {
		attached = vi.attached
		if vi.Lock != nil {
			state = "locked"
		}
	}
	status := map[string]any{
		"state":    state,
		"attached": fmt.Sprintf("%d", attached),
	}
	if ok && vi.FsType != "" {
		status["fs_type"] = vi.FsType
	}
	return &volume.GetResponse{Volume: &volume.Volume{Name: r.Name, Mountpoint: volPath, Status: status}}, nil
}

func (d *Driver) List() (*volume.ListResponse, error) {
	names := d.VolumeNames()
	vols := make([]*volume.Volume, 0, len(names))
	for _, name := range names {
		p := volumepath.VolumePath(d.root, name)
		vols = append(vols, &volume.Volume{Name: name, Mountpoint: p})
	}
	return &volume.ListResponse{Volumes: vols}, nil
}

func (d *Driver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "local"}}
}

func (d *Driver) SnapVolumes() map[string]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]string, len(d.vols))
	for name, vi := range d.vols {
		if vi.FsType != "" {
			out[name] = vi.FsType
		}
	}
	return out
}

func (d *Driver) VolumeNames() []string {
	var names []string
	d.collectVolumeNames(filepath.Join(d.root, constants.VolumesDir), "", &names)
	return names
}

func (d *Driver) collectVolumeNames(base, rel string, names *[]string) {
	entries, err := os.ReadDir(filepath.Join(base, rel))
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		child := rel + e.Name()
		childPath := filepath.Join(base, child)
		if _, err := os.Stat(filepath.Join(childPath, constants.VolumeConfigFile)); err == nil {
			*names = append(*names, child)
		} else {
			d.collectVolumeNames(base, child+"/", names)
		}
	}
}

func (d *Driver) writeVolumeConfig(volPath string, cfg *volumeConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(volPath, constants.VolumeConfigFile), data, 0o644)
}

func (d *Driver) readVolumeConfig(volPath string) *volumeConfig {
	data, err := os.ReadFile(filepath.Join(volPath, constants.VolumeConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		applog.Errorf("read_volume_config_failed", err, "path=%s", volPath)
		return nil
	}
	var cfg volumeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		applog.Errorf("parse_volume_config_failed", err, "path=%s", volPath)
		return nil
	}
	return &cfg
}

func (d *Driver) startHotSchedule(ctx context.Context, name, volPath string) {
	rm := d.ResticManager(name)
	hotTicker := time.NewTicker(constants.HotBackupInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				hotTicker.Stop()
				return
			case <-hotTicker.C:
				applog.Infof("hot_backup_start", "volume=%s", name)
				if err := rm.Backup(volPath, constants.BackupTagHot); err != nil {
					applog.Errorf("hot_backup_failed", err, "volume=%s", name)
				}
			}
		}
	}()
}

func (d *Driver) coldBackup(name, volPath, fsType string, rm *restic.Manager) error {
	if fsType == "" {
		return rm.Backup(volPath, constants.BackupTagCold)
	}

	snapDir := filepath.Join(d.root, constants.SnapshotsDir)
	info, err := snapshot.Create(volPath, snapDir, name)
	if err != nil {
		applog.Errorf("snapshot_create_failed", err, "volume=%s falling back to direct backup", name)
		return rm.Backup(volPath, constants.BackupTagCold)
	}

	snapTime := time.Now()
	if fi, err := os.Stat(info.AccessPath); err == nil {
		snapTime = fi.ModTime()
	}

	if err := rm.BackupAt(info.AccessPath, constants.BackupTagCold, snapTime); err != nil {
		return fmt.Errorf("cold backup: %w", err)
	}
	if err := snapshot.Remove(info); err != nil {
		applog.Errorf("remove_snapshot_failed", err, "volume=%s fs=%s", name, fsType)
	}
	return nil
}

func (d *Driver) monitorOrphanedSnapshots(ctx context.Context) {
	ticker := time.NewTicker(constants.OrphanCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.retryOrphanedSnapshots()
		}
	}
}

func (d *Driver) retryOrphanedSnapshots() {
	snapDir := filepath.Join(d.root, constants.SnapshotsDir)
	snaps, err := snapshot.ListOrphaned(snapDir)
	if err != nil {
		applog.Errorf("list_orphaned_snapshots_failed", err, "error=%v", err)
		return
	}
	for _, info := range snaps {
		fi, err := os.Stat(info.AccessPath)
		if err != nil {
			continue
		}
		if time.Since(fi.ModTime()) < constants.OrphanRetryMinAge {
			continue
		}
		if err := snapshot.ResolveType(info); err != nil {
			applog.Errorf("resolve_snapshot_type_failed", err, "path=%s", info.AccessPath)
			continue
		}
		applog.Infof("retry_orphaned_cold_backup", "volume=%s path=%s", info.VolName, info.AccessPath)
		rm := d.ResticManager(info.VolName)
		if err := rm.BackupAt(info.AccessPath, constants.BackupTagCold, fi.ModTime()); err != nil {
			applog.Errorf("orphaned_cold_backup_failed", err, "volume=%s", info.VolName)
			continue
		}
		if err := snapshot.Remove(info); err != nil {
			applog.Errorf("cleanup_snapshot_failed", err, "path=%s", info.AccessPath)
		}
	}
}
