package backend

import (
	"errors"
	"time"
)

var ErrKeyNotFound = errors.New("key not found")

type KeyValueStore interface {
	PutObject(key string, data []byte) error
	ReadObject(key string) ([]byte, error)
	DeleteObject(key string) error
	ListObjects(prefix string) ([]Entry, error)
	ListCommonPrefixes(prefix, delimiter string) ([]string, error)
	DeleteObjectsWithPrefix(prefix string) error
}

type Entry struct {
	Key          *string
	LastModified *time.Time
}
