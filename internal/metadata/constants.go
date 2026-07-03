package metadata

import "time"

const (
	BackendS3   = "s3"
	BackendEtcd = "etcd"
)

// TODO: Are these only used for S3? perhaps move them to that package
const (
	OwnerPrefix             = "blt-volume-manager/owners/"
	RegisteredVolumesPrefix = "blt-volume-manager/registered-volumes/"
	RestorePointPrefix      = "blt-volume-manager/restore-points/"
	VersionPrefix           = "blt-volume-manager/versions/"
)

const (
	DefaultOwnerMaxHoldMins    = 10
	DefaultOwnerTTL            = 24 * time.Hour
	DefaultOwnerAcquireTimeout = 5 * time.Second
)

//TODO: Move remaining constants to metadata.go
