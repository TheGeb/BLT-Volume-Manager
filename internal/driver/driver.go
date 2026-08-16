package driver

import (
	"context"
	"sync"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	appcfg "github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"

	_ "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot/btrfs"
	_ "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot/zfs"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type mountState int

const (
	mountStateIdle mountState = iota
	mountStateAcquiring
	mountStateReady
)

type volMountState struct {
	state      mountState
	done       chan struct{}
	acquireErr error
}

type volumeConfig struct {
	FsType string `json:"fs_type"`
	// LockKey is the persisted owner-lock key (lock-on-creation mode).
	LockKey string `json:"lock_key,omitempty"`
	// LockBackend is the metadata backend ("s3" or "etcd") that wrote LockKey,
	// so a persisted key is only resumed against the backend that understands
	// its format.
	LockBackend string `json:"lock_backend,omitempty"`
	// LockTTLMins is the lock TTL in minutes stamped at volume creation from
	// the init_lock_ttl_mins driver option (defaulting to OWNER_MAX_MINS). It
	// is immutable after creation. 0 means "no per-volume override".
	LockTTLMins int `json:"lock_ttl_mins,omitempty"`
}

type VolumeInfo struct {
	Name        string
	Path        string
	LockKey     string
	FsType      string
	LockTTLMins int
	attached    int
	cancel      context.CancelFunc
}

// Driver implements the Docker volume plugin interface, managing volume lifecycle, backups, and metadata.
type Driver struct {
	volumePath   string
	resticPath   string
	ownerMaxMins int
	lockMode     appcfg.LockMode
	// backendKind records which metadata backend ("s3" or "etcd") the owner
	// locks are stored in; "" when no metadata backend is configured. It is
	// persisted alongside a lock key so a lock is only resumed/validated
	// against the backend that actually wrote it (see api.go Mount).
	backendKind       string
	vols              map[string]*VolumeInfo
	mu                sync.Mutex
	ownerStore        *store.OwnerStore
	versionStore      *store.VersionStore
	restorePointStore *store.RestorePointStore
	mountStates       map[string]*volMountState
	mountMu           sync.Mutex
}

func New(c appcfg.Config, ctx context.Context) *Driver {
	root := c.DataDir

	var b store.MetadataStore
	if c.MetadataBackend != "" || c.S3Bucket != "" {
		var err error
		b, err = appcfg.OpenMetadataBackend(c)
		if err != nil {
			log.Errorf("metadata_backend_init_failed", err, "backend=%s", c.MetadataBackend)
		}
	}

	snapshot.InitRoot(root)

	drv := &Driver{
		volumePath:  root,
		resticPath:  c.ResticBase,
		lockMode:    c.LockMode,
		backendKind: backendKindOf(c),
		vols:        make(map[string]*VolumeInfo),
		mountStates: make(map[string]*volMountState),
	}
	if drv.lockMode == "" {
		drv.lockMode = appcfg.LockModeCreate
	}
	if b != nil {
		drv.ownerMaxMins = c.OwnerMaxMins
		drv.ownerStore = store.NewOwnerStore(b)
		drv.versionStore = store.NewVersionStore(b)
		drv.restorePointStore = store.NewRestorePointStore(b)
	}

	go drv.monitorOrphanedSnapshots(ctx)
	return drv
}

func (d *Driver) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(d.resticPath + "/restic/" + volName)
}

func (d *Driver) nextVersionTags(ctx context.Context, name string, major bool) []string {
	if d.versionStore == nil {
		return nil
	}
	tags, err := d.versionStore.NextTags(ctx, name, major)
	if err != nil {
		log.Errorf("version_counter_failed", err, "volume=%s", name)
		return nil
	}
	return tags
}
