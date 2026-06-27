//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	garageImage   = "dxflrs/garage:v2.3.0"
	garageBuiltID = "garage-test:local"
)

type GarageServer struct {
	AccessKey  string
	SecretKey  string
	BucketName string
	Endpoint   string
}

func StartGarage(t *testing.T) *GarageServer {
	t.Helper()

	bucketName := "test-bucket-" + randomString(8)

	createBucket(t, bucketName)

	t.Cleanup(func() {
		deleteBucket(t, bucketName)
	})

	return &GarageServer{
		AccessKey:  sharedAccessKey,
		SecretKey:  sharedSecretKey,
		BucketName: bucketName,
		Endpoint:   sharedEndpoint,
	}
}

func createBucket(t *testing.T, bucket string) {
	t.Helper()

	creds := credentials.NewStaticCredentialsProvider(sharedAccessKey, sharedSecretKey, "")
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}

	client := s3sdk.NewFromConfig(awsCfg, func(o *s3sdk.Options) {
		o.BaseEndpoint = aws.String(sharedEndpoint)
		o.UsePathStyle = true
	})

	_, err = client.CreateBucket(context.Background(), &s3sdk.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("create bucket %s: %v", bucket, err)
	}
}

func deleteBucket(t *testing.T, bucket string) {
	creds := credentials.NewStaticCredentialsProvider(sharedAccessKey, sharedSecretKey, "")
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		t.Logf("load aws config for bucket cleanup: %v", err)
		return
	}

	client := s3sdk.NewFromConfig(awsCfg, func(o *s3sdk.Options) {
		o.BaseEndpoint = aws.String(sharedEndpoint)
		o.UsePathStyle = true
	})

	listResp, err := client.ListObjectsV2(context.Background(), &s3sdk.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		for _, obj := range listResp.Contents {
			_, _ = client.DeleteObject(context.Background(), &s3sdk.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    obj.Key,
			})
		}
	}

	_, err = client.DeleteBucket(context.Background(), &s3sdk.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Logf("delete bucket %s: %v", bucket, err)
	}
}

func randomHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)[:n]
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
