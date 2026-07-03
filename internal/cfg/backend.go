package cfg

import (
	"fmt"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

func OpenMetadataBackend(cfg Config) (*metadata.Stores, error) {
	be, err := openBackend(cfg)
	if err != nil {
		return nil, err
	}
	return metadata.NewStores(be), nil
}

func openBackend(cfg Config) (backend.KeyValueStore, error) {
	beType := cfg.MetadataBackend
	if beType == "" {
		if cfg.S3Bucket != "" {
			beType = metadata.BackendS3
		} else {
			return nil, fmt.Errorf("no metadata backend configured: set BLT_METADATA_BACKEND, S3_BUCKET, or METADATA_S3_BUCKET")
		}
	}

	switch beType {
	case metadata.BackendS3:
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3Bucket required for s3 metadata backend")
		}
		return backend.NewS3Client(backend.S3Config{
			S3Bucket:       cfg.S3Bucket,
			S3Endpoint:     cfg.S3Endpoint,
			Region:         cfg.S3Region,
			ForcePathStyle: &cfg.S3ForcePathStyle,
			Logger:         log.S3Call,
		})
	case metadata.BackendEtcd:
		if len(cfg.EtcdEndpoints) == 0 {
			return nil, fmt.Errorf("ETCD_ENDPOINTS required for etcd metadata backend")
		}
		return backend.NewEtcdClient(backend.EtcdConfig{
			Endpoints:   cfg.EtcdEndpoints,
			DialTimeout: 5 * time.Second,
		})
	default:
		return nil, fmt.Errorf("unknown metadata backend %q (supported: s3, etcd)", beType)
	}
}
