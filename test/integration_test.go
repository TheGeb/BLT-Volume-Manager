//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata/s3"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
)

func TestResticBackupRestoreWithGarage(t *testing.T) {
	garage := StartGarage(t)

	repoURL := "s3:" + garage.Endpoint + "/" + garage.BucketName
	t.Setenv("RESTIC_REPOSITORY", repoURL)
	t.Setenv("AWS_ACCESS_KEY_ID", garage.AccessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", garage.SecretKey)
	t.Setenv("RESTIC_PASSWORD", "test-password")
	t.Setenv("S3_FORCE_PATH_STYLE", "true")

	rm := restic.NewManager(repoURL)

	s3Client, err := s3.NewClient(s3.Config{
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		Region:     "us-east-1",
	})
	if err != nil {
		t.Fatalf("create real s3 store: %v", err)
	}
	realStore := metadata.New(s3Client)

	if err := rm.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	backupDir := t.TempDir()
	srcDir := filepath.Join(backupDir, "data")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("hello garage"), 0o644); err != nil {
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

	if err := metadata.SetRestorePoint(realStore, "test-vol", snaps[0].ShortID); err != nil {
		t.Fatalf("set restore point: %v", err)
	}
	rp, err := realStore.ReadRestorePoint("test-vol")
	if err != nil {
		t.Fatalf("read restore point: %v", err)
	}
	if rp == nil || rp.SnapshotID != snaps[0].ShortID {
		t.Fatalf("unexpected restore point: %+v", rp)
	}
	targetVolume := driver.VolumeNameFromPath("/var/lib/docker-volumes/volumes/test-vol")
	rp2, err := metadata.FindRestorePointByName(realStore, targetVolume)
	if err != nil {
		t.Fatalf("find restore point: %v", err)
	}
	if rp2 != snaps[0].ShortID {
		t.Fatalf("expected restore point %s, got %s", snaps[0].ShortID, rp2)
	}

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

func TestS3OwnersWithGarage(t *testing.T) {
	garage := StartGarage(t)

	t.Setenv("AWS_ACCESS_KEY_ID", garage.AccessKey)
	t.Setenv("AWS_SECRET_ACCESS_KEY", garage.SecretKey)
	t.Setenv("S3_FORCE_PATH_STYLE", "true")

	s3Store, err := s3.NewClient(s3.Config{
		S3Bucket:   garage.BucketName,
		S3Endpoint: garage.Endpoint,
		Region:     "us-east-1",
	})
	if err != nil {
		t.Fatalf("create s3 store: %v", err)
	}

	key := "blt-volume-manager/owners/test-vol/proposal.json"
	data := []byte(`{"name":"test","expiry_time":9999999999}`)
	if err := s3Store.PutObject(key, data); err != nil {
		t.Fatalf("put lock object: %v", err)
	}

	got, err := s3Store.ReadObject(key)
	if err != nil {
		t.Fatalf("read lock object: %v", err)
	}
	if got == nil {
		t.Fatal("lock object not found")
	}

	objects, err := s3Store.ListObjects("blt-volume-manager/owners/test-vol/")
	if err != nil {
		t.Fatalf("list objects: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 lock object, got %d", len(objects))
	}

	if err := s3Store.DeleteObject(key); err != nil {
		t.Fatalf("delete lock object: %v", err)
	}
	got, err = s3Store.ReadObject(key)
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if got != nil {
		t.Fatal("lock object should have been deleted")
	}
}
