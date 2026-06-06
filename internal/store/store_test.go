package store

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestS3StoreConfigValidate(t *testing.T) {
	cfg := S3StoreConfig{S3Bucket: ""}
	if err := cfg.validate(); err == nil {
		t.Error("expected error for empty bucket name")
	}

	cfg2 := S3StoreConfig{S3Bucket: "my-bucket"}
	if err := cfg2.validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

type mockS3Store struct {
	listFunc func(prefix string) ([]types.Object, error)
}

func (m *mockS3Store) PutObject(string, []byte) error                               { return nil }
func (m *mockS3Store) ReadObject(string) ([]byte, error)                            { return nil, nil }
func (m *mockS3Store) DeleteObject(string) error                                    { return nil }
func (m *mockS3Store) ListObjects(prefix string) ([]types.Object, error)            { return m.listFunc(prefix) }
func (m *mockS3Store) ListCommonPrefixes(string, string) ([]string, error)          { return nil, nil }
func (m *mockS3Store) DeleteObjectsWithPrefix(string) error                         { return nil }
func (m *mockS3Store) WriteVolumeMarker(string) error                               { return nil }
func (m *mockS3Store) DeleteVolumeMarker(string) error                              { return nil }
func (m *mockS3Store) ListVolumeMarkers() ([]string, error)                         { return nil, nil }
func (m *mockS3Store) DeleteLockObjects() error                                      { return nil }
func (m *mockS3Store) WriteRestorePoint(string, RestorePoint) error                 { return nil }
func (m *mockS3Store) ReadRestorePoint(string) (*RestorePoint, error)               { return nil, nil }
func (m *mockS3Store) DeleteRestorePoint(string) error                              { return nil }

var _ S3Store = (*mockS3Store)(nil)

func TestListVolumeMarkers(t *testing.T) {
	s3 := &mockS3Store{
		listFunc: func(prefix string) ([]types.Object, error) {
			return nil, nil
		},
	}
	names, err := ListVolumeMarkers(s3, "prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestListVolumeMarkersWithVolumes(t *testing.T) {
	s3 := &mockS3Store{
		listFunc: func(prefix string) ([]types.Object, error) {
			return []types.Object{
				{Key: aws.String("prefix/vol-a.json")},
				{Key: aws.String("prefix/vol-b.json")},
				{Key: aws.String("prefix/deep/nested-vol.json")},
			}, nil
		},
	}
	names, err := ListVolumeMarkers(s3, "prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3, got %d: %v", len(names), names)
	}
	expected := map[string]bool{"vol-a": true, "vol-b": true, "deep/nested-vol": true}
	for _, n := range names {
		if !expected[n] {
			t.Errorf("unexpected volume %q", n)
		}
	}
}

func TestListVolumeMarkersSkipsNilKey(t *testing.T) {
	s3 := &mockS3Store{
		listFunc: func(prefix string) ([]types.Object, error) {
			return []types.Object{
				{Key: nil},
				{Key: aws.String("prefix/valid.json")},
			}, nil
		},
	}
	names, err := ListVolumeMarkers(s3, "prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "valid" {
		t.Errorf("expected [valid], got %v", names)
	}
}

func TestListVolumeMarkersSkipsEmptyName(t *testing.T) {
	s3 := &mockS3Store{
		listFunc: func(prefix string) ([]types.Object, error) {
			return []types.Object{
				{Key: aws.String("prefix/.json")},
			}, nil
		},
	}
	names, err := ListVolumeMarkers(s3, "prefix/")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0, got %d", len(names))
	}
}

func TestLockPrefixConstants(t *testing.T) {
	if LockPrefix != "blt-volume-manager/locks/" {
		t.Errorf("unexpected LockPrefix: %q", LockPrefix)
	}
	if VolumePrefix != "blt-volume-manager/registered-volumes/" {
		t.Errorf("unexpected VolumePrefix: %q", VolumePrefix)
	}
}
