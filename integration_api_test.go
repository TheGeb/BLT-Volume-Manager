//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/appconfig"
	"github.com/TheGeb/BLT-Volume-Manager/internal/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/testutil"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web"
)

// setupAPITest starts Garage, creates a driver + web server backed by real S3,
// and returns the httptest.Server, the Garage ref, and a cleanup function.
func setupAPITest(t *testing.T) (*httptest.Server, *testutil.GarageServer) {
	t.Helper()

	garage := testutil.StartGarage(t)

	t.Setenv("AWS_ACCESS_KEY_ID", garage.AccessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", garage.SecretKey)
	t.Setenv("RESTIC_PASSWORD", "test-password")
	t.Setenv("S3_FORCE_PATH_STYLE", "true")

	t.Setenv("BLT_TEST_MODE", "1")

	resticBase := "s3:" + garage.Endpoint + "/" + garage.BucketName

	mux := http.NewServeMux()
	web.NewServer(appconfig.Config{
		ResticBase: resticBase,
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		S3Region:   "us-east-1",
	}).Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return ts, garage
}

func api(t *testing.T, ts *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func apiOK(t *testing.T, ts *httptest.Server, method, path string, body any) map[string]any {
	t.Helper()
	resp := api(t, ts, method, path, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected 200, got %d: %s", method, path, resp.StatusCode, string(b))
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return m
}

func apiArray(t *testing.T, ts *httptest.Server, method, path string) []any {
	t.Helper()
	resp := api(t, ts, method, path, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected 200, got %d: %s", method, path, resp.StatusCode, string(b))
	}
	var arr []any
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode array response: %v", err)
	}
	return arr
}

func apiErr(t *testing.T, ts *httptest.Server, method, path string, body any, wantCode int) map[string]any {
	t.Helper()
	resp := api(t, ts, method, path, body)
	defer resp.Body.Close()
	if resp.StatusCode != wantCode {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, wantCode, resp.StatusCode, string(b))
	}
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	return m
}

func TestAPI_Volumes(t *testing.T) {
	ts, _ := setupAPITest(t)

	// Empty list initially
	m := apiOK(t, ts, "GET", "/api/volumes", nil)
	vols, _ := m["volumes"].([]any)
	if len(vols) != 0 {
		t.Fatalf("expected 0 volumes, got %d", len(vols))
	}
}

func TestAPI_RepoInitAndStatus(t *testing.T) {
	ts, _ := setupAPITest(t)

	// Status before init — repo doesn't exist, should return 200 with status
	m := apiOK(t, ts, "GET", "/api/repo/status?volume=test-vol", nil)
	if m["initialized"] != false {
		t.Fatal("expected uninitialized repo")
	}

	// Init the repo
	m = apiOK(t, ts, "POST", "/api/repo/init?volume=test-vol", nil)
	if m["status"] != "repository initialized" {
		t.Fatalf("init failed: %v", m)
	}

	// Status after init
	m = apiOK(t, ts, "GET", "/api/repo/status?volume=test-vol", nil)
	if m["initialized"] != true {
		t.Fatal("expected initialized repo")
	}
}

func TestAPI_Snapshots(t *testing.T) {
	ts, _ := setupAPITest(t)

	// Create test volume via the test endpoint (requires "group/name" format)
	body := map[string]string{"name": "test-group/snap-vol"}
	m := apiOK(t, ts, "POST", "/api/dummy-volume", body)
	if m["status"] != "ok" {
		t.Fatalf("create test volume: %v", m)
	}

	// List snapshots
	m = apiOK(t, ts, "GET", "/api/snapshots?volume=test-group/snap-vol", nil)
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) == 0 {
		t.Fatal("expected at least 1 snapshot")
	}
}

func TestAPI_Stats(t *testing.T) {
	ts, _ := setupAPITest(t)

	// Stats for non-existent volume — should return 200 with empty stats
	m := apiOK(t, ts, "GET", "/api/stats?volume=nonexistent", nil)
	if m == nil {
		t.Fatal("expected stats object")
	}

	// Create volume + backup via test endpoint
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "test-group/stat-vol"})

	m = apiOK(t, ts, "GET", "/api/stats?volume=test-group/stat-vol", nil)
	snaps, _ := m["snapshots"].(map[string]any)
	repo, _ := m["repo"].(map[string]any)
	if snaps == nil || snaps["total"] == nil {
		t.Fatal("expected snapshot stats")
	}
	if repo == nil || repo["total_size"] == nil {
		t.Fatal("expected repo stats")
	}
}

func TestAPI_Locks(t *testing.T) {
	ts, _ := setupAPITest(t)

	// Create a lock
	m := apiOK(t, ts, "POST", "/api/volume/test-vol/locks", nil)
	if m["volume"] != "test-vol" {
		t.Fatalf("unexpected lock response: %v", m)
	}

	// Read lock status
	m = apiOK(t, ts, "GET", "/api/volume/test-vol/locks", nil)
	if m["locked"] != true {
		t.Fatal("expected locked volume")
	}

	// Delete locks
	m = apiOK(t, ts, "DELETE", "/api/volume/test-vol/locks", nil)
	if m["status"] != "locks deleted" {
		t.Fatalf("delete locks: %v", m)
	}

	// Verify unlocked
	m = apiOK(t, ts, "GET", "/api/volume/test-vol/locks", nil)
	if m["locked"] == true {
		t.Fatal("expected unlocked after deletion")
	}
}

func TestAPI_DeleteVolume(t *testing.T) {
	ts, _ := setupAPITest(t)

	// Create volume via test endpoint (requires group/name format)
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "test-group/del-vol"})

	// Delete volume — the locks handler expects simple names for the URL path
	// but we delete the volume created under test-group/del-vol
	m := apiOK(t, ts, "DELETE", "/api/volume/test-group/del-vol", nil)
	if !strings.Contains(fmt.Sprint(m["status"]), "deleted") {
		t.Fatalf("delete volume: %v", m)
	}
}

func TestAPI_EdgeCases(t *testing.T) {
	ts, _ := setupAPITest(t)

	// 404 for unknown paths
	apiErr(t, ts, "GET", "/api/nonexistent", nil, http.StatusNotFound)
	apiErr(t, ts, "POST", "/api/nonexistent", nil, http.StatusNotFound)

	// Missing volume param on endpoints that need it
	apiErr(t, ts, "POST", "/api/repo/init", nil, http.StatusBadRequest)
	apiErr(t, ts, "GET", "/api/repo/status", nil, http.StatusBadRequest)

	// Test create-volume with valid plain name (no "/" required)
	resp := api(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "badname"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for plain name, got %d", resp.StatusCode)
	}

	// Wrong method on /api/volumes
	resp = api(t, ts, "PUT", "/api/volumes", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected method-not-allowed for PUT /api/volumes, got %d", resp.StatusCode)
	}
}

func TestAPI_S3StoreThroughGarage(t *testing.T) {
	// Verify that stores created by the API handlers actually persist data in Garage
	ts, garage := setupAPITest(t)

	// Use the web server's lock endpoint to write data to S3
	apiOK(t, ts, "POST", "/api/volume/persist-vol/locks", nil)

	// Read it back via a direct S3 SDK call to Garage (bypassing the API)
	directStore, err := store.NewS3Store(store.S3StoreConfig{
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		Region:     "us-east-1",
	})
	if err != nil {
		t.Fatalf("create direct store: %v", err)
	}

	// The lock created above should be visible via a fresh store
	objects, err := directStore.ListObjects(store.LockPrefix + "persist-vol/")
	if err != nil {
		t.Fatalf("list lock objects: %v", err)
	}
	if len(objects) == 0 {
		t.Fatal("expected lock objects in Garage, found none")
	}
}

func TestAPI_SnapshotViewFallbackHash(t *testing.T) {
	ts, _ := setupAPITest(t)

	volName := "test-group/fallback-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	m := apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snapshots, _ := m["snapshots"].([]any)
	if len(snapshots) < 1 {
		t.Fatal("expected at least 1 snapshot")
	}

	snap := snapshots[0].(map[string]any)
	realID := snap["id"].(string)
	fallbackHash, _ := snap["fallbackHash"].(string)
	if fallbackHash == "" {
		t.Fatal("expected fallbackHash in snapshot response")
	}

	nodes := apiArray(t, ts, "GET",
		"/api/snapshot-view/fake-nonexistent-id/ls?volume="+volName+"&fallbackHash="+fallbackHash)
	if len(nodes) == 0 {
		t.Fatal("expected file nodes from fallback resolution")
	}

	nodesReal := apiArray(t, ts, "GET",
		"/api/snapshot-view/"+realID+"/ls?volume="+volName)
	if len(nodesReal) != len(nodes) {
		t.Fatalf("fallback and direct ls should return same count; got %d vs %d", len(nodes), len(nodesReal))
	}

	resp := api(t, ts, "GET",
		"/api/snapshot-view/fake-id/dump?volume="+volName+"&path=/readme.txt&fallbackHash="+fallbackHash, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("dump with fallback; expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	resp2 := api(t, ts, "GET",
		"/api/snapshot-view/fake-id/ls?volume="+volName+"&fallbackHash=deadbeef1234", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Fatal("expected error with invalid fallbackHash, got 200")
	}
}

func TestAPI_SnapshotViewDiffFallbackHash(t *testing.T) {
	ts, _ := setupAPITest(t)

	volName := "test-group/diff-fallback-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	m := apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snapshots, _ := m["snapshots"].([]any)
	if len(snapshots) < 2 {
		t.Fatalf("expected at least 2 snapshots for diff, got %d", len(snapshots))
	}

	snapA := snapshots[0].(map[string]any)
	snapB := snapshots[1].(map[string]any)
	_ = snapA["id"].(string) // direct ID not needed, tested via fallback
	idB := snapB["id"].(string)
	hashA, _ := snapA["fallbackHash"].(string)
	hashB, _ := snapB["fallbackHash"].(string)

	mFallback := apiOK(t, ts, "GET",
		"/api/snapshot-view/fake-a/diff/fake-b?volume="+volName+"&fallbackHash="+hashA+"&diffFallbackHash="+hashB, nil)
	if _, ok := mFallback["change_sets"]; !ok {
		t.Fatal("expected change_sets in fallback diff response")
	}

	mPartial := apiOK(t, ts, "GET",
		"/api/snapshot-view/fake-a/diff/"+idB+"?volume="+volName+"&fallbackHash="+hashA, nil)
	if _, ok := mPartial["change_sets"]; !ok {
		t.Fatal("expected change_sets in partial fallback diff response")
	}

	respErr := api(t, ts, "GET",
		"/api/snapshot-view/dead1/diff/beef2?volume="+volName, nil)
	defer respErr.Body.Close()
	if respErr.StatusCode < 400 {
		t.Fatalf("expected 4xx error for diff with nonexistent IDs, got %d", respErr.StatusCode)
	}
}
