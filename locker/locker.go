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
}

// Simple file-based locker that creates a lock file using O_EXCL.
type fileLocker struct{
	dir string
}

func NewFileLocker(dir string) Locker {
	_ = os.MkdirAll(dir, 0755)
	return &fileLocker{dir: dir}
}

type fileLock struct {
	path string
}

func (l *fileLocker) Acquire(ctx context.Context, name string) (Lock, error) {
	path := filepath.Join(l.dir, name+".lock")
	// Try repeatedly for a short period
	deadline := time.Now().Add(5 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(fmt.Sprintf("pid:%d\n", os.Getpid()))
			f.Close()
			return &fileLock{path: path}, nil
		}
		if time.Now().After(deadline) {
			return nil, err
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
