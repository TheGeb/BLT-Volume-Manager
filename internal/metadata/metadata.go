package metadata

import (
	"os"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
)

var ErrKeyNotFound = backend.ErrKeyNotFound

const (
	BackendS3   = "s3"
	BackendEtcd = "etcd"
)

const (
	DefaultOwnerMaxHoldMins    = 10
	DefaultOwnerTTL            = 24 * time.Hour
	DefaultOwnerAcquireTimeout = 5 * time.Second
)

type Metadata struct {
	Volumes       *store.RegisteredVolumeStore
	RestorePoints *store.RestorePointStore
	Versions      *store.VersionStore
	Owners        *store.OwnerStore
	backend       backend.KeyValueStore
}

func NewMetadata(be backend.KeyValueStore) *Metadata {
	return &Metadata{
		Volumes:       store.NewRegisteredVolumeStore(be),
		RestorePoints: store.NewRestorePointStore(be),
		Versions:      store.NewVersionStore(be),
		Owners:        store.NewOwnerStore(be),
		backend:       be,
	}
}

func (m *Metadata) DeleteMetadata(volumeName string) error {
	if err := m.Volumes.Delete(volumeName); err != nil {
		return err
	}

	if err := m.Owners.DeleteForVolume(volumeName); err != nil {
		return err
	}

	if err := m.RestorePoints.Delete(volumeName); err != nil {
		return err
	}

	return m.backend.DeleteObjectsWithPrefix("restic/" + volumeName + "/")
}

func Hostname() string { //FIXME: Awkward placement, where should this live?
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}
