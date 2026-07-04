package backend

import "errors"

var ErrKeyNotFound = errors.New("key not found")

type KeyValueStore interface {
	PutObject(key string, data []byte) error
	ReadObject(key string) ([]byte, error)
	DeleteObject(key string) error
	ListObjects(prefix string) ([]Entry, error)
	DeleteObjectsWithPrefix(prefix string) error
}

type Entry struct {
	Key *string
	// ModificationCounter can represent a timestamp or a revision number, depending on the backend implementation.
	// Note: Timestamp resolution can vary by S3 backend
	ModificationCounter *int64
}
