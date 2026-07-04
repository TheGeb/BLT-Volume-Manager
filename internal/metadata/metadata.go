package metadata

import (
	"errors"
	"os"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

var ErrKeyNotFound = backend.ErrKeyNotFound

const (
	BackendS3   = "s3"
	BackendEtcd = "etcd"
)

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

type VersionCounter struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

var ErrRestorePointNotFound = errors.New("restore point not found")

func Hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}
