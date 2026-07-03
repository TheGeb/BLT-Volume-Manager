package metadata

import (
	backendpkg "github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

type Stores struct {
	Volumes       *VolumeStore
	RestorePoints *RestorePointStore
	Versions      *VersionStore
	Owners        *OwnerStore

	backend backendpkg.KeyValueStore
}

func NewStores(backend backendpkg.KeyValueStore) *Stores {
	return &Stores{
		Volumes:       NewVolumeStore(backend),
		RestorePoints: NewRestorePointStore(backend),
		Versions:      NewVersionStore(backend),
		Owners:        NewOwnerStore(backend),
		backend:       backend,
	}
}
