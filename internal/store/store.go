package store

import (
	"bytes"
	"context"
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

type S3Client struct {
	s3Client *s3.Client
	opts     S3StoreOpts
}

type S3StoreOpts struct {
	S3Bucket       string
	S3LockFolder   string
	S3VolumePrefix string
	S3Endpoint     string
	Region         string
	Logger         func(op, bucket, key string, dur time.Duration, err error)
}

func (opts S3StoreOpts) validate() error {
	if opts.S3Bucket == "" {
		return fmt.Errorf("S3Bucket required")
	}
	return nil
}

var _ S3Store = (*S3Client)(nil)

func NewS3Store(opts S3StoreOpts) (S3Store, error) {
	if err := opts.validate(); err != nil {
		return nil, fmt.Errorf("invalid S3 store options: %w", err)
	}

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
		ep := opts.S3Endpoint
		if !strings.Contains(ep, "://") {
			ep = "https://" + ep
		}
		u, err := url.Parse(ep)
		if err != nil {
			return nil, fmt.Errorf("invalid S3 endpoint %q: %w", opts.S3Endpoint, err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("S3 endpoint %q has no host", opts.S3Endpoint)
		}
		base := u.Scheme + "://" + u.Host
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(base)
			o.UsePathStyle = true
		})
	}
	if pathStyle := os.Getenv("S3_FORCE_PATH_STYLE"); strings.EqualFold(pathStyle, "1") || strings.EqualFold(pathStyle, "true") {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	} else if opts.S3Endpoint == "" {
		clientOpts = append(clientOpts, func(o *s3.Options) {
			o.UsePathStyle = false
		})
	}

	client := s3.NewFromConfig(cfg, clientOpts...)

	return &S3Client{
		s3Client: client,
		opts:     opts,
	}, nil
}

func (s *S3Client) logS3Call(op, bucket, key string, dur time.Duration, err error) {
	fn := s.opts.Logger
	if fn == nil {
		fn = LogS3
	}
	if fn != nil {
		fn(op, bucket, key, dur, err)
	}
}

func (s *S3Client) PutObject(key string, data []byte) error {
	start := time.Now()
	_, err := s.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.opts.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	s.logS3Call("PutObject", s.opts.S3Bucket, key, time.Since(start), err)
	return err
}

func (s *S3Client) ReadObject(key string) ([]byte, error) {
	start := time.Now()
	output, err := s.s3Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.opts.S3Bucket),
		Key:    aws.String(key),
	})
	s.logS3Call("GetObject", s.opts.S3Bucket, key, time.Since(start), err)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = output.Body.Close() }()
	return io.ReadAll(output.Body)
}

func (s *S3Client) DeleteObject(key string) error {
	start := time.Now()
	_, err := s.s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.opts.S3Bucket),
		Key:    aws.String(key),
	})
	s.logS3Call("DeleteObject", s.opts.S3Bucket, key, time.Since(start), err)
	return err
}

func (s *S3Client) ListObjects(prefix string) ([]types.Object, error) {
	start := time.Now()
	var objects []types.Object
	paginator := s3.NewListObjectsV2Paginator(s.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.opts.S3Bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			s.logS3Call("ListObjectsV2", s.opts.S3Bucket, prefix, time.Since(start), err)
			return nil, err
		}
		objects = append(objects, page.Contents...)
	}
	s.logS3Call("ListObjectsV2", s.opts.S3Bucket, prefix, time.Since(start), nil)
	return objects, nil
}

func (s *S3Client) ListCommonPrefixes(prefix, delimiter string) ([]string, error) {
	start := time.Now()
	var prefixes []string
	paginator := s3.NewListObjectsV2Paginator(s.s3Client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.opts.S3Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String(delimiter),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			s.logS3Call("ListObjectsV2", s.opts.S3Bucket, prefix, time.Since(start), err)
			return nil, err
		}
		for _, cp := range page.CommonPrefixes {
			if cp.Prefix != nil {
				prefixes = append(prefixes, *cp.Prefix)
			}
		}
	}
	s.logS3Call("ListObjectsV2", s.opts.S3Bucket, prefix, time.Since(start), nil)
	return prefixes, nil
}

func (s *S3Client) DeleteObjectsWithPrefix(prefix string) error {
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
	const maxBatch = 1000
	for i := 0; i < len(idents); i += maxBatch {
		end := i + maxBatch
		if end > len(idents) {
			end = len(idents)
		}
		_, err = s.s3Client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
			Bucket: aws.String(s.opts.S3Bucket),
			Delete: &types.Delete{Objects: idents[i:end]},
		})
		if err != nil {
			return fmt.Errorf("batch deleting objects (bucket=%s, prefix=%s): %w", s.opts.S3Bucket, prefix, err)
		}
	}
	return nil
}

func (s *S3Client) WriteVolumeMarker(name string) error {
	return s.PutObject(s.opts.S3VolumePrefix+name+".json", nil)
}

func (s *S3Client) DeleteVolumeMarker(name string) error {
	return s.DeleteObject(s.opts.S3VolumePrefix + name + ".json")
}

func (s *S3Client) ListVolumeMarkers() ([]string, error) {
	return ListVolumeMarkers(s, s.opts.S3VolumePrefix)
}

func (s *S3Client) DeleteLockObjects() error {
	if s.opts.S3LockFolder == "" {
		return nil
	}
	return s.DeleteObjectsWithPrefix(s.opts.S3LockFolder)
}

func (s *S3Client) WriteRestorePoint(volumeName string, rp RestorePoint) error {
	return WriteRestorePoint(s, volumeName, rp)
}

func (s *S3Client) ReadRestorePoint(volumeName string) (*RestorePoint, error) {
	return ReadRestorePoint(s, volumeName)
}

func (s *S3Client) DeleteRestorePoint(volumeName string) error {
	return DeleteRestorePoint(s, volumeName)
}
