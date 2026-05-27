//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/example/blt-volume-manager/restic"
	"github.com/example/blt-volume-manager/store"
	"github.com/example/blt-volume-manager/testutil"
)

func TestResticBackupRestoreWithGarage(t *testing.T) {
	garage := testutil.StartGarage(t)

	repoURL := "s3:" + garage.Endpoint + "/" + garage.BucketName
	t.Setenv("RESTIC_REPOSITORY", repoURL)
	t.Setenv("AWS_ACCESS_KEY_ID", garage.AccessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", garage.SecretKey)
	t.Setenv("RESTIC_PASSWORD", "test-password")
	t.Setenv("S3_FORCE_PATH_STYLE", "true")

	rm := restic.NewManager(repoURL)

	// Use real S3 via Garage for restore points (not FakeS3)
	realStore, err := store.NewS3Store(store.S3StoreOpts{
		AwsBucketName: garage.BucketName,
		S3Endpoint:    garage.Endpoint,
		Region:        "us-east-1",
	})
	if err != nil {
		t.Fatalf("create real s3 store: %v", err)
	}
	rm.SetS3Store(realStore)

	if err := rm.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Back up a file
	backupDir := t.TempDir()
	srcDir := filepath.Join(backupDir, "data")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("hello garage"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := rm.Backup(srcDir, "cold"); err != nil {
		t.Fatalf("backup: %v", err)
	}

	snaps, err := rm.ListSnapshots()
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	// Restore and verify
	restoreDir := t.TempDir()
	if err := rm.RestoreSnapshot(snaps[0].ShortID, restoreDir); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var found bool
	filepath.Walk(restoreDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fi.IsDir() && fi.Name() == "test.txt" {
			data, _ := os.ReadFile(path)
			if string(data) == "hello garage" {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Fatal("restored file with content 'hello garage' not found")
	}

	// Set and verify restore point — goes through real S3
	if err := rm.SetRestorePoint(snaps[0].ShortID, "test-vol"); err != nil {
		t.Fatalf("set restore point: %v", err)
	}
	rp, err := realStore.ReadRestorePoint("test-vol")
	if err != nil {
		t.Fatalf("read restore point: %v", err)
	}
	if rp == nil || rp.SnapshotID != snaps[0].ShortID {
		t.Fatalf("unexpected restore point: %+v", rp)
	}
	// Verify it was actually stored in Garage (not just in-memory)
	rp2, err := rm.FindRestorePoint("/var/lib/docker-volumes/volumes/test-vol")
	if err != nil {
		t.Fatalf("find restore point: %v", err)
	}
	if rp2 != snaps[0].ShortID {
		t.Fatalf("expected restore point %s, got %s", snaps[0].ShortID, rp2)
	}

	// Forget snapshot
	if err := rm.ForgetSnapshot(snaps[0].ShortID); err != nil {
		t.Fatalf("forget: %v", err)
	}
	snaps, err = rm.ListSnapshots()
	if err != nil {
		t.Fatalf("list after forget: %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after forget, got %d", len(snaps))
	}
}

func TestS3LocksWithGarage(t *testing.T) {
	garage := testutil.StartGarage(t)

	t.Setenv("AWS_ACCESS_KEY_ID", garage.AccessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", garage.SecretKey)
	t.Setenv("S3_FORCE_PATH_STYLE", "true")

	// Create a real S3 store via Garage
	store, err := store.NewS3Store(store.S3StoreOpts{
		AwsBucketName: garage.BucketName,
		S3Endpoint:    garage.Endpoint,
		Region:        "us-east-1",
	})
	if err != nil {
		t.Fatalf("create s3 store: %v", err)
	}

	// Write a lock proposal via real S3
	key := "blt-volume-manager/locks/test-vol/proposal.json"
	data := []byte(`{"name":"test","expiry_time":9999999999}`)
	if err := store.PutObject(key, data); err != nil {
		t.Fatalf("put lock object: %v", err)
	}

	// Read it back
	got, err := store.ReadObject(key)
	if err != nil {
		t.Fatalf("read lock object: %v", err)
	}
	if got == nil {
		t.Fatal("lock object not found")
	}

	// List with prefix
	objects, err := store.ListObjects("blt-volume-manager/locks/test-vol/")
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 lock object, got %d", len(objects))
	}

	// Delete
	if err := store.DeleteObject(key); err != nil {
		t.Fatalf("delete lock object: %v", err)
	}
	got, err = store.ReadObject(key)
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if got != nil {
		t.Fatal("lock object should have been deleted")
	}
}
