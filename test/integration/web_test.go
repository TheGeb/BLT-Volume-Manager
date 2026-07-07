//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func DoRequest(t *testing.T, baseURL, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, baseURL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func DoOK(t *testing.T, baseURL, method, path string, body any) map[string]any {
	t.Helper()
	resp := DoRequest(t, baseURL, method, path, body)
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

func DoArray(t *testing.T, baseURL, method, path string) []any {
	t.Helper()
	resp := DoRequest(t, baseURL, method, path, nil)
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

func DoErr(t *testing.T, baseURL, method, path string, body any, wantCode int) map[string]any {
	t.Helper()
	resp := DoRequest(t, baseURL, method, path, body)
	if resp.StatusCode != wantCode {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: expected %d, got %d: %s", method, path, wantCode, resp.StatusCode, string(b))
	}
	var m map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&m)
	return m
}

func setupAPITest(t *testing.T, backendType string) (*httptest.Server, *GarageServer) {
	t.Helper()

	etcd := StartEtcd(t)
	garage := StartGarage(t)

	resticBase := "s3:" + garage.Endpoint + "/" + garage.BucketName

	conf := cfg.Config{
		ResticBase: resticBase,
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		S3Region:   "us-east-1",
	}
	if backendType == "etcd" {
		conf.MetadataBackend = "etcd"
		conf.EtcdEndpoints = []string{etcd.Endpoint}
	}

	mux := http.NewServeMux()
	srv := server.New(conf)
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

func testAPIVolumes(t *testing.T, ts *httptest.Server) {
	m := apiOK(t, ts, "GET", "/api/volumes", nil)
	vols, _ := m["volumes"].([]any)
	if len(vols) != 0 {
		t.Fatalf("expected 0 volumes, got %d", len(vols))
	}
}

func testAPIRepoInitAndStatus(t *testing.T, ts *httptest.Server) {
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

func testAPISnapshots(t *testing.T, ts *httptest.Server) {
	volName := "test-group/snap-vol"
	body := map[string]string{"name": volName}
	m := apiOK(t, ts, "POST", "/api/dummy-volume", body)
	if m["status"] != "ok" {
		t.Fatalf("create test volume: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) == 0 {
		t.Fatal("expected at least 1 snapshot")
	}
}

func testAPIStats(t *testing.T, ts *httptest.Server) {
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "test-group/stat-vol"})

	m := apiOK(t, ts, "GET", "/api/stats?volume=test-group/stat-vol", nil)
	repo, _ := m["repo"].(map[string]any)
	if repo == nil || repo["total_size"] == nil {
		t.Fatal("expected repo stats")
	}
}

func testAPIOwners(t *testing.T, ts *httptest.Server) {
	volName := "test-group/own-vol"
	m := apiOK(t, ts, "POST", "/api/volume/"+volName+"/owners", map[string]string{"owner": "test-owner"})
	if m["volume"] != volName {
		t.Fatalf("unexpected owner response: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/volume/"+volName+"/owners", nil)
	if m["owner"] == "" {
		t.Fatal("expected owned volume")
	}

	m = apiOK(t, ts, "DELETE", "/api/volume/"+volName+"/owners", nil)
	if m["status"] != "owners deleted" {
		t.Fatalf("delete owners: %v", m)
	}

	m = apiOK(t, ts, "GET", "/api/volume/"+volName+"/owners", nil)
	if m["owner"] != "" {
		t.Fatal("expected unlocked after deletion")
	}
}

func testAPIDeleteVolume(t *testing.T, ts *httptest.Server) {
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "test-group/del-vol"})

	m := apiOK(t, ts, "DELETE", "/api/volume/test-group/del-vol", nil)
	if !strings.Contains(fmt.Sprint(m["status"]), "deleted") {
		t.Fatalf("delete volume: %v", m)
	}
}

func testAPIEdgeCases(t *testing.T, ts *httptest.Server) {
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

func testAPISnapshotViewFallbackHash(t *testing.T, ts *httptest.Server) {
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
}

func testAPIFallbackHashComprehensive(t *testing.T, ts *httptest.Server) {
	volName := "test-group/fb-comp-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	m := apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snapshots, _ := m["snapshots"].([]any)
	if len(snapshots) < 1 {
		t.Fatal("expected at least 1 snapshot")
	}

	hashes := make(map[string]string)
	for i, s := range snapshots {
		snap := s.(map[string]any)
		id, _ := snap["id"].(string)
		fh, _ := snap["fallbackHash"].(string)
		if fh == "" {
			t.Fatalf("snapshot %d (%s) has no fallbackHash", i, id)
		}
		if existing, dup := hashes[fh]; dup {
			t.Fatalf("duplicate fallbackHash %q for %s and %s", fh, existing, id)
		}
		hashes[fh] = id
	}

	first := snapshots[0].(map[string]any)
	realID := first["id"].(string)
	fh := first["fallbackHash"].(string)

	// Dump via direct ID
	resp := DoRequest(t, ts.URL, "GET",
		"/api/snapshot-view/"+realID+"/dump?volume="+volName+"&path=/readme.txt", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("direct dump: expected 200, got %d", resp.StatusCode)
	}
	directContent, _ := io.ReadAll(resp.Body)

	// Dump via fake ID + fallbackHash — must match
	resp2 := DoRequest(t, ts.URL, "GET",
		"/api/snapshot-view/fake-id/dump?volume="+volName+"&path=/readme.txt&fallbackHash="+fh, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("fallback dump: expected 200, got %d: %s", resp2.StatusCode, string(b))
	}
	fallbackContent, _ := io.ReadAll(resp2.Body)
	if string(directContent) != string(fallbackContent) {
		t.Fatalf("content mismatch:\ndirect:  %q\nfallback: %q", string(directContent), string(fallbackContent))
	}

	// Invalid fallbackHash returns 400
	resp3 := DoRequest(t, ts.URL, "GET",
		"/api/snapshot-view/fake-id/ls?volume="+volName+"&fallbackHash=deadbeef", nil)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp3.Body)
		t.Fatalf("bad fallbackHash: expected 400, got %d: %s", resp3.StatusCode, string(b))
	}

	// ls returns same file set via fallback hash
	nodesFallback := apiArray(t, ts, "GET",
		"/api/snapshot-view/fake-id/ls?volume="+volName+"&fallbackHash="+fh)
	nodesDirect := apiArray(t, ts, "GET",
		"/api/snapshot-view/"+realID+"/ls?volume="+volName)
	if len(nodesFallback) != len(nodesDirect) {
		t.Fatalf("ls node count: fallback=%d direct=%d", len(nodesFallback), len(nodesDirect))
	}
	for j, nf := range nodesFallback {
		nfMap := nf.(map[string]any)
		ndMap := nodesDirect[j].(map[string]any)
		if nfMap["name"] != ndMap["name"] {
			t.Fatalf("node %d name mismatch: fallback=%q direct=%q", j, nfMap["name"], ndMap["name"])
		}
	}
}

func testAPISnapshotViewDiffFallbackHash(t *testing.T, ts *httptest.Server) {
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

func testAPIHealth(t *testing.T, ts *httptest.Server) {
	resp := DoRequest(t, ts.URL, "GET", "/api/health", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func testAPIRepoCheck(t *testing.T, ts *httptest.Server) {
	apiOK(t, ts, "POST", "/api/repo/init?volume=check-vol", nil)

	m := apiOK(t, ts, "POST", "/api/repo/check?volume=check-vol", nil)
	if m["status"] != "Check completed, repository is healthy." {
		t.Fatalf("check failed: %v", m)
	}
}

func testAPIRepoRepair(t *testing.T, ts *httptest.Server) {
	apiOK(t, ts, "POST", "/api/repo/init?volume=repair-vol", nil)

	m := apiOK(t, ts, "POST", "/api/repo/repair?volume=repair-vol", nil)
	if _, ok := m["status"]; !ok {
		t.Fatalf("repair missing status: %v", m)
	}
}

func testAPISnapshotHosts(t *testing.T, ts *httptest.Server) {
	volName := "test-group/hosts-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	hosts := apiArray(t, ts, "GET", "/api/snapshots/hosts?volume="+volName)
	if len(hosts) == 0 {
		t.Fatal("expected at least 1 host")
	}
}

func testAPISnapshotDeleteBatch(t *testing.T, ts *httptest.Server) {
	volName := "test-group/batch-del-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	m := apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) == 0 {
		t.Fatal("expected snapshots")
	}

	ids := make([]string, len(snaps))
	for i, s := range snaps {
		ids[i] = s.(map[string]any)["id"].(string)
	}

	m = apiOK(t, ts, "POST", "/api/snapshots/delete-batch", map[string]any{
		"volume": volName,
		"ids":    ids,
	})
	if _, ok := m["deleted"]; !ok {
		t.Fatalf("delete-batch missing deleted count: %v", m)
	}
}

func testAPIDummySnapshot(t *testing.T, ts *httptest.Server) {
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": "test-group/ds-vol"})

	m := apiOK(t, ts, "POST", "/api/dummy-snapshot", map[string]string{"volume": "test-group/ds-vol"})
	if m["status"] != "ok" {
		t.Fatalf("dummy-snapshot failed: %v", m)
	}
}

func testAPIDevMode(t *testing.T, ts *httptest.Server) {
	m := apiOK(t, ts, "GET", "/api/dev-mode", nil)
	if m["enabled"] != true {
		t.Fatal("expected dev mode enabled")
	}
}

func testAPIVolumeOwnersList(t *testing.T, ts *httptest.Server) {
	apiOK(t, ts, "POST", "/api/volume/list-own-vol/owners", map[string]string{"owner": "test-owner"})

	resp := DoRequest(t, ts.URL, "GET", "/api/volumes/owners", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}
}

func testAPIRestorePoint(t *testing.T, ts *httptest.Server) {
	volName := "test-group/rp-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	m := apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) == 0 {
		t.Fatal("expected snapshots")
	}
	snapID := snaps[0].(map[string]any)["id"].(string)

	apiOK(t, ts, "PUT", "/api/volume/"+volName+"/restore-point", map[string]string{
		"snapshot_id": snapID,
	})

	m = apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	if m["restorePointID"] != snapID {
		t.Fatalf("expected restorePointID %q, got %q", snapID, m["restorePointID"])
	}

	apiOK(t, ts, "DELETE", "/api/volume/"+volName+"/restore-point", nil)

	m = apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	if m["restorePointID"] != "" {
		t.Fatalf("expected empty restorePointID after delete, got %q", m["restorePointID"])
	}
}

func testAPISnapshotSizes(t *testing.T, ts *httptest.Server) {
	volName := "test-group/sizes-vol"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": volName})

	m := apiOK(t, ts, "GET", "/api/snapshots?volume="+volName, nil)
	snaps, _ := m["snapshots"].([]any)
	if len(snaps) == 0 {
		t.Fatal("expected snapshots")
	}

	ids := make([]string, len(snaps))
	for i, s := range snaps {
		ids[i] = s.(map[string]any)["id"].(string)
	}

	m = apiOK(t, ts, "POST", "/api/snapshot/sizes", map[string]any{
		"volume": volName,
		"ids":    ids,
	})
	for _, id := range ids {
		size, ok := m[id].(float64)
		if !ok || size <= 0 {
			t.Fatalf("expected positive size for %s, got %v", id, m[id])
		}
	}
}

func testAPIVolumeCopy(t *testing.T, ts *httptest.Server) {
	src := "test-group/copy-src"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": src})

	m := apiOK(t, ts, "POST", "/api/volume/"+src+"/copy", map[string]any{
		"target":           "test-group/copy-dst",
		"preserve_history": true,
	})
	if _, ok := m["status"]; !ok {
		t.Fatalf("copy missing status: %v", m)
	}
}

func testAPIVolumeRename(t *testing.T, ts *httptest.Server) {
	src := "test-group/rename-src"
	apiOK(t, ts, "POST", "/api/dummy-volume", map[string]string{"name": src})

	m := apiOK(t, ts, "POST", "/api/volume/"+src+"/rename", map[string]string{
		"target": "test-group/rename-dst",
	})
	if _, ok := m["status"]; !ok {
		t.Fatalf("rename missing status: %v", m)
	}
}

func testAPIStatsRefresh(t *testing.T, ts *httptest.Server) {
	m := apiOK(t, ts, "POST", "/api/stats/refresh", nil)
	if m == nil {
		t.Fatal("expected stats object after refresh")
	}
}

func TestAPI(t *testing.T) {
	setupLogCapture(t)

	for _, backendType := range []string{"s3", "etcd"} {
		backendType := backendType
		t.Run(backendType, func(t *testing.T) {
			ts, _ := setupAPITest(t, backendType)

			t.Run("Volumes", func(t *testing.T) { t.Parallel(); testAPIVolumes(t, ts) })
			t.Run("RepoInitAndStatus", func(t *testing.T) { t.Parallel(); testAPIRepoInitAndStatus(t, ts) })
			t.Run("Snapshots", func(t *testing.T) { t.Parallel(); testAPISnapshots(t, ts) })
			t.Run("Stats", func(t *testing.T) { t.Parallel(); testAPIStats(t, ts) })
			t.Run("Owners", func(t *testing.T) { t.Parallel(); testAPIOwners(t, ts) })
			t.Run("DeleteVolume", func(t *testing.T) { t.Parallel(); testAPIDeleteVolume(t, ts) })
			t.Run("EdgeCases", func(t *testing.T) { t.Parallel(); testAPIEdgeCases(t, ts) })
			t.Run("SnapshotViewFallbackHash", func(t *testing.T) { t.Parallel(); testAPISnapshotViewFallbackHash(t, ts) })
			t.Run("FallbackHashComprehensive", func(t *testing.T) { t.Parallel(); testAPIFallbackHashComprehensive(t, ts) })
			t.Run("SnapshotViewDiffFallbackHash", func(t *testing.T) { t.Parallel(); testAPISnapshotViewDiffFallbackHash(t, ts) })
			t.Run("Health", func(t *testing.T) { t.Parallel(); testAPIHealth(t, ts) })
			t.Run("RestorePoint", func(t *testing.T) { t.Parallel(); testAPIRestorePoint(t, ts) })
			t.Run("SnapshotSizes", func(t *testing.T) { t.Parallel(); testAPISnapshotSizes(t, ts) })
			t.Run("VolumeCopy", func(t *testing.T) { t.Parallel(); testAPIVolumeCopy(t, ts) })
			t.Run("VolumeRename", func(t *testing.T) { t.Parallel(); testAPIVolumeRename(t, ts) })
			t.Run("DevMode", func(t *testing.T) { t.Parallel(); testAPIDevMode(t, ts) })
			t.Run("StatsRefresh", func(t *testing.T) { t.Parallel(); testAPIStatsRefresh(t, ts) })
			t.Run("RepoCheck", func(t *testing.T) { t.Parallel(); testAPIRepoCheck(t, ts) })
			t.Run("RepoRepair", func(t *testing.T) { t.Parallel(); testAPIRepoRepair(t, ts) })
			t.Run("SnapshotHosts", func(t *testing.T) { t.Parallel(); testAPISnapshotHosts(t, ts) })
			t.Run("SnapshotDeleteBatch", func(t *testing.T) { t.Parallel(); testAPISnapshotDeleteBatch(t, ts) })
			t.Run("DummySnapshot", func(t *testing.T) { t.Parallel(); testAPIDummySnapshot(t, ts) })
			t.Run("VolumeOwnersList", func(t *testing.T) { t.Parallel(); testAPIVolumeOwnersList(t, ts) })
		})
	}
}
