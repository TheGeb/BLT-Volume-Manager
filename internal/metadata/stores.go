package metadata

import "github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"

type Stores struct {
	Volumes       *VolumeStore
	RestorePoints *RestorePointStore
	Versions      *VersionStore
	Owners        *OwnerStore

	be backend.KeyValueStore
}

func NewStores(be backend.KeyValueStore) *Stores {
	return &Stores{
		Volumes:       NewVolumeStore(be),
		RestorePoints: NewRestorePointStore(be),
		Versions:      NewVersionStore(be),
		Owners:        NewOwnerStore(be),
		be:            be,
	}
}
