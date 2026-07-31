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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var ErrNotFound = errors.New("key not found")

type Object struct {
	Key                 *string
	ModificationCounter *int64
}

type Config struct {
	Bucket         string
	Endpoint       string
	Region         string
	ForcePathStyle bool
	Logger         func(op, bucket, key string, dur time.Duration, err error)
}

func (cfg Config) validate() error {
	if cfg.Bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}
	return nil
}

type Client struct {
	raw    *s3sdk.Client
	bucket string
	logger func(op, bucket, key string, dur time.Duration, err error)
}

func NewClient(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	loadCfg := []func(*config.LoadOptions) error{}
	if cfg.Region != "" {
		loadCfg = append(loadCfg, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), loadCfg...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	clientOpts := []func(*s3sdk.Options){}
	if cfg.Endpoint != "" {
		ep := cfg.Endpoint
		if !strings.Contains(ep, "://") {
			ep = "https://" + ep
		}
		u, err := url.Parse(ep)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint %q: %w", cfg.Endpoint, err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("endpoint %q has no host", cfg.Endpoint)
		}
		clientOpts = append(clientOpts, func(o *s3sdk.Options) {
			o.BaseEndpoint = aws.String(u.Scheme + "://" + u.Host)
			o.UsePathStyle = true
		})
	}
	if cfg.ForcePathStyle {
		clientOpts = append(clientOpts, func(o *s3sdk.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3sdk.NewFromConfig(awsCfg, clientOpts...)
	return &Client{
		raw:    client,
		bucket: cfg.Bucket,
		logger: cfg.Logger,
	}, nil
}

func (c *Client) logCall(op, key string, dur time.Duration, err error) {
	if fn := c.logger; fn != nil {
		fn(op, c.bucket, key, dur, err)
	}
}

func (c *Client) PutObject(ctx context.Context, key string, data []byte) error {
	start := time.Now()
	_, err := c.raw.PutObject(ctx, &s3sdk.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	c.logCall("PutObject", key, time.Since(start), err)
	return err
}

func (c *Client) ReadObject(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	output, err := c.raw.GetObject(ctx, &s3sdk.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	c.logCall("GetObject", key, time.Since(start), err)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer func() { _ = output.Body.Close() }()
	return io.ReadAll(output.Body)
}

func (c *Client) DeleteObject(ctx context.Context, key string) error {
	start := time.Now()
	_, err := c.raw.DeleteObject(ctx, &s3sdk.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	c.logCall("DeleteObject", key, time.Since(start), err)
	return err
}

func (c *Client) ListObjects(ctx context.Context, prefix string) ([]Object, error) {
	start := time.Now()
	var objects []Object
	paginator := s3sdk.NewListObjectsV2Paginator(c.raw, &s3sdk.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			c.logCall("ListObjectsV2", prefix, time.Since(start), err)
			return nil, err
		}
		for _, obj := range page.Contents {
			var mod *int64
			if obj.LastModified != nil {
				mod = aws.Int64(obj.LastModified.UnixNano())
			}
			objects = append(objects, Object{Key: obj.Key, ModificationCounter: mod})
		}
	}
	c.logCall("ListObjectsV2", prefix, time.Since(start), nil)
	return objects, nil
}

func (c *Client) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	objects, err := c.ListObjects(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list objects (prefix=%q): %w", prefix, err)
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
		result, err := c.raw.DeleteObjects(ctx, &s3sdk.DeleteObjectsInput{
			Bucket: aws.String(c.bucket),
			Delete: &types.Delete{Objects: idents[i:end]},
		})
		if err != nil {
			return fmt.Errorf("batch delete objects (bucket=%q, prefix=%q): %w", c.bucket, prefix, err)
		}
		if err := checkDeleteObjectsResponse(result, c.bucket, prefix); err != nil {
			return err
		}
	}
	return nil
}

func checkDeleteObjectsResponse(result *s3sdk.DeleteObjectsOutput, bucket, prefix string) error {
	if result != nil && len(result.Errors) > 0 {
		failed := make([]string, 0, len(result.Errors))
		for _, e := range result.Errors {
			key := "(unknown)"
			if e.Key != nil {
				key = *e.Key
			}
			msg := "(unknown)"
			if e.Message != nil {
				msg = *e.Message
			}
			failed = append(failed, fmt.Sprintf("%s: %s", key, msg))
		}
		return fmt.Errorf("batch delete objects (bucket=%q, prefix=%q): partial failure: %s", bucket, prefix, strings.Join(failed, "; "))
	}
	return nil
}
