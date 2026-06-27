package metadata

type MockObjectStore struct {
	Objects    []Object
	ObjectsErr error
	ListFunc   func(prefix string) ([]Object, error)
}

func (m *MockObjectStore) PutObject(string, []byte) error    { return nil }
func (m *MockObjectStore) ReadObject(string) ([]byte, error) { return nil, nil }
func (m *MockObjectStore) DeleteObject(string) error         { return nil }
func (m *MockObjectStore) ListObjects(prefix string) ([]Object, error) {
	if m.ListFunc != nil {
		return m.ListFunc(prefix)
	}
	return m.Objects, m.ObjectsErr
}
func (m *MockObjectStore) ListCommonPrefixes(string, string) ([]string, error) { return nil, nil }
func (m *MockObjectStore) DeleteObjectsWithPrefix(string) error                { return nil }
