package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// LogS3 is a callback for structured S3 call logging. Set by the web server.
var LogS3 func(op, bucket, key string, dur time.Duration, err error)

const (
	LockPrefix         = "blt-volume-manager/locks/"
	VolumePrefix       = "blt-volume-manager/registered-volumes/"
	RestorePointPrefix = "blt-volume-manager/restore-points/"
)

type S3Store interface {
	PutObject(key string, data []byte) error
	ReadObject(key string) ([]byte, error)
	DeleteObject(key string) error
	ListObjects(prefix string) ([]types.Object, error)
	ListCommonPrefixes(prefix, delimiter string) ([]string, error)
	DeleteObjectsWithPrefix(prefix string) error
	WriteVolumeMarker(name string) error
	DeleteVolumeMarker(name string) error
	ListVolumeMarkers() ([]string, error)
	DeleteLockObjects() error
	WriteRestorePoint(vol string, rp RestorePoint) error
	ReadRestorePoint(vol string) (*RestorePoint, error)
	DeleteRestorePoint(vol string) error
}

type S3rw struct {
	s3Client *s3.Client
	opts     S3StoreOpts
}

type S3StoreOpts struct {
	AwsBucketName   string
	AwsLockFolder   string
	AwsVolumePrefix string
	S3Endpoint      string
	Region          string
}

func (opts S3StoreOpts) validate() error {
	if opts.AwsBucketName == "" {
		return fmt.Errorf("AwsBucketName required")
	}
	return nil
}

var _ S3Store = (*S3rw)(nil)

func NewS3Store(opts S3StoreOpts) (S3Store, error) {
	loadOpts := []func(*config.LoadOptions) error{}
	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("error loading s3 config %w", err)
	}

	clientOpts := []func(*s3.Options){}
	if opts.S3Endpoint != "" {
		u, err := url.Parse(opts.S3Endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid S3 endpoint %q: %w", opts.S3Endpoint, err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("S3 endpoint %q has no host", opts.S3Endpoint)
		}
		scheme := u.Scheme
		if scheme == "" {
			scheme = "http" //FIXME default https, ensure that http still works if specified though
		}
		base := scheme + "://" + u.Host
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(base)
			o.UsePathStyle = true
		})
	}
	if pathStyle := os.Getenv("S3_FORCE_PATH_STYLE"); strings.EqualFold(pathStyle, "1") || strings.EqualFold(pathStyle, "true") {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(cfg, clientOpts...)

	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("invalid S3 store options: %w", err)
	}

	return &S3rw{
		s3Client: client,
		opts:     opts,
	}, nil
}

func (s *S3rw) PutObject(key string, data []byte) error {
	start := time.Now()
	_, err := s.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if LogS3 != nil {
		LogS3("PutObject", s.opts.AwsBucketName, key, time.Since(start), err)
	}
	return err
}

func (s *S3rw) ReadObject(key string) ([]byte, error) {
	start := time.Now()
	output, err := s.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    aws.String(key),
	})
	if LogS3 != nil {
		LogS3("GetObject", s.opts.AwsBucketName, key, time.Since(start), err)
	}
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, nil
		}
		return nil, err
	}
	defer output.Body.Close()
	return io.ReadAll(output.Body)
}

func (s *S3rw) DeleteObject(key string) error {
	start := time.Now()
	_, err := s.s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    aws.String(key),
	})
	if LogS3 != nil {
		LogS3("DeleteObject", s.opts.AwsBucketName, key, time.Since(start), err)
	}
	return err
}

func (s *S3rw) ListObjects(prefix string) ([]types.Object, error) {
	start := time.Now()
	var objects []types.Object
	paginator := s3.NewListObjectsV2Paginator(s.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.opts.AwsBucketName),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			if LogS3 != nil {
				LogS3("ListObjectsV2", s.opts.AwsBucketName, prefix, time.Since(start), err)
			}
			return nil, err
		}
		objects = append(objects, page.Contents...)
	}
	if LogS3 != nil {
		LogS3("ListObjectsV2", s.opts.AwsBucketName, prefix, time.Since(start), nil)
	}
	return objects, nil
}

func (s *S3rw) ListCommonPrefixes(prefix, delimiter string) ([]string, error) {
	start := time.Now()
	var prefixes []string
	paginator := s3.NewListObjectsV2Paginator(s.s3Client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.opts.AwsBucketName),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String(delimiter),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			if LogS3 != nil {
				LogS3("ListObjectsV2", s.opts.AwsBucketName, prefix, time.Since(start), err)
			}
			return nil, err
		}
		for _, cp := range page.CommonPrefixes {
			if cp.Prefix != nil {
				prefixes = append(prefixes, *cp.Prefix)
			}
		}
	}
	if LogS3 != nil {
		LogS3("ListObjectsV2", s.opts.AwsBucketName, prefix, time.Since(start), nil)
	}
	return prefixes, nil
}

func (s *S3rw) DeleteObjectsWithPrefix(prefix string) error {
	objects, err := s.ListObjects(prefix)
	if err != nil {
		return fmt.Errorf("listing objects (prefix=%s): %w", prefix, err)
	}
	if len(objects) == 0 {
		return nil
	}
	idents := make([]types.ObjectIdentifier, 0, len(objects))
	for _, obj := range objects {
		if obj.Key != nil {
			idents = append(idents, types.ObjectIdentifier{Key: obj.Key})
		}
	}
	_, err = s.s3Client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Delete: &types.Delete{Objects: idents},
	})
	if err != nil {
		return fmt.Errorf("batch deleting objects (bucket=%s, prefix=%s): %w", s.opts.AwsBucketName, prefix, err)
	}
	return nil
}

func (s *S3rw) WriteVolumeMarker(name string) error {
	return s.PutObject(s.opts.AwsVolumePrefix+name+".json", nil)
}

func (s *S3rw) DeleteVolumeMarker(name string) error {
	return s.DeleteObject(s.opts.AwsVolumePrefix + name + ".json")
}

func (s *S3rw) ListVolumeMarkers() ([]string, error) {
	objects, err := s.ListObjects(s.opts.AwsVolumePrefix)
	if err != nil {
		return nil, err
	}
	prefix := s.opts.AwsVolumePrefix
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

func (s *S3rw) DeleteLockObjects() error {
	return s.DeleteObjectsWithPrefix(s.opts.AwsLockFolder)
}

func (s *S3rw) WriteRestorePoint(volumeName string, rp RestorePoint) error {
	data, err := json.Marshal(rp)
	if err != nil {
		return fmt.Errorf("marshal restore point: %w", err)
	}
	key := RestorePointPrefix + volumeName + ".json"
	return s.PutObject(key, data)
}

func (s *S3rw) ReadRestorePoint(volumeName string) (*RestorePoint, error) {
	key := RestorePointPrefix + volumeName + ".json"
	data, err := s.ReadObject(key)
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

func (s *S3rw) DeleteRestorePoint(volumeName string) error {
	key := RestorePointPrefix + volumeName + ".json"
	return s.DeleteObject(key)
}
