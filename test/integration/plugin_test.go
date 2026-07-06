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
	"path/filepath"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/driver"
	"github.com/docker/go-plugins-helpers/volume"
)

func setupPluginTest(t *testing.T, backendType string) string {
	t.Helper()

	garage := StartGarage(t)

	dataDir := t.TempDir()
	resticBase := "s3:" + garage.Endpoint + "/" + garage.BucketName

	conf := cfg.Config{
		DataDir:    dataDir,
		ResticBase: resticBase,
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		S3Region:   "us-east-1",
	}
	if backendType == "etcd" {
		etcd := StartEtcd(t)
		conf.MetadataBackend = "etcd"
		conf.EtcdEndpoints = []string{etcd.Endpoint}
	}

	drv := driver.New(conf, context.Background())
	h := volume.NewHandler(drv)

	socketPath := filepath.Join(t.TempDir(), "plugin.sock")
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close(); os.Remove(socketPath) })
	go h.Serve(l)

	return socketPath
}

func pluginDo(t *testing.T, socketPath, endpoint string, req, resp any) {
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
	if resp != nil {
		if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
			body, _ := io.ReadAll(httpResp.Body)
			t.Fatalf("%s: decode error: %v\nbody: %s", endpoint, err, string(body))
		}
	}
}

func pluginOK(t *testing.T, socketPath, endpoint string, req any) map[string]any {
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
	body, _ := io.ReadAll(httpResp.Body)
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("%s: decode: %v\nbody: %s", endpoint, err, string(body))
	}
	if errStr, hasErr := m["Err"].(string); hasErr && errStr != "" {
		t.Fatalf("%s: unexpected error: %s", endpoint, errStr)
	}
	return m
}

func testPluginCreateVolume(t *testing.T, socket string) {
	m := pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "test-vol"})
	if err, ok := m["Err"].(string); ok && err != "" {
		t.Fatalf("create: %s", err)
	}
}

func testPluginCreateDuplicate(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "dup-vol"})
	m := pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "dup-vol"})
	if err, ok := m["Err"].(string); ok && err != "" {
		t.Fatalf("duplicate create: %s", err)
	}
}

func testPluginListVolumes(t *testing.T, socket string) {
	m := pluginOK(t, socket, "VolumeDriver.List", nil)
	vols, _ := m["Volumes"].([]any)
	initialCount := len(vols)

	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "list-vol-1"})
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "list-vol-2"})

	m = pluginOK(t, socket, "VolumeDriver.List", nil)
	vols, _ = m["Volumes"].([]any)
	if got := len(vols); got != initialCount+2 {
		t.Fatalf("expected %d volumes, got %d", initialCount+2, got)
	}
}

func testPluginGetVolume(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "get-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "get-vol"})
	v, ok := m["Volume"].(map[string]any)
	if !ok {
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
	if mp, _ := m["Mountpoint"].(string); mp == "" {
		t.Fatal("expected mountpoint in path response")
	}
}

func testPluginMountUnmount(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "mount-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: "mount-vol", ID: "mount-1"})
	mp, _ := m["Mountpoint"].(string)
	if mp == "" {
		t.Fatal("expected mountpoint after mount")
	}

	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "mount-vol", ID: "mount-1"})
}

func testPluginRemoveVolume(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "remove-vol"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "remove-vol"})

	m := pluginOK(t, socket, "VolumeDriver.List", nil)
	vols, _ := m["Volumes"].([]any)
	for _, v := range vols {
		if vm, ok := v.(map[string]any); ok && vm["Name"] == "remove-vol" {
			t.Fatal("volume should have been removed")
		}
	}
}

func testPluginFullLifecycle(t *testing.T, socket string) {
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "lifecycle-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: "lifecycle-vol", ID: "lifecycle-1"})
	mp, _ := m["Mountpoint"].(string)
	if mp == "" {
		t.Fatal("expected mountpoint")
	}

	m = pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "lifecycle-vol"})
	v, _ := m["Volume"].(map[string]any)
	if v["Name"] != "lifecycle-vol" {
		t.Fatal("wrong volume name on get")
	}

	m = pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "lifecycle-vol"})
	if m["Mountpoint"] != mp {
		t.Fatalf("path returned different mountpoint: %v vs %v", m["Mountpoint"], mp)
	}

	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "lifecycle-vol", ID: "lifecycle-1"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "lifecycle-vol"})
}

func testPluginCapabilities(t *testing.T, socket string) {
	m := pluginOK(t, socket, "VolumeDriver.Capabilities", nil)
	caps, ok := m["Capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("expected Capabilities in response: %v", m)
	}
	if caps["Scope"] != "local" {
		t.Fatalf("expected scope 'local', got %v", caps["Scope"])
	}
}

func testPluginEdgeCases(t *testing.T, socket string) {
	m := pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "no-such-vol"})
	if err, ok := m["Err"].(string); ok && err != "" {
		t.Fatalf("remove non-existent: %s", err)
	}

	m = pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "no-such-vol"})
	if v, ok := m["Volume"].(map[string]any); ok {
		if v["Name"] != "no-such-vol" {
			t.Fatalf("expected name no-such-vol, got %v", v["Name"])
		}
	}

	m = pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "no-such-vol", ID: "test"})
	if err, ok := m["Err"].(string); ok && err != "" {
		t.Fatalf("unmount non-existent: %s", err)
	}

	m = pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "no-such-vol"})
	if mp, _ := m["Mountpoint"].(string); mp == "" {
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
			t.Run("Capabilities", func(t *testing.T) { testPluginCapabilities(t, socket) })
			t.Run("EdgeCases", func(t *testing.T) { testPluginEdgeCases(t, socket) })
		})
	}
}
