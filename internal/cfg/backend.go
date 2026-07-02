package cfg

import (
	"fmt"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata/etcd"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata/s3"
)

func OpenMetadataBackend(cfg Config) (metadata.ObjectStore, error) {
	backend := cfg.MetadataBackend
	if backend == "" {
		if cfg.S3Bucket != "" {
			backend = metadata.BackendS3
		} else {
			//TODO: Finalize env and log
			return nil, fmt.Errorf("no metadata backend configured: set BLT_METADATA_BACKEND, S3_BUCKET, or METADATA_S3_BUCKET")
		}
	}

	switch backend {
	case metadata.BackendS3:
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3Bucket required for s3 metadata backend")
		}
		return s3.NewClient(s3.Config{
			S3Bucket:       cfg.S3Bucket,
			S3Endpoint:     cfg.S3Endpoint,
			Region:         cfg.S3Region,
			ForcePathStyle: &cfg.S3ForcePathStyle, //TODO: make this more clear
			Logger:         log.S3Call,
		})
	case metadata.BackendEtcd:
		if len(cfg.EtcdEndpoints) == 0 {
			return nil, fmt.Errorf("ETCD_ENDPOINTS required for etcd metadata backend")
		}
		return etcd.NewClient(etcd.Config{
			Endpoints:   cfg.EtcdEndpoints,
			DialTimeout: 5 * time.Second,
		})
	default:
		return nil, fmt.Errorf("unknown metadata backend %q (supported: s3, etcd)", backend)
	}
}
