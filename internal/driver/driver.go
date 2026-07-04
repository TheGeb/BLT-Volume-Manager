package driver

import (
	"context"
	"sync"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	appcfg "github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"

	_ "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot/btrfs"
	_ "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot/zfs"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type volumeConfig struct {
	FsType string `json:"fs_type"`
}

type VolumeInfo struct {
	Name     string
	Path     string
	LockKey  string
	FsType   string
	attached int
	cancel   context.CancelFunc
}

type Driver struct {
	root              string // TODO: Rename to something like volumePath?
	resticBase        string // TODO: Rename to something like resticPath?
	ownerMaxMins      int
	vols              map[string]*VolumeInfo
	mu                sync.Mutex
	ownerStore        *metadata.OwnerStore
	versionStore      *metadata.VersionStore
	restorePointStore *metadata.RestorePointStore
}

func New(c appcfg.Config, ctx context.Context) *Driver {
	root := c.DataDir

	var stores *metadata.Stores
	if c.MetadataBackend != "" || c.S3Bucket != "" {
		var err error
		stores, err = appcfg.OpenMetadataBackend(c)
		if err != nil {
			// TODO: Throw error?
			log.Errorf("metadata_backend_init_failed", err, "backend=%s", c.MetadataBackend)
		}
	}

	snapshot.InitRoot(root)

	drv := &Driver{
		root:       root,
		resticBase: c.ResticBase,
		vols:       make(map[string]*VolumeInfo),
	}
	if stores != nil {
		drv.ownerMaxMins = c.OwnerMaxMins
		drv.ownerStore = stores.Owners
		drv.versionStore = stores.Versions
		drv.restorePointStore = stores.RestorePoints
	}

	go drv.monitorOrphanedSnapshots(ctx)
	return drv
}

func (d *Driver) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(d.resticBase + "/restic/" + volName)
}

func (d *Driver) nextVersionTags(name string, major bool) []string {
	if d.versionStore == nil {
		return nil
	}
	tags, err := d.versionStore.NextTags(name, major)
	if err != nil {
		log.Errorf("version_counter_failed", err, "volume=%s", name)
		return nil
	}
	return tags
}
