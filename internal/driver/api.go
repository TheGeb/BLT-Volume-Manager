package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	appcfg "github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/docker/go-plugins-helpers/volume"
)

func logReq(obj any) {
	b, _ := json.Marshal(obj)
	log.Debugf("driver_request", "%s", string(b))
}

func logResp(obj any, err error) {
	msg := ""
	if obj != nil {
		b, _ := json.Marshal(obj)
		msg = string(b)
	}
	if err != nil {
		if msg != "" {
			msg += " "
		}
		msg += fmt.Sprintf("error=%v", err)
	}
	if msg != "" {
		log.Debugf("driver_response", "%s", msg)
	}
}

func (d *Driver) Create(r *volume.CreateRequest) (err error) {
	name := r.Name
	logReq(r)
	defer func() { logResp(nil, err) }()
	volPath := VolumePath(d.volumePath, name)

	fsType := ""
	if r.Options != nil {
		fsType = d.initFsType(r.Options, name, volPath)
	}
	if fsType == "" {
		if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
			return err
		}
	}

	cfg := &volumeConfig{FsType: fsType, LockTTLMins: d.optionTTLMins(r.Options)}
	if d.ownerStore != nil && d.lockMode == appcfg.LockModeCreate {
		// Lock on creation: hold a permanent lock until the volume is
		// removed, and persist it so it survives a daemon restart.
		lockKey, err := d.ownerStore.LockVolume(context.Background(), name, ownerName(), 0)
		if err != nil {
			// Docker may re-create an existing volume. If the lock is already
			// held by this host, treat the duplicate create as a success and
			// reuse the persisted lock key.
			if errors.Is(err, store.ErrLockConflict) {
				owned, ferr := d.ownerStore.FindForVolume(context.Background(), name)
				if ferr == nil && owned.Owner == ownerName() {
					if existing := d.readVolumeConfig(volPath); existing != nil && existing.LockKey != "" {
						cfg.LockKey = existing.LockKey
						cfg.LockBackend = d.backendKind
					}
				} else {
					return err
				}
			} else {
				return err
			}
		} else {
			cfg.LockKey = lockKey
			cfg.LockBackend = d.backendKind
		}
	}
	if err := d.writeVolumeConfig(volPath, cfg); err != nil {
		if cfg.LockKey != "" {
			_ = d.ownerStore.ReleaseLock(context.Background(), cfg.LockKey)
		}
		return fmt.Errorf("write volume config: %w", err)
	}

	if d.ownerStore != nil && d.lockMode != appcfg.LockModeCreate {
		myName := ownerName()
		expiry := d.lockExpiry(cfg.LockTTLMins)
		lockKey, err := d.ownerStore.LockVolume(context.Background(), name, myName, expiry)
		if err != nil {
			return err
		}
		if rerr := d.ownerStore.ReleaseLock(context.Background(), lockKey); rerr != nil {
			log.Errorf("release_owner_failed", rerr, "volume=%s", name)
		}
	}

	// Cold backup on create - marks the volume's initial state (v0, v0.0)
	rm := d.ResticManager(name)
	if err := rm.Backup(context.Background(), volPath, "cold", "v0", "v0.0"); err != nil {
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
		v, ok := resolveInitOpt(opts, candidate, candidate)
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
		zfsPool, _ := resolveInitOpt(opts, "zfs-pool", "zfs-pool")
		fsOpts := snapshot.FsOptions{ZfsPool: zfsPool}
		if err := snapshot.InitFs(volPath, t, fsOpts); err != nil {
			log.Errorf("fs_init_failed", err, "volume=%s fs=%s", name, candidate)
			return ""
		}
		log.Infof("fs_initialized", "volume=%s fs=%s", name, candidate)
		return candidate
	}
	return ""
}

// backendKindOf returns the metadata backend kind for a config, mirroring
// openBackend's inference: etcd when configured, else s3 whenever a metadata
// backend (or just an S3 bucket) exists.
func backendKindOf(c appcfg.Config) string {
	if c.MetadataBackend == "etcd" {
		return "etcd"
	}
	if c.MetadataBackend != "" || c.S3Bucket != "" {
		return "s3"
	}
	return ""
}

// resolveInitOpt returns the value for an init-time driver option given at
// volume creation. Init-time options are stamped into the volume config at
// Create and ignored afterwards, so changing them later (e.g. in compose) does
// not affect an existing volume. The cannonically-prefixed form wins; the
// unprefixed legacy spelling is still accepted.
func resolveInitOpt(opts map[string]string, name, legacy string) (string, bool) {
	if v, ok := opts["init_"+name]; ok {
		return v, true
	}
	if legacy != "" {
		if v, ok := opts[legacy]; ok {
			return v, true
		}
	}
	return "", false
}

// optionTTLMins resolves the lock TTL (minutes) from init_lock_ttl_mins,
// defaulting to the driver-wide OWNER_MAX_MINS if not set or invalid.
func (d *Driver) optionTTLMins(opts map[string]string) int {
	t := 0
	if opts != nil {
		if v, ok := resolveInitOpt(opts, "lock_ttl_mins", ""); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				t = n
			}
		}
	}
	if t <= 0 {
		t = d.ownerMaxMins
	}
	if t <= 0 {
		t = 5
	}
	return t
}

// lockExpiry returns the absolute owner-lock expiry for a volume, adding a
// two-minute safety margin so a mount's lock outlives the container that holds
// it. A per-volume TTL (init_lock_ttl_mins) overrides the driver default.
func (d *Driver) lockExpiry(ttlMins int) int64 {
	if ttlMins <= 0 {
		ttlMins = d.ownerMaxMins
	}
	if ttlMins <= 0 {
		ttlMins = 5
	}
	return time.Now().Add(time.Minute * time.Duration(ttlMins+2)).Unix()
}

// renewalInterval returns how often a mount-mode owner lock is re-stamped,
// in seconds. It is always strictly less than half the lock's expiry window so
// the lock never lapses during normal operation - expiry is only reached when
// the owning daemon stops renewing (an outage). Capped at two minutes.
func (d *Driver) renewalInterval(ttlMins int) time.Duration {
	if ttlMins <= 0 {
		ttlMins = d.ownerMaxMins
	}
	if ttlMins <= 0 {
		ttlMins = 5
	}
	expirySec := (ttlMins + 2) * 60
	r := expirySec/2 - 15
	if r > 120 {
		r = 120
	}
	if r < 30 {
		r = 30
	}
	return time.Duration(r) * time.Second
}

func (d *Driver) Remove(r *volume.RemoveRequest) (err error) {
	name := r.Name
	logReq(r)
	defer func() { logResp(nil, err) }()
	d.mu.Lock()
	vi, ok := d.vols[name]
	var lockKey string
	if ok && vi != nil {
		lockKey = vi.LockKey
		if vi.cancel != nil {
			vi.cancel()
			vi.cancel = nil
		}
	}
	d.mu.Unlock()

	volPath := VolumePath(d.volumePath, name)
	if lockKey == "" && d.lockMode == appcfg.LockModeCreate {
		// The lock may have been persisted to config by Create or a previous
		// Mount (e.g. after a daemon restart when d.vols is empty). Only
		// release it if it was written by the backend we are currently using.
		if cfg := d.readVolumeConfig(volPath); cfg != nil && cfg.LockBackend == d.backendKind {
			lockKey = cfg.LockKey
		}
	}
	cfg := d.readVolumeConfig(volPath)
	fsType := ""
	if cfg != nil {
		fsType = cfg.FsType
	}

	rm := d.ResticManager(name)
	if err := d.coldBackup(context.Background(), name, volPath, fsType, rm); err != nil {
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
	if lockKey != "" {
		if err := d.ownerStore.ReleaseLock(context.Background(), lockKey); err != nil {
			log.Errorf("release_owner_failed", err, "volume=%s", name)
		}
	}
	return nil
}

func (d *Driver) Mount(r *volume.MountRequest) (res *volume.MountResponse, err error) {
	name := r.Name
	logReq(r)
	defer func() { logResp(res, err) }()
	volPath := VolumePath(d.volumePath, name)
	if err := os.MkdirAll(volPath, app.DefaultDirPerm); err != nil {
		return nil, fmt.Errorf("create volume dir: %w", err)
	}

	d.mu.Lock()
	vi, ok := d.vols[name]
	if !ok {
		cfg := d.readVolumeConfig(volPath)
		fsType := ""
		var ttl int
		if cfg != nil {
			fsType = cfg.FsType
			ttl = cfg.LockTTLMins
		}
		vi = &VolumeInfo{Name: name, Path: volPath, FsType: fsType, LockTTLMins: ttl}
		if d.lockMode == appcfg.LockModeCreate && cfg != nil {
			// Only resume a persisted lock written by the backend we are
			// currently using; its key format (S3 proposal path vs. etcd
			// "<vol>/lock") is backend-specific, and a key from another
			// backend cannot be validated here. It is re-acquired below.
			if cfg.LockBackend == d.backendKind {
				vi.LockKey = cfg.LockKey
			}
		}
		d.vols[name] = vi
	}
	vi.attached++
	needsLock := vi.LockKey == ""
	// In lock-on-creation mode the permanent lock is held from create to
	// remove, so a mount does not re-acquire it. It still runs the per-mount
	// setup (cold backup, restore point, hot schedule) - the schedule is not
	// running between mount cycles because Unmount cancels it on full detach,
	// so a fresh mount restarts it via vi.cancel == nil.
	needsMountSetup := d.lockMode == appcfg.LockModeCreate && vi.cancel == nil
	d.mu.Unlock()

	// A lock carried over from the volume config (lock-on-creation) may have
	// been released or expired since it was persisted; fall back to acquiring
	// a fresh one. This also re-acquires expired mount-mode locks on remount.
	if !needsLock && d.ownerStore != nil {
		valid, verr := d.ownerStore.LockIsValid(context.Background(), vi.LockKey)
		if verr != nil {
			log.Errorf("mount_owner_check_failed", verr, "volume=%s", name)
			valid = false
		}
		if !valid {
			d.mu.Lock()
			vi.LockKey = ""
			d.mu.Unlock()
			needsLock = true
		}
	}

	needsLockOrSetup := needsLock || needsMountSetup
	rm := d.ResticManager(name)

	if needsLockOrSetup {
		d.mountMu.Lock()
		ms, exists := d.mountStates[name]
		if exists && ms.state == mountStateAcquiring {
			d.mountMu.Unlock()
			<-ms.done
			if ms.acquireErr != nil {
				d.mu.Lock()
				vi.attached--
				d.mu.Unlock()
				return nil, ms.acquireErr
			}
			return &volume.MountResponse{Mountpoint: volPath}, nil
		}
		if exists && ms.state == mountStateReady && ms.acquireErr == nil {
			// A previous mount already acquired the lock while this goroutine
			// was waiting to take mountMu (we read needsLock before the leader
			// set LockKey). Treat this mount as a follower: nothing left to do.
			d.mountMu.Unlock()
			return &volume.MountResponse{Mountpoint: volPath}, nil
		}
		ms = &volMountState{state: mountStateAcquiring, done: make(chan struct{})}
		d.mountStates[name] = ms
		d.mountMu.Unlock()

		if needsLock {
			lockExpiry := d.lockExpiry(vi.LockTTLMins)
			if d.lockMode == appcfg.LockModeCreate {
				// Create-mode volumes hold a permanent lock from create to
				// remove. A re-acquired lock (the persisted key was lost or
				// expired) must be permanent too: renewal is a no-op in this
				// mode, so a time-limited lock would silently lapse mid-mount.
				lockExpiry = 0
			}
			lockKey, lockErr := d.ownerStore.CheckAndUpdateLock(context.Background(), name, ownerName(), lockExpiry)
			if lockErr != nil {
				d.mu.Lock()
				vi.attached--
				d.mu.Unlock()

				d.mountMu.Lock()
				ms.acquireErr = lockErr
				ms.state = mountStateReady
				close(ms.done)
				d.mountMu.Unlock()

				log.Errorf("mount_owner_lock_failed", lockErr, "volume=%s", name)
				return nil, fmt.Errorf("mount owner lock: %w", lockErr)
			}

			d.mu.Lock()
			vi.LockKey = lockKey
			d.mu.Unlock()

			if d.lockMode == appcfg.LockModeCreate {
				if cerr := d.persistLockKey(volPath, lockKey); cerr != nil {
					log.Errorf("persist_lock_key_failed", cerr, "volume=%s", name)
				}
			}
		}

		if vt := d.nextVersionTags(context.Background(), name, true); vt != nil {
			if err := rm.Backup(context.Background(), volPath, restic.WithTags("cold", vt...)...); err != nil {
				log.Errorf("mount_cold_backup_failed", err, "volume=%s", name)
			}
		}

		if vi.FsType != "" && d.restorePointStore != nil {
			snapID, err := d.restorePointStore.FindByName(context.Background(), name)
			if err != nil {
				log.Errorf("check_restore_point_failed", err, "volume=%s", name)
			} else if snapID != "" {
				d.mu.Lock()
				lk := vi.LockKey
				d.mu.Unlock()
				valid, verr := d.ownerStore.LockIsValid(context.Background(), lk)
				switch {
				case verr != nil:
					log.Errorf("owner_check_failed", verr, "volume=%s", name)
				case !valid:
					log.Warnf("owner_expired_skipping_restore", "volume=%s", name)
				default:
					log.Infof("restore_point_found", "volume=%s snapshot=%s", name, snapID)

					snapDir := filepath.Join(d.volumePath, SnapshotsDir)
					preSnap, snapErr := snapshot.Create(volPath, snapDir, name+snapshot.PreRestoreSuffix)
					if snapErr != nil {
						log.Errorf("pre_restore_snapshot_failed", snapErr, "volume=%s", name)
					}
					if err := rm.RestoreSnapshot(context.Background(), snapID, volPath); err != nil {
						log.Errorf("restore_failed", err, "volume=%s snapshot=%s", name, snapID)
					} else {
						log.Infof("restore_complete_removing_point", "volume=%s", name)
						if err := d.restorePointStore.Delete(context.Background(), name); err != nil {
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

		var ctx2 context.Context
		var cancel context.CancelFunc
		d.mu.Lock()
		if vi.cancel == nil {
			ctx2, cancel = context.WithCancel(context.Background())
			vi.cancel = cancel
		}
		startSchedule := vi.cancel != nil
		d.mu.Unlock()

		if startSchedule {
			d.startHotSchedule(ctx2, name, volPath)
			// Renew the mount-mode owner lock while the volume is attached so its
			// expiry only lapses on an outage (renewal stops), never mid-run.
			d.startLockRenewal(ctx2, name, vi.LockTTLMins)
		}

		d.mountMu.Lock()
		ms.state = mountStateReady
		close(ms.done)
		d.mountMu.Unlock()
	}

	return &volume.MountResponse{Mountpoint: volPath}, nil
}

func (d *Driver) Unmount(r *volume.UnmountRequest) (err error) {
	name := r.Name
	logReq(r)
	defer func() { logResp(nil, err) }()
	d.mu.Lock()
	vi, ok := d.vols[name]
	var releaseKey string
	if ok {
		vi.attached--
		if vi.attached <= 0 {
			rm := d.ResticManager(name)
			if err := d.coldBackup(context.Background(), name, vi.Path, vi.FsType, rm); err != nil {
				log.Errorf("unmount_cold_backup_failed", err, "volume=%s", name)
			}
			if vi.cancel != nil {
				vi.cancel()
				vi.cancel = nil
			}
			// Graceful full detach: release the mount-mode lock instead of
			// leaving it to expire. TTL expiry is reserved for outages (the
			// daemon died without unmounting). Permanent (lock-on-creation)
			// locks are held until Remove, not released here.
			if d.lockMode != appcfg.LockModeCreate && vi.LockKey != "" && d.ownerStore != nil {
				releaseKey = vi.LockKey
				vi.LockKey = ""
			}
			d.mountMu.Lock()
			delete(d.mountStates, name)
			d.mountMu.Unlock()
		}
	}
	d.mu.Unlock()

	if releaseKey != "" {
		if err := d.ownerStore.ReleaseLock(context.Background(), releaseKey); err != nil {
			log.Errorf("unmount_release_owner_failed", err, "volume=%s", name)
		}
	}
	return nil
}

func (d *Driver) Path(r *volume.PathRequest) (res *volume.PathResponse, err error) {
	logReq(r)
	defer func() { logResp(res, err) }()
	volPath := VolumePath(d.volumePath, r.Name)
	return &volume.PathResponse{Mountpoint: volPath}, nil
}

type VolumeStatus struct {
	State    string `json:"state"`
	Attached string `json:"attached"`
	FsType   string `json:"fs_type,omitempty"`
}

func (d *Driver) Get(r *volume.GetRequest) (res *volume.GetResponse, err error) {
	logReq(r)
	defer func() { logResp(res, err) }()
	volPath := VolumePath(d.volumePath, r.Name)
	d.mu.Lock()
	vi, ok := d.vols[r.Name]
	lockKey := ""
	if ok {
		lockKey = vi.LockKey
	}
	d.mu.Unlock()
	if lockKey == "" && d.lockMode == appcfg.LockModeCreate {
		if cfg := d.readVolumeConfig(volPath); cfg != nil {
			lockKey = cfg.LockKey
		}
	}
	state := "unclaimed"
	attached := 0
	if ok {
		attached = vi.attached
	}
	if lockKey != "" {
		state = "owned"
	}
	statusMap := map[string]any{
		"state":    state,
		"attached": fmt.Sprintf("%d", attached),
	}
	if ok && vi.FsType != "" {
		statusMap["fs_type"] = vi.FsType
	}
	return &volume.GetResponse{Volume: &volume.Volume{Name: r.Name, Mountpoint: volPath, Status: statusMap}}, nil
}

func (d *Driver) List() (res *volume.ListResponse, err error) {
	defer func() { logResp(res, err) }()
	names := d.VolumeNames()
	vols := make([]*volume.Volume, 0, len(names))
	for _, name := range names {
		p := VolumePath(d.volumePath, name)
		vols = append(vols, &volume.Volume{Name: name, Mountpoint: p})
	}
	return &volume.ListResponse{Volumes: vols}, nil
}

func (d *Driver) Capabilities() (res *volume.CapabilitiesResponse) {
	defer func() { logResp(res, nil) }()
	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "local"}}
}

type SnapVolume struct {
	Name   string
	FsType string
}

func (d *Driver) SnapVolumes() []SnapVolume {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]SnapVolume, 0, len(d.vols))
	for name, vi := range d.vols {
		if vi.FsType != "" {
			out = append(out, SnapVolume{Name: name, FsType: vi.FsType})
		}
	}
	return out
}

func (d *Driver) VolumeNames() []string {
	var names []string
	d.collectVolumeNames(filepath.Join(d.volumePath, VolumesDir), "", &names)
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
		if _, err := os.Stat(filepath.Join(childPath, "volume.json")); err == nil {
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
	return os.WriteFile(filepath.Join(volPath, "volume.json"), data, app.DefaultFilePerm)
}

// persistLockKey updates the volume config so the current lock key survives a
// daemon restart (used in lock-on-creation mode).
func (d *Driver) persistLockKey(volPath, lockKey string) error {
	cfg := d.readVolumeConfig(volPath)
	if cfg == nil {
		cfg = &volumeConfig{}
	}
	cfg.LockKey = lockKey
	cfg.LockBackend = d.backendKind
	return d.writeVolumeConfig(volPath, cfg)
}

func ownerName() string {
	name := os.Getenv("BLT_OWNER_NAME")
	if name == "" {
		name = fmt.Sprintf("%s-%d", metadata.Hostname(), os.Getpid())
	}
	return name
}

func (d *Driver) readVolumeConfig(volPath string) *volumeConfig {
	data, err := os.ReadFile(filepath.Join(volPath, "volume.json"))
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

// startLockRenewal re-stamps a mount-mode owner lock periodically while the
// volume stays attached, so the lock only ever expires when the daemon stops
// renewing (an outage). It stops when the mount context is cancelled (full
// detach or daemon shutdown), which precedes ReleaseLock in Unmount. It is a
// no-op for permanent (lock-on-creation) locks and when no backend is present.
func (d *Driver) startLockRenewal(ctx context.Context, name string, ttlMins int) {
	if d.ownerStore == nil || d.lockMode == appcfg.LockModeCreate {
		return
	}
	interval := d.renewalInterval(ttlMins)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.mu.Lock()
				vi := d.vols[name]
				running := vi != nil && vi.attached > 0
				cur := ""
				if running {
					cur = vi.LockKey
				}
				d.mu.Unlock()
				if !running || cur == "" {
					return
				}
				newKey, err := d.ownerStore.CheckAndUpdateLock(ctx, name, ownerName(), d.lockExpiry(ttlMins))
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					if errors.Is(err, store.ErrLockConflict) {
						log.Warnf("lock_renewal_lost", "volume=%s", name)
						_ = d.ownerStore.ReleaseLock(ctx, cur)
						d.mu.Lock()
						if v := d.vols[name]; v != nil && v.LockKey == cur {
							v.LockKey = ""
						}
						d.mu.Unlock()
						return
					}
					log.Errorf("lock_renewal_failed", err, "volume=%s", name)
					continue
				}
				d.mu.Lock()
				if v := d.vols[name]; v != nil {
					v.LockKey = newKey
				}
				d.mu.Unlock()
				if cur != newKey {
					_ = d.ownerStore.ReleaseLock(ctx, cur)
				}
			}
		}
	}()
}
