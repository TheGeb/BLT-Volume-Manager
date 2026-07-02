package metadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

var (
	ErrKeyNotFound    = errors.New("key not found")
	ErrNotImplemented = errors.New("not implemented")
)

func New(s ObjectStore) *Store {
	return &Store{store: s}
}

func (s *Store) PutObject(key string, data []byte) error     { return s.store.PutObject(key, data) }
func (s *Store) ReadObject(key string) ([]byte, error)       { return s.store.ReadObject(key) }
func (s *Store) DeleteObject(key string) error               { return s.store.DeleteObject(key) }
func (s *Store) ListObjects(prefix string) ([]Object, error) { return s.store.ListObjects(prefix) }
func (s *Store) ListCommonPrefixes(prefix, delimiter string) ([]string, error) {
	return s.store.ListCommonPrefixes(prefix, delimiter)
}

func (s *Store) DeleteObjectsWithPrefix(prefix string) error {
	return s.store.DeleteObjectsWithPrefix(prefix)
}

func Hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}

type objectStoreOwnerClient struct {
	store       ObjectStore
	maxHoldMins int
}

type objectStoreOwnerLock struct {
	store ObjectStore
	myKey string
}

func NewOwnerClient(s ObjectStore, maxHoldMins int) OwnerClient {
	return &objectStoreOwnerClient{store: s, maxHoldMins: maxHoldMins}
}

func (c *objectStoreOwnerClient) Lock(ctx context.Context, name string) (OwnerLock, error) {
	folder := OwnerFolder(name)
	myName := fmt.Sprintf("%s-%d", Hostname(), os.Getpid()) //TODO: Allow environment override for name?
	expiry := time.Now().Add(time.Minute * time.Duration(c.maxHoldMins+2)).Unix() //FIXME: Why max mins + 2?

	myKey, err := AcquireOwnerLock(c.store, folder, myName, expiry)
	if err != nil {
		return nil, &OwnerLockError{Code: OwnerLockHeldByAnother, Msg: err.Error()}
	}

	return &objectStoreOwnerLock{store: c.store, myKey: myKey}, nil
}

func (o *objectStoreOwnerLock) Release() error {
	return o.store.DeleteObject(o.myKey)
}

func (o *objectStoreOwnerLock) IsValid() (bool, error) {
	_, err := o.store.ReadObject(o.myKey)
	if err != nil {
		return false, nil
	}
	return true, nil
}
