//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/s3"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func setupAPITest(t *testing.T) (*httptest.Server, *GarageServer) {
	t.Helper()

	garage := StartGarage(t)

	resticBase := "s3:" + garage.Endpoint + "/" + garage.BucketName

	mux := http.NewServeMux()
	srv := server.New(cfg.Config{
		ResticBase: resticBase,
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		S3Region:   "us-east-1",
	})
	if err := web.Register(srv, mux); err != nil {
		t.Fatalf("register web routes failed: %v", err)
	}
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return ts, garage
}

func apiOK(t *testing.T, ts *httptest.Server, method, path string, body any) map[string]any {
	return DoOK(t, ts.URL, method, path, body)
}

func apiArray(t *testing.T, ts *httptest.Server, method, path string) []any {
	return DoArray(t, ts.URL, method, path)
}

func apiErr(t *testing.T, ts *httptest.Server, method, path string, body any, wantCode int) map[string]any {
	return DoErr(t, ts.URL, method, path, body, wantCode)
}

func TestAPI_Volumes(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	m := apiOK(t, ts, "GET", "/api/volumes", nil)
	vols, _ := m["volumes"].([]any)
	if len(vols) != 0 {
		t.Fatalf("expected 0 volumes, got %d", len(vols))
	}
}

func TestAPI_RepoInitAndStatus(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	m := apiOK(t, ts, "GET", "/api/repo/status?volume=test-vol", nil)
	if m["initialized"] != false {
		t.Fatal("expected uninitialized repo")
	}

	m = apiOK(t, ts, "POST", "/api/repo/init?volume=test-vol", nil)
	if m["status"] != "repository initialized" {
		t.Fatalf("init failed: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/repo/status?volume=test-vol", nil)
	if m["initialized"] != true {
		t.Fatal("expected initialized repo")
	}
}

func TestAPI_Snapshots(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	body := map[string]string{"name": "test-group/snap-vol"}
	m := apiOK(t, ts, "POST", "/api/dummy-volume", body)
	if m["status"] != "ok" {
		t.Fatalf("create test volume: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/snapshots?volume=test-group/snap-vol", nil)
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) == 0 {
		t.Fatal("expected at least 1 snapshot")
	}
}

func TestAPI_Stats(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	m := apiOK(t, ts, "GET", "/api/stats?volume=nonexistent", nil)
	if m == nil {
		t.Fatal("expected stats object")
	}

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

func TestAPI_Owners(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	m := apiOK(t, ts, "POST", "/api/volume/test-vol/owners", nil)
	if m["volume"] != "test-vol" {
		t.Fatalf("unexpected owner response: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/volume/test-vol/owners", nil)
	if m["owned"] != true {
		t.Fatal("expected owned volume")
	}

	m = apiOK(t, ts, "DELETE", "/api/volume/test-vol/owners", nil)
	if m["status"] != "owners deleted" {
		t.Fatalf("delete owners: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/volume/test-vol/owners", nil)
	if m["owned"] == true {
		t.Fatal("expected unlocked after deletion")
	}
}

func TestAPI_DeleteVolume(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "test-group/del-vol"})

	m := apiOK(t, ts, "DELETE", "/api/volume/test-group/del-vol", nil)
	if !strings.Contains(fmt.Sprint(m["status"]), "deleted") {
		t.Fatalf("delete volume: %v", m)
	}
}

func TestAPI_EdgeCases(t *testing.T) {
	t.Parallel()
	ts, _ := setupAPITest(t)

	apiErr(t, ts, "GET", "/api/nonexistent", nil, http.StatusNotFound)
	apiErr(t, ts, "POST", "/api/nonexistent", nil, http.StatusNotFound)

	apiErr(t, ts, "POST", "/api/repo/init", nil, http.StatusBadRequest)
	apiErr(t, ts, "GET", "/api/repo/status", nil, http.StatusBadRequest)

	resp := DoRequest(t, ts.URL, "POST", "/api/dummy-volume", map[string]string{"name": "badname"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for plain name, got %d", resp.StatusCode)
	}

	resp = DoRequest(t, ts.URL, "PUT", "/api/volumes", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected method-not-allowed for PUT /api/volumes, got %d", resp.StatusCode)
	}
}

func TestAPI_S3StoreThroughGarage(t *testing.T) {
	t.Parallel()
	ts, garage := setupAPITest(t)

	apiOK(t, ts, "POST", "/api/volume/persist-vol/owners", nil)

	directStore, err := s3.NewClient(s3.Config{
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		Region:     "us-east-1",
	})
	if err != nil {
		t.Fatalf("create direct store: %v", err)
	}

	objects, err := directStore.ListObjects(metadata.OwnerPrefix + "persist-vol/")
	if err != nil {
		t.Fatalf("list owner objects: %v", err)
	}
	if len(objects) == 0 {
		t.Fatal("expected owner objects in Garage, found none")
	}
}

func TestAPI_SnapshotViewFallbackHash(t *testing.T) {
	t.Parallel()
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

	resp := DoRequest(t, ts.URL, "GET",
		"/api/snapshot-view/fake-id/dump?volume="+volName+"&path=/readme.txt&fallbackHash="+fallbackHash, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("dump with fallback; expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	resp2 := DoRequest(t, ts.URL, "GET",
		"/api/snapshot-view/fake-id/ls?volume="+volName+"&fallbackHash=deadbeef1234", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Fatal("expected error with invalid fallbackHash, got 200")
	}
}

func TestAPI_SnapshotViewDiffFallbackHash(t *testing.T) {
	t.Parallel()
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
	_ = snapA["id"].(string)
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

	respErr := DoRequest(t, ts.URL, "GET",
		"/api/snapshot-view/dead1/diff/beef2?volume="+volName, nil)
	defer respErr.Body.Close()
	if respErr.StatusCode < 400 {
		t.Fatalf("expected 4xx error for diff with nonexistent IDs, got %d", respErr.StatusCode)
	}
}
