package metadata

import (
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
	DefaultOwnerMaxHoldMins    = 10
	DefaultOwnerTTL            = 24 * time.Hour
	DefaultOwnerAcquireTimeout = 5 * time.Second
)

func Hostname() string { // FIXME: Awkward placement, where should this live?
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}
