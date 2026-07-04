//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

func TestResticBackupRestoreWithGarage(t *testing.T) {
	t.Parallel()
	garage := StartGarage(t)

	repoURL := "s3:" + garage.Endpoint + "/" + garage.BucketName
	rm := restic.NewManager(repoURL)

	if err := rm.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	backupDir := t.TempDir()
	srcDir := filepath.Join(backupDir, "data")
	if err := os.MkdirAll(srcDir, app.DefaultDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("hello garage"), app.DefaultFilePerm); err != nil {
		t.Fatal(err)
	}
	if err := rm.Backup(srcDir, restic.BackupTagCold); err != nil {
		t.Fatalf("backup: %v", err)
	}

	snaps, err := rm.ListSnapshots()
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	restoreDir := t.TempDir()
	if err := rm.RestoreSnapshot(snaps[0].ShortID, restoreDir); err != nil {
		t.Fatalf("restore: %v", err)
	}
	var found bool
	filepath.Walk(restoreDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			t.Logf("walk error at %s: %v", path, err)
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

	if err := rm.ForgetSnapshots(snaps[0].ShortID); err != nil {
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

func TestS3OwnersWithGarage(t *testing.T) {
	t.Parallel()
	garage := StartGarage(t)

	be, err := backend.NewS3Client(backend.S3Config{
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		Region:     "us-east-1",
	})
	if err != nil {
		t.Fatalf("create s3 store: %v", err)
	}

	ownerStore := store.NewOwnerStore(be)
	key := "blt-volume-manager/owners/test-vol/proposal.json"
	data := []byte(`{"name":"test","expiry_time":9999999999}`)
	if err := be.PutObject(key, data); err != nil {
		t.Fatalf("put lock object: %v", err)
	}

	got, err := be.ReadObject(key)
	if err != nil {
		t.Fatalf("read lock object: %v", err)
	}
	if got == nil {
		t.Fatal("lock object not found")
	}

	objects, err := ownerStore.ListForVolume("test-vol")
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 lock object, got %d", len(objects))
	}

	if err := be.DeleteObject(key); err != nil {
		t.Fatalf("delete lock object: %v", err)
	}
	got, err = be.ReadObject(key)
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if got != nil {
		t.Fatal("lock object should have been deleted")
	}
}
