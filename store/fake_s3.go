package store

import (
	"encoding/json"
	"fmt"
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
	return f.PutObject(f.opts.AwsVolumePrefix+name+".json", nil)
}

func (f *FakeS3) DeleteVolumeMarker(name string) error {
	return f.DeleteObject(f.opts.AwsVolumePrefix + name + ".json")
}

func (f *FakeS3) ListVolumeMarkers() ([]string, error) {
	objects, err := f.ListObjects(f.opts.AwsVolumePrefix)
	if err != nil {
		return nil, err
	}
	prefix := f.opts.AwsVolumePrefix
	var names []string
	for _, obj := range objects {
		if obj.Key == nil {
			continue
		}
		name := strings.TrimPrefix(*obj.Key, prefix)
		name = strings.TrimSuffix(name, ".json")
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (f *FakeS3) DeleteLockObjects() error {
	return f.DeleteObjectsWithPrefix(f.opts.AwsLockFolder)
}

func (f *FakeS3) WriteRestorePoint(volumeName string, rp RestorePoint) error {
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	key := RestorePointPrefix + volumeName + ".json"
	return f.PutObject(key, data)
}

func (f *FakeS3) ReadRestorePoint(volumeName string) (*RestorePoint, error) {
	key := RestorePointPrefix + volumeName + ".json"
	data, err := f.ReadObject(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var rp RestorePoint
	if err := json.Unmarshal(data, &rp); err != nil {
		return nil, fmt.Errorf("parse restore point: %w", err)
	}
	return &rp, nil
}

func (f *FakeS3) DeleteRestorePoint(volumeName string) error {
	key := RestorePointPrefix + volumeName + ".json"
	return f.DeleteObject(key)
}

func timePtr(t time.Time) *time.Time { return &t }
