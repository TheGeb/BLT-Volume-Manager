package store

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type FakeS3 struct {
	mu   sync.Mutex
	data map[string][]byte
	opts S3StoreOpts

	// optional error injection
	PutObjectErr    error
	ReadObjectErr   error
	DeleteObjectErr error
	ListObjectsErr  error
}

func NewFakeS3(opts S3StoreOpts) *FakeS3 {
	return &FakeS3{
		data: make(map[string][]byte),
		opts: opts,
	}
}

func (f *FakeS3) PutObject(key string, data []byte) error {
	if f.PutObjectErr != nil {
		return f.PutObjectErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = data
	return nil
}

func (f *FakeS3) ReadObject(key string) ([]byte, error) {
	if f.ReadObjectErr != nil {
		return nil, f.ReadObjectErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.data[key]
	if !ok {
		return nil, nil
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (f *FakeS3) DeleteObject(key string) error {
	if f.DeleteObjectErr != nil {
		return f.DeleteObjectErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return nil
}

func (f *FakeS3) ListObjects(prefix string) ([]types.Object, error) {
	if f.ListObjectsErr != nil {
		return nil, f.ListObjectsErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var objects []types.Object
	for k := range f.data {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		key := k
		objects = append(objects, types.Object{
			Key:          &key,
			LastModified: timePtr(time.Now()),
		})
	}
	sort.Slice(objects, func(i, j int) bool {
		return *objects[i].Key < *objects[j].Key
	})
	return objects, nil
}

func (f *FakeS3) ListCommonPrefixes(prefix, delimiter string) ([]string, error) {
	if f.ListObjectsErr != nil {
		return nil, f.ListObjectsErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := make(map[string]bool)
	for k := range f.data {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if idx := strings.Index(rest, delimiter); idx >= 0 {
			seen[prefix+rest[:idx+len(delimiter)]] = true
		}
	}
	var out []string
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func (f *FakeS3) DeleteObjectsWithPrefix(prefix string) error {
	objects, err := f.ListObjects(prefix)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if obj.Key != nil {
			if err := f.DeleteObject(*obj.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *FakeS3) WriteVolumeMarker(name string) error {
	return f.PutObject(f.opts.S3VolumePrefix+name+".json", nil)
}

func (f *FakeS3) DeleteVolumeMarker(name string) error {
	return f.DeleteObject(f.opts.S3VolumePrefix + name + ".json")
}

func (f *FakeS3) ListVolumeMarkers() ([]string, error) {
	return ListVolumeMarkers(f, f.opts.S3VolumePrefix)
}

func (f *FakeS3) DeleteLockObjects() error {
	return f.DeleteObjectsWithPrefix(f.opts.S3LockFolder)
}

func (f *FakeS3) WriteRestorePoint(volumeName string, rp RestorePoint) error {
	return WriteRestorePoint(f, volumeName, rp)
}

func (f *FakeS3) ReadRestorePoint(volumeName string) (*RestorePoint, error) {
	return ReadRestorePoint(f, volumeName)
}

func (f *FakeS3) DeleteRestorePoint(volumeName string) error {
	return DeleteRestorePoint(f, volumeName)
}

func timePtr(t time.Time) *time.Time { return &t }
