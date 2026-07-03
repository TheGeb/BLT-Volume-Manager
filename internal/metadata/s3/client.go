package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Config struct {
	S3Bucket       string
	S3OwnerFolder  string
	S3VolumePrefix string
	S3Endpoint     string
	Region         string
	ForcePathStyle *bool
	Logger         func(op, bucket, key string, dur time.Duration, err error)
}

func (cfg Config) validate() error {
	if cfg.S3Bucket == "" {
		return fmt.Errorf("S3Bucket required")
	}
	return nil
}

type Client struct {
	s3Client *s3sdk.Client
	cfg      Config
}

var _ metadata.ObjectStore = (*Client)(nil)

func NewClient(cfg Config) (metadata.ObjectStore, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid S3 store config: %w", err)
	}

	loadCfg := []func(*config.LoadOptions) error{}
	if cfg.Region != "" {
		loadCfg = append(loadCfg, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), loadCfg...)
	if err != nil {
		return nil, fmt.Errorf("error loading s3 config %w", err)
	}

	clientOpts := []func(*s3sdk.Options){}
	if cfg.S3Endpoint != "" {
		ep := cfg.S3Endpoint
		if !strings.Contains(ep, "://") {
			ep = "https://" + ep
		}
		u, err := url.Parse(ep)
		if err != nil {
			return nil, fmt.Errorf("invalid S3 endpoint %q: %w", cfg.S3Endpoint, err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("S3 endpoint %q has no host", cfg.S3Endpoint)
		}
		base := u.Scheme + "://" + u.Host
		clientOpts = append(clientOpts, func(o *s3sdk.Options) {
			o.BaseEndpoint = aws.String(base)
			o.UsePathStyle = true
		})
	}
	forcePathStyle := cfg.ForcePathStyle
	if forcePathStyle != nil && *forcePathStyle {
		clientOpts = append(clientOpts, func(o *s3sdk.Options) {
			o.UsePathStyle = true
		})
	} else if cfg.S3Endpoint == "" {
		clientOpts = append(clientOpts, func(o *s3sdk.Options) {
			o.UsePathStyle = false
		})
	}

	client := s3sdk.NewFromConfig(awsCfg, clientOpts...)

	return &Client{
		s3Client: client,
		cfg:      cfg,
	}, nil
}

func (s *Client) logS3Call(op, bucket, key string, dur time.Duration, err error) {
	if fn := s.cfg.Logger; fn != nil {
		fn(op, bucket, key, dur, err)
	}
}

func (s *Client) PutObject(key string, data []byte) error {
	start := time.Now()
	_, err := s.s3Client.PutObject(context.Background(), &s3sdk.PutObjectInput{
		Bucket: aws.String(s.cfg.S3Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	s.logS3Call("PutObject", s.cfg.S3Bucket, key, time.Since(start), err)
	return err
}

func (s *Client) ReadObject(key string) ([]byte, error) {
	start := time.Now()
	output, err := s.s3Client.GetObject(context.Background(), &s3sdk.GetObjectInput{
		Bucket: aws.String(s.cfg.S3Bucket),
		Key:    aws.String(key),
	})
	s.logS3Call("GetObject", s.cfg.S3Bucket, key, time.Since(start), err)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, metadata.ErrKeyNotFound
		}
		return nil, err
	}
	defer func() { _ = output.Body.Close() }()
	return io.ReadAll(output.Body)
}

func (s *Client) DeleteObject(key string) error {
	start := time.Now()
	_, err := s.s3Client.DeleteObject(context.Background(), &s3sdk.DeleteObjectInput{
		Bucket: aws.String(s.cfg.S3Bucket),
		Key:    aws.String(key),
	})
	s.logS3Call("DeleteObject", s.cfg.S3Bucket, key, time.Since(start), err)
	return err
}

func (s *Client) ListObjects(prefix string) ([]metadata.Object, error) {
	start := time.Now()
	var objects []metadata.Object
	paginator := s3sdk.NewListObjectsV2Paginator(s.s3Client, &s3sdk.ListObjectsV2Input{
		Bucket: aws.String(s.cfg.S3Bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			s.logS3Call("ListObjectsV2", s.cfg.S3Bucket, prefix, time.Since(start), err)
			return nil, err
		}
		for _, obj := range page.Contents {
			objects = append(objects, metadata.Object{Key: obj.Key, LastModified: obj.LastModified})
		}
	}
	s.logS3Call("ListObjectsV2", s.cfg.S3Bucket, prefix, time.Since(start), nil)
	return objects, nil
}

func (s *Client) ListCommonPrefixes(prefix, delimiter string) ([]string, error) {
	start := time.Now()
	var prefixes []string
	paginator := s3sdk.NewListObjectsV2Paginator(s.s3Client, &s3sdk.ListObjectsV2Input{
		Bucket:    aws.String(s.cfg.S3Bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String(delimiter),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			s.logS3Call("ListObjectsV2", s.cfg.S3Bucket, prefix, time.Since(start), err)
			return nil, err
		}
		for _, cp := range page.CommonPrefixes {
			if cp.Prefix != nil {
				prefixes = append(prefixes, *cp.Prefix)
			}
		}
	}
	s.logS3Call("ListObjectsV2", s.cfg.S3Bucket, prefix, time.Since(start), nil)
	return prefixes, nil
}

func (s *Client) DeleteObjectsWithPrefix(prefix string) error {
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
		_, err = s.s3Client.DeleteObjects(context.Background(), &s3sdk.DeleteObjectsInput{
			Bucket: aws.String(s.cfg.S3Bucket),
			Delete: &types.Delete{Objects: idents[i:end]},
		})
		if err != nil {
			return fmt.Errorf("batch deleting objects (bucket=%s, prefix=%s): %w", s.cfg.S3Bucket, prefix, err)
		}
	}
	return nil
}
