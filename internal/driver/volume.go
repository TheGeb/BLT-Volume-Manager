package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	snapshot "github.com/TheGeb/docker-s3-volume-plugin/internal/driver/fs_snapshot"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/docker/go-plugins-helpers/volume"
)

func (d *Driver) Create(r *volume.CreateRequest) error {
	name := r.Name
	volPath := VolumePath(d.root, name)

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
	if d.ownerClient != nil {
		ownerLock, err := d.ownerClient.Lock(ctx, name)
		if err != nil {
			return err
		}
		if rerr := ownerLock.Release(); rerr != nil {
			log.Errorf("release_owner_failed", rerr, "volume=%s", name)
		}
	}

	// Cold backup on create — marks the volume's initial state (v0, v0.0)
	rm := d.ResticManager(name)
	if err := rm.Backup(volPath, "cold", "v0", "v0.0"); err != nil {
		log.Errorf("create_cold_backup_failed", err, "volume=%s", name)
	}

	return nil
}

func (d *Driver) initFsType(opts map[string]string, name, volPath string) string {
	for _, t := range snapshot.RegisteredTypes() {
		candidate := t.String()
		if candidate == "" {
			continue
		}
		v, ok := opts[candidate]
		if !ok || !strings.EqualFold(v, "true") {
			continue
		}
		detected := snapshot.Detect(volPath)
		if detected.String() == candidate {
			return candidate
		}
		parent := filepath.Dir(volPath)
		if snapshot.Detect(parent) != t {
			log.Warnf("volume_fs_mismatch", "volume=%s fs=%s parent=%s", name, candidate, parent)
			return ""
		}
		if err := snapshot.InitFs(volPath, t, opts); err != nil {
			log.Errorf("fs_init_failed", err, "volume=%s fs=%s", name, candidate)
			return ""
		}
		log.Infof("fs_initialized", "volume=%s fs=%s", name, candidate)
		return candidate
	}
	return ""
}

func (d *Driver) Remove(r *volume.RemoveRequest) error {
	name := r.Name
	d.mu.Lock()
	vi, ok := d.vols[name]
	var ownerLock metadata.OwnerLock
	if ok && vi != nil {
		ownerLock = vi.OwnerLock
	}
	d.mu.Unlock()

	volPath := VolumePath(d.root, name)
	cfg := d.readVolumeConfig(volPath)
	fsType := ""
	if cfg != nil {
		fsType = cfg.FsType
	}

	rm := d.ResticManager(name)
	if err := d.coldBackup(name, volPath, fsType, rm); err != nil {
		log.Errorf("final_backup_failed", err, "volume=%s", name)
	}
	if fsType != "" {
		if err := snapshot.DestroyVolume(volPath, snapshot.FromString(fsType)); err != nil {
			log.Errorf("destroy_volume_failed", err, "path=%s fs=%s", volPath, fsType)
		}
	} else {
		if err := os.RemoveAll(volPath); err != nil {
			log.Errorf("remove_volume_dir_failed", err, "path=%s", volPath)
		}
	}
	if ownerLock != nil {
		if err := ownerLock.Release(); err != nil {
			log.Errorf("release_owner_failed", err, "volume=%s", name)
		}
	}
	return nil
}

func (d *Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	name := r.Name
	volPath := VolumePath(d.root, name)
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
	ownerLock, err := d.ownerClient.Lock(ctx, name)
	if err != nil {
		log.Errorf("mount_owner_lock_failed", err, "volume=%s", name)
	} else {
		vi.OwnerLock = ownerLock

		// Cold backup at mount — marks session boundary with new major version
		if vt := d.nextVersionTags(name, true); vt != nil {
			tags := append([]string{"cold"}, vt...)
			if err := rm.Backup(volPath, tags...); err != nil {
				log.Errorf("mount_cold_backup_failed", err, "volume=%s", name)
			}
		}

		if vi.FsType != "" {
			snapID, err := metadata.FindRestorePointByName(d.metadataStore, name)
			if err != nil {
				log.Errorf("check_restore_point_failed", err, "volume=%s", name)
			} else if snapID != "" {
				valid, verr := vi.OwnerLock.IsValid()
				switch {
				case verr != nil:
					log.Errorf("owner_check_failed", verr, "volume=%s", name)
				case !valid:
					log.Warnf("owner_expired_skipping_restore", "volume=%s", name)
				default:
					log.Infof("restore_point_found", "volume=%s snapshot=%s", name, snapID)

					snapDir := filepath.Join(d.root, SnapshotsDir)
					preSnap, snapErr := snapshot.Create(volPath, snapDir, name+snapshot.PreRestoreSuffix)
					if snapErr != nil {
						log.Errorf("pre_restore_snapshot_failed", snapErr, "volume=%s", name)
					}
					if err := rm.RestoreSnapshot(snapID, volPath); err != nil {
						log.Errorf("restore_failed", err, "volume=%s snapshot=%s", name, snapID)
					} else {
						log.Infof("restore_complete_removing_point", "volume=%s", name)
						if err := metadata.DeleteRestorePoint(d.metadataStore, name); err != nil {
							log.Errorf("remove_restore_point_failed", err, "volume=%s", name)
						}
					}
					if preSnap != nil {
						if err := snapshot.Remove(preSnap); err != nil {
							log.Errorf("cleanup_pre_restore_snapshot_failed", err, "volume=%s", name)
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
				log.Errorf("unmount_cold_backup_failed", err, "volume=%s", name)
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
	volPath := VolumePath(d.root, r.Name)
	return &volume.PathResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	volPath := VolumePath(d.root, r.Name)
	d.mu.Lock()
	vi, ok := d.vols[r.Name]
	d.mu.Unlock()
	state := "unclaimed"
	attached := 0
	if ok {
		attached = vi.attached
		if vi.OwnerLock != nil {
			state = "owned"
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
		p := VolumePath(d.root, name)
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
	d.collectVolumeNames(filepath.Join(d.root, VolumesDir), "", &names)
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
		if _, err := os.Stat(filepath.Join(childPath, VolumeConfigFile)); err == nil {
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
	return os.WriteFile(filepath.Join(volPath, VolumeConfigFile), data, 0o644)
}

func (d *Driver) readVolumeConfig(volPath string) *volumeConfig {
	data, err := os.ReadFile(filepath.Join(volPath, VolumeConfigFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		log.Errorf("read_volume_config_failed", err, "path=%s", volPath)
		return nil
	}
	var cfg volumeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Errorf("parse_volume_config_failed", err, "path=%s", volPath)
		return nil
	}
	return &cfg
}
