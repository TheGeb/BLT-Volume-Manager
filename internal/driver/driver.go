package driver

import (
	"context"
	"sync"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
	appcfg "github.com/TheGeb/docker-s3-volume-plugin/internal/cfg"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver/snapshot"

	// register btrfs/zfs snapshotters via init()
	_ "github.com/TheGeb/docker-s3-volume-plugin/internal/driver/snapshot/btrfs"
	_ "github.com/TheGeb/docker-s3-volume-plugin/internal/driver/snapshot/zfs"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
)

type volumeConfig struct {
	FsType string `json:"fs_type"`
}

type VolumeInfo struct {
	Name      string
	Path      string
	OwnerLock metadata.OwnerLock
	FsType    string
	attached  int
	cancel    context.CancelFunc
}

type Driver struct {
	root          string
	resticBase    string
	ownerClient   metadata.OwnerClient
	vols          map[string]*VolumeInfo
	mu            sync.Mutex
	metadataStore *metadata.Store
}

func New(c appcfg.Config, ctx context.Context) *Driver {
	root := c.DataDir

	var objectStore metadata.ObjectStore
	if c.MetadataBackend != "" || c.S3Bucket != "" {
		var err error
		objectStore, err = appcfg.OpenMetadataBackend(c)
		if err != nil {
			app.Errorf("metadata_backend_init_failed", err, "backend=%s", c.MetadataBackend)
		}
	}

	var l metadata.OwnerClient
	if objectStore != nil {
		l = metadata.NewOwnerClient(objectStore, c.OwnerMaxMins)
	}

	snapshot.InitRoot(root)

	drv := &Driver{
		root:        root,
		resticBase:  c.ResticBase,
		ownerClient: l,
		vols:        make(map[string]*VolumeInfo),
	}

	if objectStore != nil {
		drv.metadataStore = metadata.New(objectStore)
	}

	go drv.monitorOrphanedSnapshots(ctx)
	return drv
}

func (d *Driver) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(d.resticBase + "/restic/" + volName)
}

func (d *Driver) nextVersionTags(name string, major bool) []string {
	if d.metadataStore == nil {
		return nil
	}
	tags, err := d.metadataStore.NextVersionTags(name, major)
	if err != nil {
		app.Errorf("version_counter_failed", err, "volume=%s", name)
		return nil
	}
	return tags
}
