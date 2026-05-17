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
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3rw struct {
	s3Client *s3.Client
	opts     S3StoreOpts
}

type S3StoreOpts struct {
	AwsBucketName string
	AwsLockFolder string
	S3Endpoint    string
	Region        string
}

func (opts S3StoreOpts) validate() error {
	if opts.AwsBucketName == "" {
		return fmt.Errorf("AwsBucketName required")
	}
	return nil
}

func NewS3Store(opts S3StoreOpts) (*S3rw, error) {
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
			scheme = "https"
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

func (s *S3rw) GetLockOwner() (*LockOwner, error) {
	bucketKey := s.opts.AwsLockFolder + "owner.json"
	getLockOwnerObject := &s3.GetObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    &bucketKey,
	}

	output, err := s.s3Client.GetObject(context.Background(), getLockOwnerObject)

	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting lock owner (bucket=%s, key=%s): %w", s.opts.AwsBucketName, bucketKey, err)
	}

	body, err := io.ReadAll(output.Body)
	defer output.Body.Close()

	if err != nil {
		return nil, fmt.Errorf("error reading the lock owner file")
	}

	var lockOwner LockOwner
	err = json.Unmarshal(body, &lockOwner)
	if err != nil {
		return nil, fmt.Errorf("error reading the owner file")
	}

	return &lockOwner, nil
}

func (s *S3rw) GetLockCounter() (*LockCounter, error) {
	bucketKey := s.opts.AwsLockFolder + "counter.json"
	getLockOwnerObject := &s3.GetObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    &bucketKey,
	}

	output, err := s.s3Client.GetObject(context.Background(), getLockOwnerObject)
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting the lock counter")
	}

	body, err := io.ReadAll(output.Body)
	defer output.Body.Close()

	if err != nil {
		return nil, fmt.Errorf("error reading the lock counter file")
	}

	b := string(body)
	if b == "" {
		return nil, fmt.Errorf("error reading the lock counter file")
	}

	c, e := strconv.Atoi(b)
	if e != nil {
		return nil, fmt.Errorf("error reading the lock counter file")
	}

	return &LockCounter{
		Counter: c,
	}, nil
}

func (s *S3rw) SetLockCounter(c LockCounter) error {
	contents := strconv.Itoa(c.Counter)
	bucketKey := s.opts.AwsLockFolder + "counter.json"
	putObjectRequest := &s3.PutObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    aws.String(bucketKey),
		Body:   strings.NewReader(contents),
	}

	_, err := s.s3Client.PutObject(context.Background(), putObjectRequest)

	if err != nil {
		return fmt.Errorf("setting lock counter %d (bucket=%s, key=%s): %w", c.Counter, s.opts.AwsBucketName, bucketKey, err)
	}
	return nil
}

func (s *S3rw) SetLockOwner(owner LockOwner) error {
	jsonData, err := json.Marshal(owner)
	if err != nil {
		return fmt.Errorf("unable to set lock owner, Error marshalling")
	}

	bucketKey := s.opts.AwsLockFolder + "owner.json"
	putObjectRequest := &s3.PutObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    aws.String(bucketKey),
		Body:   bytes.NewReader(jsonData),
	}

	_, err = s.s3Client.PutObject(context.Background(), putObjectRequest)
	if err != nil {
		return fmt.Errorf("setting lock owner %s (bucket=%s, key=%s): %w", owner.Name, s.opts.AwsBucketName, bucketKey, err)
	}

	return nil
}

func (s *S3rw) RollBackLockOwner() error {
	bucketKey := s.opts.AwsLockFolder + "owner.json"
	deleteObjectRequest := &s3.DeleteObjectInput{
		Bucket: aws.String(s.opts.AwsBucketName),
		Key:    aws.String(bucketKey),
	}

	_, err := s.s3Client.DeleteObject(context.Background(), deleteObjectRequest)
	if err != nil {
		return fmt.Errorf("rolling back lock owner (bucket=%s, key=%s): %w", s.opts.AwsBucketName, bucketKey, err)
	}

	return nil
}

func (s *S3rw) DeleteLockObjects() error {
	ownerKey := s.opts.AwsLockFolder + "owner.json"
	counterKey := s.opts.AwsLockFolder + "counter.json"

	for _, key := range []string{ownerKey, counterKey} {
		_, err := s.s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
			Bucket: aws.String(s.opts.AwsBucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("deleting lock object %s (bucket=%s): %w", key, s.opts.AwsBucketName, err)
		}
	}

	return nil
}
