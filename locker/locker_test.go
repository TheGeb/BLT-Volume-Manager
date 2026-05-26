package locker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLockerAcquire(t *testing.T) {
	dir := t.TempDir()
	l := NewFileLocker(dir)

	lock, err := l.Acquire(context.Background(), "test-vol")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	lockPath := filepath.Join(dir, "test-vol.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file not created")
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	if len(data) == 0 {
		t.Error("lock file is empty")
	}

	lock.Release()
}

func TestFileLockIsValid(t *testing.T) {
	dir := t.TempDir()
	l := NewFileLocker(dir)

	lock, err := l.Acquire(context.Background(), "valid-test")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	valid, err := lock.IsValid()
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if !valid {
		t.Error("expected valid lock")
	}

	lock.Release()

	valid, err = lock.IsValid()
	if err != nil {
		t.Fatalf("IsValid error after release: %v", err)
	}
	if valid {
		t.Error("expected invalid lock after release")
	}
}

func TestFileLockIsValidNoFile(t *testing.T) {
	fl := &fileLock{path: "/nonexistent/path.lock"}
	valid, err := fl.IsValid()
	if err != nil {
		t.Fatalf("IsValid error: %v", err)
	}
	if valid {
		t.Error("expected invalid for nonexistent path")
	}
}

func TestFileLockerAcquireExistingLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "existing.lock")
	if err := os.WriteFile(lockPath, []byte("pid:123"), 0644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	l := NewFileLocker(dir)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := l.Acquire(ctx, "existing")
	if err == nil {
		t.Error("expected error when lock file already exists")
	}
}

func TestNewFileLockerCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "locks")
	l := NewFileLocker(dir)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("NewFileLocker should not create directory immediately")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lock, err := l.Acquire(ctx, "testlock")
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}
	defer lock.Release()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Acquire should create directory")
	}
}
