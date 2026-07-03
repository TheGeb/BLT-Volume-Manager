package metadata

import (
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

type MockKeyValueStore struct {
	Objects    []backend.Entry
	ObjectsErr error
	ListFunc   func(prefix string) ([]backend.Entry, error)
}

func (m *MockKeyValueStore) PutObject(string, []byte) error {
	return nil
}

func (m *MockKeyValueStore) ReadObject(string) ([]byte, error) {
	return nil, backend.ErrKeyNotFound
}

func (m *MockKeyValueStore) DeleteObject(string) error {
	return nil
}

func (m *MockKeyValueStore) ListObjects(prefix string) ([]backend.Entry, error) {
	if m.ListFunc != nil {
		return m.ListFunc(prefix)
	}
	return m.Objects, m.ObjectsErr
}

func (m *MockKeyValueStore) ListCommonPrefixes(string, string) ([]string, error) {
	return nil, nil
}

func (m *MockKeyValueStore) DeleteObjectsWithPrefix(string) error {
	return nil
}
