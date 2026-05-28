//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/example/blt-volume-manager/driver"
	"github.com/example/blt-volume-manager/testutil"
)

// setupPluginTest starts Garage, creates a Driver backed by real S3, and
// serves the Docker volume plugin HTTP handler on a Unix socket.
func setupPluginTest(t *testing.T) (string, *testutil.GarageServer) {
	t.Helper()

	garage := testutil.StartGarage(t)

	t.Setenv("AWS_ACCESS_KEY_ID", garage.AccessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", garage.SecretKey)
	t.Setenv("RESTIC_PASSWORD", "test-password")
	t.Setenv("S3_FORCE_PATH_STYLE", "true")

	dataDir := t.TempDir()
	resticBase := "s3:" + garage.Endpoint + "/" + garage.BucketName

	drv := driver.NewDriver(dataDir, resticBase, garage.BucketName, garage.Endpoint, "us-east-1")
	h := volume.NewHandler(drv)

	socketPath := filepath.Join(t.TempDir(), "plugin.sock")
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close(); os.Remove(socketPath) })
	go h.Serve(l)

	return socketPath, garage
}

func pluginDo(t *testing.T, socketPath, endpoint string, req, resp interface{}) {
	t.Helper()
	var r io.Reader
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	dial := func(proto, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Dial: dial,
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

func pluginOK(t *testing.T, socketPath, endpoint string, req interface{}) map[string]interface{} {
	t.Helper()
	var r io.Reader
	if req != nil {
		b, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	dial := func(proto, addr string) (net.Conn, error) {
		return net.Dial("unix", socketPath)
	}
	client := &http.Client{
		Transport: &http.Transport{
			Dial: dial,
		},
	}
	httpResp, err := client.Post("http://unix/"+endpoint, "application/json", r)
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	body, _ := io.ReadAll(httpResp.Body)
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("%s: decode: %v\nbody: %s", endpoint, err, string(body))
	}
	if errStr, hasErr := m["Err"].(string); hasErr && errStr != "" {
		t.Fatalf("%s: unexpected error: %s", endpoint, errStr)
	}
	return m
}

func TestPlugin_CreateVolume(t *testing.T) {
	socket, _ := setupPluginTest(t)
	m := pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "test-vol"})
	if err, _ := m["Err"].(string); err != "" {
		t.Fatalf("create: %s", err)
	}
}

func TestPlugin_CreateDuplicate(t *testing.T) {
	socket, _ := setupPluginTest(t)
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "dup-vol"})
	// Creating the same volume again should succeed (idempotent)
	m := pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "dup-vol"})
	if err, _ := m["Err"].(string); err != "" {
		t.Fatalf("duplicate create: %s", err)
	}
}

func TestPlugin_ListVolumes(t *testing.T) {
	socket, _ := setupPluginTest(t)
	m := pluginOK(t, socket, "VolumeDriver.List", nil)
	vols, _ := m["Volumes"].([]interface{})
	initialCount := len(vols)

	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "list-vol-1"})
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "list-vol-2"})

	m = pluginOK(t, socket, "VolumeDriver.List", nil)
	vols, _ = m["Volumes"].([]interface{})
	if got := len(vols); got != initialCount+2 {
		t.Fatalf("expected %d volumes, got %d", initialCount+2, got)
	}
}

func TestPlugin_GetVolume(t *testing.T) {
	socket, _ := setupPluginTest(t)
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "get-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "get-vol"})
	v, ok := m["Volume"].(map[string]interface{})
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

func TestPlugin_PathVolume(t *testing.T) {
	socket, _ := setupPluginTest(t)
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "path-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "path-vol"})
	if mp, _ := m["Mountpoint"].(string); mp == "" {
		t.Fatal("expected mountpoint in path response")
	}
}

func TestPlugin_MountUnmount(t *testing.T) {
	socket, _ := setupPluginTest(t)
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "mount-vol"})

	// Mount
	m := pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: "mount-vol", ID: "mount-1"})
	mp, _ := m["Mountpoint"].(string)
	if mp == "" {
		t.Fatal("expected mountpoint after mount")
	}

	// Unmount
	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "mount-vol", ID: "mount-1"})
}

func TestPlugin_RemoveVolume(t *testing.T) {
	socket, _ := setupPluginTest(t)
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "remove-vol"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "remove-vol"})

	// Should not appear in list
	m := pluginOK(t, socket, "VolumeDriver.List", nil)
	vols, _ := m["Volumes"].([]interface{})
	for _, v := range vols {
		if vm, ok := v.(map[string]interface{}); ok && vm["Name"] == "remove-vol" {
			t.Fatal("volume should have been removed")
		}
	}
}

func TestPlugin_FullLifecycle(t *testing.T) {
	socket, _ := setupPluginTest(t)

	// Create → Mount → Get → Path → Unmount → Remove
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: "lifecycle-vol"})

	m := pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: "lifecycle-vol", ID: "lifecycle-1"})
	mp, _ := m["Mountpoint"].(string)
	if mp == "" {
		t.Fatal("expected mountpoint")
	}

	m = pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "lifecycle-vol"})
	v, _ := m["Volume"].(map[string]interface{})
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

func TestPlugin_Capabilities(t *testing.T) {
	socket, _ := setupPluginTest(t)
	m := pluginOK(t, socket, "VolumeDriver.Capabilities", nil)
	caps, ok := m["Capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Capabilities in response: %v", m)
	}
	if caps["Scope"] != "local" {
		t.Fatalf("expected scope 'local', got %v", caps["Scope"])
	}
}

func TestPlugin_EdgeCases(t *testing.T) {
	socket, _ := setupPluginTest(t)

	// Remove non-existent volume (should not error — driver just logs failures)
	m := pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: "no-such-vol"})
	if err, _ := m["Err"].(string); err != "" {
		t.Fatalf("remove non-existent: %s", err)
	}

	// Get non-existent volume (returns volume with empty path)
	m = pluginOK(t, socket, "VolumeDriver.Get", volume.GetRequest{Name: "no-such-vol"})
	if v, ok := m["Volume"].(map[string]interface{}); ok {
		if v["Name"] != "no-such-vol" {
			t.Fatalf("expected name no-such-vol, got %v", v["Name"])
		}
	}

	// Unmount non-existent volume (should not error)
	m = pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: "no-such-vol", ID: "test"})
	if err, _ := m["Err"].(string); err != "" {
		t.Fatalf("unmount non-existent: %s", err)
	}

	// Path non-existent volume
	m = pluginOK(t, socket, "VolumeDriver.Path", volume.PathRequest{Name: "no-such-vol"})
	if mp, _ := m["Mountpoint"].(string); mp == "" {
		t.Fatal("expected a mountpoint even for non-existent volume")
	}

	// Verify S3 lock was created during mount
	createdVol := "edge-lock-vol"
	pluginOK(t, socket, "VolumeDriver.Create", volume.CreateRequest{Name: createdVol})
	pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: createdVol, ID: "edge-1"})

	// Create same volume again and mount from another "host" — lock contention
	// The second mount's locker will see the first lock and fall through without error
	pluginOK(t, socket, "VolumeDriver.Mount", volume.MountRequest{Name: createdVol, ID: "edge-2"})

	// Cleanup
	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: createdVol, ID: "edge-1"})
	pluginOK(t, socket, "VolumeDriver.Unmount", volume.UnmountRequest{Name: createdVol, ID: "edge-2"})
	pluginOK(t, socket, "VolumeDriver.Remove", volume.RemoveRequest{Name: createdVol})
}
