package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/example/blt-volume-manager/locker"
	"github.com/example/blt-volume-manager/restic"
	"github.com/example/blt-volume-manager/snapshot"
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
}

func NewDriver(root string, resticBase string, lockBucket string, s3Endpoint string, s3Region string) *Driver {
	var l locker.Locker
	if lockBucket != "" {
		if sl, err := locker.NewS3Locker(lockBucket, s3Endpoint, s3Region); err == nil {
			l = sl
		} else {
			log.Printf("failed to init s3 locker, falling back to file locker: %v", err)
			l = locker.NewFileLocker(filepath.Join(root, "locks"))
		}
	} else {
		l = locker.NewFileLocker(filepath.Join(root, "locks"))
	}

	zfsParent := ""
	if p, err := snapshot.ParentDataset(root); err == nil {
		zfsParent = p
	}

	drv := &Driver{
		root:       root,
		resticBase: resticBase,
		locker:     l,
		vols:       make(map[string]*VolumeInfo),
		zfsParent:  zfsParent,
	}
	go drv.monitorOrphanedSnapshots(context.Background())
	return drv
}

func (d *Driver) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(d.resticBase + "/restic/" + volName)
}

func (d *Driver) Create(r *volume.CreateRequest) error {
	name := r.Name
	volPath := filepath.Join(d.root, "volumes", name)

	fsType := ""
	if r.Options != nil {
		fsType = d.initFsType(r.Options, name, volPath)
	}
	if fsType == "" {
		if err := os.MkdirAll(volPath, 0755); err != nil {
			return err
		}
	}
	if err := d.writeVolumeConfig(volPath, &volumeConfig{FsType: fsType}); err != nil {
		return fmt.Errorf("write volume config: %w", err)
	}

	ctx := context.Background()
	lock, err := d.locker.Acquire(ctx, name)
	if err == nil { 
		//FIXME: Need to adjust this restore logic to check snapshot tags for restore point
		// Also should it take latest even if it's a hot backup? Maybe send an alert
		restoreMode := "latest"
		if r.Options != nil {
			restoreMode = r.Options["restore"]
		}
		if restoreMode == "" {
			restoreMode = "latest"
		}
		rm := d.ResticManager(name)
		if err := rm.RestoreIfExists(volPath, restoreMode); err != nil {
			log.Printf("restore failed: %v", err)
		}
		lock.Release() // FIXME: This doesn't seem right - should retain lock on success?
	} else {
		log.Printf("create: couldn't acquire lock for %s: %v", name, err)
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
			log.Printf("volume %s: %s requested but parent %s is not on %s", name, candidate, parent, candidate)
			return ""
		}
		switch candidate {
		case "btrfs":
			if err := snapshot.InitBtrfs(volPath); err != nil {
				log.Printf("volume %s: init btrfs subvolume: %v", name, err)
				return ""
			}
			log.Printf("volume %s: initialized as btrfs subvolume", name)
		case "zfs":
			parentDS := d.zfsParent
			if p, ok := opts["zfs-pool"]; ok && p != "" {
				parentDS = p
			}
			if parentDS == "" {
				log.Printf("volume %s: zfs requested but no parent dataset found", name)
				return ""
			}
			if _, err := snapshot.InitZFS(volPath, parentDS); err != nil {
				log.Printf("volume %s: init zfs dataset: %v", name, err)
				return ""
			}
			log.Printf("volume %s: initialized as ZFS dataset under %s", name, parentDS)
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
	d.mu.Unlock()

	volPath := filepath.Join(d.root, "volumes", name)
	cfg := d.readVolumeConfig(volPath)
	fsType := ""
	if cfg != nil {
		fsType = cfg.FsType
	}

	rm := d.ResticManager(name)
	if err := d.coldBackup(name, volPath, fsType, rm); err != nil { //FIXME: Snapshot first and backup snapshot
		log.Printf("final backup before remove failed: %v", err)
	}
	switch fsType {
	case "btrfs":
		log.Printf("btrfs subvolume delete %s", volPath)
		exec.Command("btrfs", "subvolume", "delete", volPath).Run()
	case "zfs":
		if ds, err := snapshot.ParentDataset(volPath); err == nil {
			log.Printf("zfs destroy -r %s", ds)
			exec.Command("zfs", "destroy", "-r", ds).Run()
		}
	default:
		os.RemoveAll(volPath)
	}
	if ok && vi.Lock != nil {
		vi.Lock.Release()
	}
	return nil
}

func (d *Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	name := r.Name
	volPath := filepath.Join(d.root, "volumes", name)
	os.MkdirAll(volPath, 0755)

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
	lock, err := d.locker.Acquire(ctx, name) //FIXME: this should be more of a "check lock"
	if err != nil {
		log.Printf("Mount: failed to acquire lock: %v", err)
	} else {
		vi.Lock = lock

		if vi.FsType != "" {
			snapID, err := rm.FindRestorePoint(volPath)
			if err != nil {
				log.Printf("check restore-point: %v", err)
			} else if snapID != "" {
				valid, verr := vi.Lock.IsValid()
				if verr != nil {
					log.Printf("restore-point: lock check failed: %v", verr)
				} else if !valid {
					log.Printf("restore-point: lock no longer held, skipping restore")
				} else {
					log.Printf("restore-point found for %s (%s)", name, snapID)

					log.Printf("backing up current state (rollback) for %s", name)
					if err := rm.Backup(volPath, "rollback"); err != nil { //TODO: If no mounted containers, filesystem snapshot first then cold backup
						log.Printf("rollback backup failed: %v", err)
					}

					snapDir := filepath.Join(d.root, "snapshots")
					preSnap, snapErr := snapshot.Create(volPath, snapDir, name+"-pre-restore")
					if snapErr != nil {
						log.Printf("pre-restore snapshot: %v", snapErr)
					}
					if err := rm.RestoreSnapshot(snapID, volPath); err != nil {
						log.Printf("restore-point restore failed: %v", err)
					} else {
						log.Printf("restore-point restored, removing tag")
						if err := rm.UntagSnapshot(snapID, "restore-point"); err != nil {
							log.Printf("remove restore-point tag: %v", err)
						}
					}
					if preSnap != nil {
						if err := snapshot.Remove(preSnap); err != nil {
							log.Printf("cleanup pre-restore snapshot: %v", err)
						}
					}
				}
			}
		}

		ctx2, cancel := context.WithCancel(context.Background())
		vi.cancel = cancel
		d.startHotSchedule(ctx2, name, volPath)
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
			if err := d.coldBackup(name, vi.Path, vi.FsType, rm); err != nil { //TODO: hot backup if other containers still have mount, else no cold backup?
				log.Printf("unmount cold backup failed: %v", err)
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
	volPath := filepath.Join(d.root, "volumes", r.Name)
	return &volume.PathResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	volPath := filepath.Join(d.root, "volumes", r.Name)
	d.mu.Lock()
	vi, ok := d.vols[r.Name]
	d.mu.Unlock()
	state := "unlocked"
	attached := 0
	status := map[string]interface{}{
		"state":    state,
		"attached": fmt.Sprintf("%d", attached),
	}
	if ok {
		attached = vi.attached
		if vi.Lock != nil {
			state = "locked"
		}
		if vi.FsType != "" {
			status["fs_type"] = vi.FsType
		}
	}
	return &volume.GetResponse{Volume: &volume.Volume{Name: r.Name, Mountpoint: volPath, Status: status}}, nil
}

func (d *Driver) List() (*volume.ListResponse, error) {
	root := filepath.Join(d.root, "volumes")
	entries, _ := os.ReadDir(root)
	vols := make([]*volume.Volume, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(root, e.Name())
		vols = append(vols, &volume.Volume{Name: e.Name(), Mountpoint: p})
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
	root := filepath.Join(d.root, "volumes")
	entries, _ := os.ReadDir(root)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

func (d *Driver) writeVolumeConfig(volPath string, cfg *volumeConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(volPath, "volume.json"), data, 0644)
}

func (d *Driver) readVolumeConfig(volPath string) *volumeConfig {
	data, err := os.ReadFile(filepath.Join(volPath, "volume.json"))
	if err != nil {
		return nil
	}
	var cfg volumeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func (d *Driver) startHotSchedule(ctx context.Context, name, volPath string) {
	//TODO: should hot backups create filesystem snapshots too?
	rm := d.ResticManager(name)
	hotTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				hotTicker.Stop()
				return
			case <-hotTicker.C:
				log.Printf("hot backup for %s", name)
				if err := rm.Backup(volPath, "hot"); err != nil {
					log.Printf("hot backup error: %v", err)
				}
			}
		}
	}()
}

func (d *Driver) coldBackup(name, volPath, fsType string, rm *restic.Manager) error {
	if fsType == "" {
		return rm.Backup(volPath, "cold")
	}

	snapDir := filepath.Join(d.root, "snapshots")
	info, err := snapshot.Create(volPath, snapDir, name)
	if err != nil {
		log.Printf("cold backup: snapshot create failed (%v), falling back to direct backup", err)
		return rm.Backup(volPath, "cold")
	}

	snapTime := time.Now()
	if fi, err := os.Stat(info.AccessPath); err == nil {
		snapTime = fi.ModTime()
	}

	if err := rm.BackupAt(info.AccessPath, "cold", snapTime); err != nil {
		return fmt.Errorf("cold backup: %w", err)
	}
	if err := snapshot.Remove(info); err != nil {
		log.Printf("remove %s snapshot for %s: %v", fsType, name, err)
	}
	return nil
}

func (d *Driver) monitorOrphanedSnapshots(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
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
	snapDir := filepath.Join(d.root, "snapshots")
	snaps, err := snapshot.ListOrphaned(snapDir)
	if err != nil {
		log.Printf("list snapshots: %v", err)
		return
	}
	for _, info := range snaps {
		fi, err := os.Stat(info.AccessPath)
		if err != nil {
			continue
		}
		if time.Since(fi.ModTime()) < 10*time.Minute {
			continue
		}
		if err := snapshot.ResolveType(info); err != nil {
			log.Printf("resolve snapshot type for %s: %v", info.AccessPath, err)
			continue
		}
		log.Printf("retrying orphaned cold backup for %s (%s)", info.VolName, info.AccessPath)
		rm := d.ResticManager(info.VolName)
		if err := rm.BackupAt(info.AccessPath, "cold", fi.ModTime()); err != nil { //FIXME: Can we assume it's a cold snapshot?
			log.Printf("orphaned cold backup for %s failed: %v", info.VolName, err)
			continue
		}
		if err := snapshot.Remove(info); err != nil {
			log.Printf("cleanup snapshot %s after retry: %v", info.AccessPath, err)
		}
	}
}
