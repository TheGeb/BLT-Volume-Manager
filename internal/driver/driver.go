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
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
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
	volumePath        string
	resticPath        string
	ownerMaxMins      int
	vols              map[string]*VolumeInfo
	mu                sync.Mutex
	ownerStore        *store.OwnerStore
	versionStore      *store.VersionStore
	restorePointStore *store.RestorePointStore
}

func New(c appcfg.Config, ctx context.Context) *Driver {
	root := c.DataDir

	var md *metadata.Metadata
	if c.MetadataBackend != "" || c.S3Bucket != "" {
		var err error
		md, err = appcfg.OpenMetadataBackend(c)
		if err != nil {
			log.Errorf("metadata_backend_init_failed", err, "backend=%s", c.MetadataBackend)
		}
	}

	snapshot.InitRoot(root)

	drv := &Driver{
		volumePath: root,
		resticPath: c.ResticBase,
		vols:       make(map[string]*VolumeInfo),
	}
	if md != nil {
		drv.ownerMaxMins = c.OwnerMaxMins
		drv.ownerStore = md.Owners
		drv.versionStore = md.Versions
		drv.restorePointStore = md.RestorePoints
	}

	go drv.monitorOrphanedSnapshots(ctx)
	return drv
}

func (d *Driver) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(d.resticPath + "/restic/" + volName)
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
