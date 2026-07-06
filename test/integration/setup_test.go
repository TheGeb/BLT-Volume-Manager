//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	garageImage          = "dxflrs/garage:v2.3.0"
	garageBuiltID        = "garage-test:local"
	garageSharedBuildDir = ".docker-build-integration"
	etcdImage            = "gcr.io/etcd-development/etcd:v3.5.17"
)

var (
	sharedContainerID     string
	sharedEndpoint        string
	sharedAccessKey       string
	sharedSecretKey       string
	sharedEtcdContainerID string
	sharedEtcdEndpoint    string
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		fmt.Fprintln(os.Stderr, "docker not found, skipping integration tests")
		os.Exit(0)
	}

	if err := startSharedGarage(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shared Garage: %v\n", err)
		os.Exit(1)
	}

	if err := startSharedEtcd(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shared etcd: %v\n", err)
		os.Exit(1)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", sharedAccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", sharedSecretKey)
	os.Setenv("RESTIC_PASSWORD", "test-password")
	os.Setenv("S3_FORCE_PATH_STYLE", "true")
	os.Setenv("BLT_DEV_MODE", "1")

	code := m.Run()
	stopSharedEtcd()
	stopSharedGarage()
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

func startSharedGarage() error {
	sharedAccessKey = randomHex(20)
	sharedSecretKey = randomHex(40)

	if err := os.MkdirAll(garageSharedBuildDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

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

	if err := os.WriteFile(filepath.Join(garageSharedBuildDir, "config.toml"), []byte(configStr), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	dockerfile := "FROM " + garageImage + "\nCOPY config.toml /etc/garage.toml\n"
	if err := os.WriteFile(filepath.Join(garageSharedBuildDir, "Dockerfile"), []byte(dockerfile), 0o644); err != nil {
		return fmt.Errorf("write dockerfile: %w", err)
	}

	if out, err := exec.Command("docker", "build", "-t", garageBuiltID, garageSharedBuildDir).CombinedOutput(); err != nil {
		return fmt.Errorf("build garage image: %w\n%s", err, out)
	}

	cmd := exec.Command("docker", "run", "-d",
		"-p", "3900",
		"-e", "GARAGE_DEFAULT_ACCESS_KEY="+sharedAccessKey,
		"-e", "GARAGE_DEFAULT_SECRET_KEY="+sharedSecretKey,
		"-e", "GARAGE_DEFAULT_BUCKET=shared-garage-default",
		garageBuiltID,
		"/garage", "server", "--single-node", "--default-bucket",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start garage container: %w\n%s", err, out)
	}
	sharedContainerID = strings.TrimSpace(string(out))

	var hostPort string
	for i := 0; i < 120; i++ {
		portOut, err := exec.Command("docker", "port", sharedContainerID, "3900").CombinedOutput()
		if err == nil {
			hostPort = strings.TrimSpace(string(portOut))
			if hostPort != "" {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if hostPort == "" {
		inspect, _ := exec.Command("docker", "inspect", sharedContainerID).CombinedOutput()
		logs, _ := exec.Command("docker", "logs", sharedContainerID).CombinedOutput()
		_ = exec.Command("docker", "rm", "-f", sharedContainerID).Run()
		return fmt.Errorf("garage container did not publish port 3900\ninspect:\n%s\nlogs:\n%s", inspect, logs)
	}

	parts := strings.Split(hostPort, ":")
	sharedEndpoint = fmt.Sprintf("http://localhost:%s", parts[len(parts)-1])

	if err := waitForEndpoint(sharedEndpoint, 60*time.Second); err != nil {
		_ = exec.Command("docker", "rm", "-f", sharedContainerID).Run()
		return fmt.Errorf("garage not ready: %w", err)
	}

	return nil
}

func stopSharedGarage() {
	_ = exec.Command("docker", "rm", "-f", sharedContainerID).Run()
	_ = os.RemoveAll(garageSharedBuildDir)
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

func startSharedEtcd() error {
	if out, err := exec.Command("docker", "pull", etcdImage).CombinedOutput(); err != nil {
		return fmt.Errorf("pull etcd image: %w\n%s", err, out)
	}

	cmd := exec.Command("docker", "run", "-d",
		"-p", "2379",
		etcdImage,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("start etcd container: %w\n%s", err, out)
	}
	sharedEtcdContainerID = strings.TrimSpace(string(out))

	var hostPort string
	for i := 0; i < 120; i++ {
		portOut, err := exec.Command("docker", "port", sharedEtcdContainerID, "2379").CombinedOutput()
		if err == nil {
			hostPort = strings.TrimSpace(string(portOut))
			if hostPort != "" {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if hostPort == "" {
		inspect, _ := exec.Command("docker", "inspect", sharedEtcdContainerID).CombinedOutput()
		logs, _ := exec.Command("docker", "logs", sharedEtcdContainerID).CombinedOutput()
		_ = exec.Command("docker", "rm", "-f", sharedEtcdContainerID).Run()
		return fmt.Errorf("etcd container did not publish port 2379\ninspect:\n%s\nlogs:\n%s", inspect, logs)
	}

	parts := strings.Split(hostPort, ":")
	etcdPort := parts[len(parts)-1]
	sharedEtcdEndpoint = fmt.Sprintf("http://localhost:%s", etcdPort)

	if err := waitForPort("localhost", etcdPort, 60*time.Second); err != nil {
		_ = exec.Command("docker", "rm", "-f", sharedEtcdContainerID).Run()
		return fmt.Errorf("etcd not ready: %w", err)
	}

	return nil
}

func stopSharedEtcd() {
	if sharedEtcdContainerID != "" {
		_ = exec.Command("docker", "rm", "-f", sharedEtcdContainerID).Run()
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

func waitForPort(host, port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("port %s not reachable within %v", net.JoinHostPort(host, port), timeout)
}

func waitForEndpoint(endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest("GET", endpoint+"/", nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req) //nolint:gosec // Test helper checking known Garage endpoint
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("endpoint %s not reachable within %v", endpoint, timeout)
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
