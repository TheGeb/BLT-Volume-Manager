//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	garageImage = "dxflrs/garage:v2.3.0"
	etcdImage   = "gcr.io/etcd-development/etcd:v3.5.17"
)

var (
	sharedGarageContainer testcontainers.Container
	sharedEtcdContainer   testcontainers.Container
	sharedEndpoint        string
	sharedAccessKey       string
	sharedSecretKey       string
	sharedEtcdEndpoint    string
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintln(os.Stderr, "docker not found, skipping integration tests")
		os.Exit(0)
	}

	ctx := context.Background()

	if err := startSharedGarage(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shared Garage: %v\n", err)
		os.Exit(1)
	}

	if err := startSharedEtcd(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shared etcd: %v\n", err)
		os.Exit(1)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", sharedAccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", sharedSecretKey)
	os.Setenv("RESTIC_PASSWORD", "test-password")
	os.Setenv("RESTIC_FROM_PASSWORD", "test-password")
	os.Setenv("S3_FORCE_PATH_STYLE", "true")
	os.Setenv("BLT_DEV_MODE", "1")

	code := m.Run()
	stopSharedGarage()
	stopSharedEtcd()
	os.Exit(code)
}

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

func startSharedGarage(ctx context.Context) error {
	sharedAccessKey = randomHex(20)
	sharedSecretKey = randomHex(40)

	rpcSecret := randomHex(64)
	configStr := fmt.Sprintf(`
metadata_dir = "/tmp/garage/meta"
data_dir = "/tmp/garage/data"
db_engine = "sqlite"
rpc_bind_addr = "[::]:3901"
rpc_secret = "%s"
replication_factor = 1

[s3_api]
s3_region = "us-east-1"
api_bind_addr = "0.0.0.0:3900"
root_domain = ".s3.garage.localhost"

[s3_web]
bind_addr = "0.0.0.0:3902"
root_domain = ".web.garage.localhost"

[admin]
api_bind_addr = "0.0.0.0:3903"
`, rpcSecret)

	req := testcontainers.ContainerRequest{
		Image:        garageImage,
		ExposedPorts: []string{"3900/tcp"},
		Env: map[string]string{
			"GARAGE_DEFAULT_ACCESS_KEY": sharedAccessKey,
			"GARAGE_DEFAULT_SECRET_KEY": sharedSecretKey,
			"GARAGE_DEFAULT_BUCKET":     "shared-garage-default",
		},
		Cmd: []string{"/garage", "server", "--single-node", "--default-bucket"},
		Files: []testcontainers.ContainerFile{
			{
				Reader:        strings.NewReader(configStr),
				ContainerFilePath: "/etc/garage.toml",
				FileMode:      0o644,
			},
		},
		WaitingFor: wait.ForHTTP("/").
			WithPort("3900/tcp").
			WithStatusCodeMatcher(func(int) bool { return true }).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start garage container: %w", err)
	}
	sharedGarageContainer = container

	port, err := container.MappedPort(ctx, "3900/tcp")
	if err != nil {
		return fmt.Errorf("get garage mapped port: %w", err)
	}
	sharedEndpoint = fmt.Sprintf("http://localhost:%s", port.Port())

	return nil
}

func stopSharedGarage() {
	if sharedGarageContainer != nil {
		_ = sharedGarageContainer.Terminate(context.Background())
	}
}

type EtcdServer struct {
	Endpoint string
}

func StartEtcd(t *testing.T) *EtcdServer {
	t.Helper()
	return &EtcdServer{
		Endpoint: sharedEtcdEndpoint,
	}
}

func startSharedEtcd(ctx context.Context) error {
	req := testcontainers.ContainerRequest{
		Image:        etcdImage,
		ExposedPorts: []string{"2379/tcp"},
		Entrypoint:   []string{"etcd"},
		Cmd: []string{
			"--listen-client-urls", "http://0.0.0.0:2379",
			"--advertise-client-urls", "http://0.0.0.0:2379",
		},
		WaitingFor: wait.ForHTTP("/health").
			WithPort("2379/tcp").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("start etcd container: %w", err)
	}
	sharedEtcdContainer = container

	port, err := container.MappedPort(ctx, "2379/tcp")
	if err != nil {
		return fmt.Errorf("get etcd mapped port: %w", err)
	}
	sharedEtcdEndpoint = fmt.Sprintf("http://localhost:%s", port.Port())

	return nil
}

func stopSharedEtcd() {
	if sharedEtcdContainer != nil {
		_ = sharedEtcdContainer.Terminate(context.Background())
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
