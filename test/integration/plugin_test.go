//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/go-plugins-helpers/volume"
)

type volumePluginResponse struct {
	Err        string           `json:"Err"`
	Volumes    []map[string]any `json:"Volumes,omitempty"`
	Volume     map[string]any   `json:"Volume,omitempty"`
	Mountpoint string           `json:"Mountpoint,omitempty"`
}

func setupPluginTest(t *testing.T, backendType string) string {
	t.Helper()

	setupLogCapture(t)

	garage := StartGarage(t)

	socketPath := filepath.Join(t.TempDir(), "plugin.sock")
	dataDir := t.TempDir()

	env := append(os.Environ(),
		"BLT_LISTEN=1",
		"RESTIC_REPOSITORY=s3:"+garage.Endpoint+"/"+garage.BucketName,
		"S3_ENDPOINT="+garage.Endpoint,
		"S3_REGION=us-east-1",
	)
	if backendType == "etcd" {
		etcd := StartEtcd(t)
		env = append(env, "BLT_METADATA_BACKEND=etcd")
		env = append(env, "ETCD_ENDPOINTS="+etcd.Endpoint)
	}

	cmd := exec.Command(driverBin, "-data-dir", dataDir, "-socket", socketPath)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start driver: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	})

	waitForSocket(t, socketPath, 15*time.Second)
	return socketPath
}

func pluginOK(t *testing.T, socketPath, endpoint string, req any) volumePluginResponse {
	t.Helper()
	var r io.Reader
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
	httpResp, err := client.Post("http://unix/"+endpoint, "application/json", r)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.Fatalf("%s: read body: %v", endpoint, err)
	}
	var m volumePluginResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("%s: decode: %v\nbody: %s", endpoint, err, string(body))
	}
	if m.Err != "" {
		t.Fatalf("%s: unexpected error: %s", endpoint, m.Err)
	}
	return m
}

func testPluginCreateVolume(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "test-vol"})
}

func testPluginCreateDuplicate(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "dup-vol"})
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "dup-vol"})
}

func testPluginListVolumes(t *testing.T, socket string) {
	m := pluginOK(t, socket, "VolumeDriver.List", nil)
	initialCount := len(m.Volumes)

	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "list-vol-1"})
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "list-vol-2"})

	m = pluginOK(t, socket, "VolumeDriver.List", nil)
	if got := len(m.Volumes); got != initialCount+2 {
		t.Fatalf("expected %d volumes, got %d", initialCount+2, got)
	}
}

func testPluginGetVolume(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "get-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "get-vol"})
	v := m.Volume
	if v == nil {
		t.Fatalf("get: no Volume in response: %v", m)
	}
	if v["Name"] != "get-vol" {
		t.Fatalf("expected name get-vol, got %v", v["Name"])
	}
	mountpoint, _ := v["Mountpoint"].(string)
	if mountpoint == "" {
		t.Fatal("expected non-empty mountpoint")
	}
}

func testPluginPathVolume(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "path-vol"})
	m := pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "path-vol"})
	if m.Mountpoint == "" {
		t.Fatal("expected mountpoint in path response")
	}
}

func testPluginMountUnmount(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "mount-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: "mount-vol", ID: "mount-1"})
	if m.Mountpoint == "" {
		t.Fatal("expected mountpoint after mount")
	}

	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "mount-vol", ID: "mount-1"})
}

func testPluginRemoveVolume(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "remove-vol"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "remove-vol"})

	m := pluginOK(t, socket, "VolumeDriver.List", nil)
	for _, vm := range m.Volumes {
		if vm["Name"] == "remove-vol" {
			t.Fatal("volume should have been removed")
		}
	}
}

func testPluginFullLifecycle(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "lifecycle-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: "lifecycle-vol", ID: "lifecycle-1"})
	mp := m.Mountpoint
	if mp == "" {
		t.Fatal("expected mountpoint")
	}

	m = pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "lifecycle-vol"})
	if m.Volume["Name"] != "lifecycle-vol" {
		t.Fatal("wrong volume name on get")
	}

	m = pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "lifecycle-vol"})
	if m.Mountpoint != mp {
		t.Fatalf("path returned different mountpoint: %v vs %v", m.Mountpoint, mp)
	}

	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "lifecycle-vol", ID: "lifecycle-1"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "lifecycle-vol"})
}

func testPluginEdgeCases(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "no-such-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "no-such-vol"})
	if m.Volume != nil && m.Volume["Name"] != "no-such-vol" {
		t.Fatalf("expected name no-such-vol, got %v", m.Volume["Name"])
	}

	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "no-such-vol", ID: "test"})

	m = pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "no-such-vol"})
	if m.Mountpoint == "" {
		t.Fatal("expected a mountpoint even for non-existent volume")
	}

	createdVol := "edge-lock-vol"
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: createdVol})
	pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: createdVol, ID: "edge-1"})
	pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: createdVol, ID: "edge-2"})
	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: createdVol, ID: "edge-1"})
	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: createdVol, ID: "edge-2"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: createdVol})
}

func TestPlugin(t *testing.T) {
	for _, backendType := range []string{"s3", "etcd"} {
		backendType := backendType
		t.Run(backendType, func(t *testing.T) {
			socket := setupPluginTest(t, backendType)

			t.Run("CreateVolume", func(t *testing.T) { testPluginCreateVolume(t, socket) })
			t.Run("CreateDuplicate", func(t *testing.T) { testPluginCreateDuplicate(t, socket) })
			t.Run("ListVolumes", func(t *testing.T) { testPluginListVolumes(t, socket) })
			t.Run("GetVolume", func(t *testing.T) { testPluginGetVolume(t, socket) })
			t.Run("PathVolume", func(t *testing.T) { testPluginPathVolume(t, socket) })
			t.Run("MountUnmount", func(t *testing.T) { testPluginMountUnmount(t, socket) })
			t.Run("RemoveVolume", func(t *testing.T) { testPluginRemoveVolume(t, socket) })
			t.Run("FullLifecycle", func(t *testing.T) { testPluginFullLifecycle(t, socket) })
			t.Run("EdgeCases", func(t *testing.T) { testPluginEdgeCases(t, socket) })
		})
	}
}
