package metadata

import "time"

const (
	BackendS3   = "s3"
	BackendEtcd = "etcd"
)

const (
	OwnerPrefix        = "blt-volume-manager/owners/"
	VolumesPrefix      = "blt-volume-manager/registered-volumes/"
	RestorePointPrefix = "blt-volume-manager/restore-points/"
	VersionPrefix      = "blt-volume-manager/versions/"
)

const (
	DefaultOwnerMaxHoldMins    = 10
	DefaultOwnerTTL            = 24 * time.Hour
	DefaultOwnerAcquireTimeout = 5 * time.Second
)
