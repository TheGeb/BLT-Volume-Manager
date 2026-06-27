//go:build integration

package integration

import (
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
	garageSharedBuildDir = ".docker-build-integration"
)

var (
	sharedContainerID string
	sharedEndpoint    string
	sharedAccessKey   string
	sharedSecretKey   string
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

	os.Setenv("AWS_ACCESS_KEY_ID", sharedAccessKey)
	os.Setenv("AWS_SECRET_ACCESS_KEY", sharedSecretKey)
	os.Setenv("RESTIC_PASSWORD", "test-password")
	os.Setenv("S3_FORCE_PATH_STYLE", "true")
	os.Setenv("BLT_TEST_MODE", "1")

	code := m.Run()
	stopSharedGarage()
	os.Exit(code)
}

func startSharedGarage() error {
	sharedAccessKey = randomHex(20)
	sharedSecretKey = randomHex(40)

	if err := os.MkdirAll(garageSharedBuildDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

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

	if err := os.WriteFile(filepath.Join(garageSharedBuildDir, "config.toml"), []byte(config), 0o644); err != nil {
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
	for i := 0; i < 60; i++ {
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
		logs, _ := exec.Command("docker", "logs", sharedContainerID).CombinedOutput()
		_ = exec.Command("docker", "rm", "-f", sharedContainerID).Run()
		return fmt.Errorf("garage container did not publish port 3900\nlogs:\n%s", logs)
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
