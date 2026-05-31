package locker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Locker provides a pluggable locking implementation. Replace with S3 locker as needed.
type Locker interface {
	Acquire(ctx context.Context, name string) (Lock, error)
}

type Lock interface {
	Release() error
	IsValid() (bool, error)
}

// Simple file-based locker that creates a lock file using O_EXCL.
type fileLocker struct {
	dir string
}

func NewFileLocker(dir string) Locker {
	return &fileLocker{dir: dir}
}

type fileLock struct {
	path string
}

func (l *fileLocker) Acquire(ctx context.Context, name string) (Lock, error) {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}
	path := filepath.Join(l.dir, name+".lock")
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if _, werr := fmt.Fprintf(f, "pid:%d\n", os.Getpid()); werr != nil {
				_ = f.Close()
				_ = os.Remove(path)
				return nil, fmt.Errorf("write lock file: %w", werr)
			}
			if cerr := f.Close(); cerr != nil {
				_ = os.Remove(path)
				return nil, fmt.Errorf("close lock file: %w", cerr)
			}
			return &fileLock{path: path}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire lock after 5s: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (fl *fileLock) Release() error {
	return os.Remove(fl.path)
}

func (fl *fileLock) IsValid() (bool, error) {
	_, err := os.Stat(fl.path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
