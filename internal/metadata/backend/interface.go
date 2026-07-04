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
	Key                 *string
	ModificationCounter *int64
}
