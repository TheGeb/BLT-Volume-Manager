package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	accessKey := randomHex(20)
	secretKey := randomHex(40)
	bucketName := "test-bucket-" + randomString(8)

	// Build a one-off image with the config baked in — use project-relative path
	// so Docker's build context is accessible to the daemon.
	buildDir := filepath.Join(".docker-build-"+randomString(8))
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(buildDir) })
	rpcSecret := randomHex(64)
	config := fmt.Sprintf(`
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

	if err := os.WriteFile(filepath.Join(buildDir, "config.toml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}
	dockerfile := "FROM " + garageImage + "\nCOPY config.toml /etc/garage.toml\n"
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatal(err)
	}

	if out, err := exec.Command("docker", "build", "-t", garageBuiltID, buildDir).CombinedOutput(); err != nil {
		t.Fatalf("build garage image: %v\n%s", err, out)
	}

	cmd := exec.Command("docker", "run", "-d",
		"-p", "3900",
		"-e", "GARAGE_DEFAULT_ACCESS_KEY="+accessKey,
		"-e", "GARAGE_DEFAULT_SECRET_KEY="+secretKey,
		"-e", "GARAGE_DEFAULT_BUCKET="+bucketName,
		garageBuiltID,
		"/garage", "server", "--single-node", "--default-bucket",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("start garage container: %v\n%s", err, out)
	}
	containerID := strings.TrimSpace(string(out))

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
	})

	var hostPort string
	for i := 0; i < 30; i++ {
		portOut, err := exec.Command("docker", "port", containerID, "3900").CombinedOutput()
		if err == nil {
			hostPort = strings.TrimSpace(string(portOut))
			if hostPort != "" {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if hostPort == "" {
		logs, _ := exec.Command("docker", "logs", containerID).CombinedOutput()
		_ = exec.Command("docker", "rm", "-f", containerID).Run()
		t.Fatalf("garage container did not publish port 3900\nlogs:\n%s", logs)
	}

	parts := strings.Split(hostPort, ":")
	endpoint := fmt.Sprintf("http://localhost:%s", parts[len(parts)-1])

	gs := &GarageServer{
		AccessKey:  accessKey,
		SecretKey:  secretKey,
		BucketName: bucketName,
		Endpoint:   endpoint,
	}

	gs.waitReady(t)
	return gs
}

func (g *GarageServer) waitReady(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// We hit the S3 API root; an AccessDenied response means Garage is up.
	checkURL := g.Endpoint + "/"
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("garage at %s did not become ready within 30s", g.Endpoint)
		default:
			resp, err := http.Get(checkURL)
			if err == nil {
				_ = resp.Body.Close()
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
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
