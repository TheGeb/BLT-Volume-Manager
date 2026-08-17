//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/migrate"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startIsolatedEtcd starts a dedicated etcd container so migration tests do
// not share state with the (shared) etcd used by the other integration tests.
func startIsolatedEtcd(t *testing.T) *EtcdServer {
	t.Helper()
	ctx := context.Background()
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
		t.Fatalf("start isolated etcd: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	port, err := container.MappedPort(ctx, "2379/tcp")
	if err != nil {
		t.Fatalf("etcd mapped port: %v", err)
	}
	return &EtcdServer{Endpoint: fmt.Sprintf("http://localhost:%s", port.Port())}
}

func s3MetaConfig(garage *GarageServer) cfg.Config {
	return cfg.Config{
		MetadataBackend:  "s3",
		S3Bucket:         garage.BucketName,
		S3Endpoint:       garage.Endpoint,
		S3Region:         "us-east-1",
		S3ForcePathStyle: true,
	}
}

func etcdMetaConfig(etcd *EtcdServer) cfg.Config {
	return cfg.Config{
		MetadataBackend: "etcd",
		EtcdEndpoints:   []string{etcd.Endpoint},
	}
}

func openMeta(t *testing.T, c cfg.Config) store.MetadataStore {
	t.Helper()
	b, err := cfg.OpenMetadataBackend(c)
	if err != nil {
		t.Fatalf("open metadata backend: %v", err)
	}
	t.Cleanup(func() {
		if cl, ok := b.(interface{ Close() error }); ok {
			_ = cl.Close()
		}
	})
	return b
}

// seedMetadata writes one of each metadata type for the volume "mig-vol".
func seedMetadata(t *testing.T, b store.MetadataStore) {
	t.Helper()
	ctx := context.Background()

	if _, err := store.WriteOwnerLock(ctx, b, "mig-vol", "host-1", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatalf("seed owner lock: %v", err)
	}
	if err := store.NewRegisteredVolumeStore(b).Register(ctx, "mig-vol"); err != nil {
		t.Fatalf("seed registered volume: %v", err)
	}
	if err := store.NewVersionStore(b).WriteCounter(ctx, "mig-vol", store.VersionCounter{Major: 3, Minor: 2}); err != nil {
		t.Fatalf("seed version counter: %v", err)
	}
	if err := store.NewRestorePointStore(b).Set(ctx, "mig-vol", "snap-123"); err != nil {
		t.Fatalf("seed restore point: %v", err)
	}
}

// verifyMigrated asserts all seeded metadata reached the target backend with
// the owner lock refreshed to a future expiry.
func verifyMigrated(t *testing.T, b store.MetadataStore) {
	t.Helper()
	ctx := context.Background()

	vols, err := store.NewRegisteredVolumeStore(b).List(ctx)
	if err != nil {
		t.Fatalf("list registered volumes: %v", err)
	}
	if len(vols) != 1 || vols[0] != "mig-vol" {
		t.Fatalf("expected registered volumes [mig-vol], got %v", vols)
	}

	counter, err := store.NewVersionStore(b).ReadCounter(ctx, "mig-vol")
	if err != nil {
		t.Fatalf("read version counter: %v", err)
	}
	if counter.Major != 3 || counter.Minor != 2 {
		t.Fatalf("version counter = %+v, want 3.2", counter)
	}

	snapID, err := store.NewRestorePointStore(b).FindByName(ctx, "mig-vol")
	if err != nil {
		t.Fatalf("read restore point: %v", err)
	}
	if snapID != "snap-123" {
		t.Fatalf("restore point = %q, want snap-123", snapID)
	}

	vo, err := store.NewOwnerStore(b).FindForVolume(ctx, "mig-vol")
	if err != nil {
		t.Fatalf("find owner: %v", err)
	}
	if vo.Owner != "host-1" {
		t.Fatalf("owner = %q, want host-1", vo.Owner)
	}
	if vo.Expiry == 0 {
		t.Error("expected a time-limited lock with a future expiry")
	} else if vo.Expiry <= time.Now().Unix() {
		t.Errorf("migrated lock expiry %d should be in the future", vo.Expiry)
	}
}

func TestMigrateBackends_S3ToEtcd(t *testing.T) {
	setupLogCapture(t)

	garage := StartGarage(t)
	etcd := startIsolatedEtcd(t)

	src := openMeta(t, s3MetaConfig(garage))
	seedMetadata(t, src)

	res, err := migrate.MigrateBackends(context.Background(), s3MetaConfig(garage), etcdMetaConfig(etcd), false)
	if err != nil {
		t.Fatalf("migrate s3->etcd: %v", err)
	}
	if res.OwnerLocks != 1 || res.Keys != 3 || res.DryRun {
		t.Fatalf("result = %+v, want 1 owner lock, 3 keys, non dry-run", res)
	}

	verifyMigrated(t, openMeta(t, etcdMetaConfig(etcd)))
}

func TestMigrateBackends_EtcdToS3(t *testing.T) {
	setupLogCapture(t)

	garage := StartGarage(t)
	etcd := startIsolatedEtcd(t)

	src := openMeta(t, etcdMetaConfig(etcd))
	seedMetadata(t, src)

	res, err := migrate.MigrateBackends(context.Background(), etcdMetaConfig(etcd), s3MetaConfig(garage), false)
	if err != nil {
		t.Fatalf("migrate etcd->s3: %v", err)
	}
	if res.OwnerLocks != 1 || res.Keys != 3 {
		t.Fatalf("result = %+v, want 1 owner lock and 3 keys", res)
	}

	verifyMigrated(t, openMeta(t, s3MetaConfig(garage)))
}

func TestAPIMigrate_Endpoint(t *testing.T) {
	ts, garage := setupAPITest(t, "s3")
	etcd := startIsolatedEtcd(t)

	seedMetadata(t, openMeta(t, s3MetaConfig(garage)))

	m := apiOK(t, ts, "POST", "/api/migrate", map[string]any{
		"dry_run": false,
		"from": map[string]any{
			"type":             "s3",
			"bucket":           garage.BucketName,
			"endpoint":         garage.Endpoint,
			"region":           "us-east-1",
			"force_path_style": true,
		},
		"to": map[string]any{
			"type":           "etcd",
			"etcd_endpoints": []string{etcd.Endpoint},
		},
	})
	if m["owner_locks"] != float64(1) || m["keys"] != float64(3) || m["dry_run"] != false {
		t.Fatalf("unexpected migration result: %v", m)
	}

	verifyMigrated(t, openMeta(t, etcdMetaConfig(etcd)))
}

func TestAPIMigrate_DryRunWritesNothing(t *testing.T) {
	ts, garage := setupAPITest(t, "s3")
	etcd := startIsolatedEtcd(t)

	seedMetadata(t, openMeta(t, s3MetaConfig(garage)))

	m := apiOK(t, ts, "POST", "/api/migrate", map[string]any{
		"dry_run": true,
		"from": map[string]any{
			"type":     "s3",
			"bucket":   garage.BucketName,
			"endpoint": garage.Endpoint,
			"region":   "us-east-1",
		},
		"to": map[string]any{
			"type":           "etcd",
			"etcd_endpoints": []string{etcd.Endpoint},
		},
	})
	if m["dry_run"] != true {
		t.Fatalf("expected dry_run=true, got %v", m)
	}
	if m["owner_locks"] != float64(1) || m["keys"] != float64(3) {
		t.Fatalf("dry run should still report counts, got %v", m)
	}

	dst := openMeta(t, etcdMetaConfig(etcd))
	vols, err := store.NewRegisteredVolumeStore(dst).List(context.Background())
	if err != nil {
		t.Fatalf("list target: %v", err)
	}
	if len(vols) != 0 {
		t.Fatalf("dry run must not write to the target, found %v", vols)
	}
}

func TestAPIMigrate_Validation(t *testing.T) {
	ts, _ := setupAPITest(t, "s3")

	apiErr(t, ts, "POST", "/api/migrate", map[string]any{
		"from": map[string]any{"type": "redis"},
		"to":   map[string]any{"type": "s3", "bucket": "meta"},
	}, http.StatusBadRequest)

	apiErr(t, ts, "POST", "/api/migrate", map[string]any{
		"from": map[string]any{"type": "s3"},
		"to":   map[string]any{"type": "etcd", "etcd_endpoints": []string{"http://x:2379"}},
	}, http.StatusBadRequest)
}
