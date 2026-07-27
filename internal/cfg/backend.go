package cfg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/etcd"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
)

type s3metaBackend struct {
	*s3.Client
}

func (b *s3metaBackend) ReadObject(ctx context.Context, key string) ([]byte, error) {
	data, err := b.Client.ReadObject(ctx, key)
	if errors.Is(err, s3.ErrNotFound) {
		return nil, store.ErrKeyNotFound
	}
	return data, err
}

func OpenMetadataBackend(cfg Config) (store.Backend, error) {
	b, err := openBackend(cfg)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func openBackend(cfg Config) (store.Backend, error) {
	backendType := cfg.MetadataBackend
	if backendType == "" {
		if cfg.S3Bucket != "" {
			backendType = metadata.BackendS3
		} else {
			return nil, fmt.Errorf("no metadata backend configured: set BLT_METADATA_BACKEND, S3_BUCKET, or METADATA_S3_BUCKET")
		}
	}

	switch backendType {
	case metadata.BackendS3:
		if cfg.S3Bucket == "" {
			return nil, fmt.Errorf("S3Bucket required for s3 metadata backend")
		}
		client, err := s3.NewClient(s3.Config{
			Bucket:         cfg.S3Bucket,
			Endpoint:       cfg.S3Endpoint,
			Region:         cfg.S3Region,
			ForcePathStyle: cfg.S3ForcePathStyle,
			Logger:         log.S3Call,
		})
		if err != nil {
			return nil, err
		}
		return &s3metaBackend{Client: client}, nil
	case metadata.BackendEtcd:
		if len(cfg.EtcdEndpoints) == 0 {
			return nil, fmt.Errorf("ETCD_ENDPOINTS required for etcd metadata backend")
		}
		return etcd.NewEtcdClient(etcd.EtcdConfig{
			Endpoints:      cfg.EtcdEndpoints,
			DialTimeout:    5 * time.Second,
			RequestTimeout: 10 * time.Second,
		})
	default:
		return nil, fmt.Errorf("unknown metadata backend %q (supported: s3, etcd)", backendType)
	}
}
