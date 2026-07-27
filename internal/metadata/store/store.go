package store

import (
	"context"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

// Backend is the interface for metadata persistence backends (S3, etcd, etc.).
type Backend interface {
	PutObject(ctx context.Context, key string, data []byte) error
	ReadObject(ctx context.Context, key string) ([]byte, error)
	DeleteObject(ctx context.Context, key string) error
	ListObjects(ctx context.Context, prefix string) ([]backend.Entry, error)
	DeleteObjectsWithPrefix(ctx context.Context, prefix string) error
}
