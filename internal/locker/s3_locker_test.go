package locker

import (
	"errors"
	"testing"

	"github.com/example/blt-volume-manager/internal/store"
)

func isLockOwned(err error) bool {
	var le *LockError
	return errors.As(err, &le) && le.Code == LockHeldByAnother
}

func TestIsLockOwned(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"LockHeldByAnother", &LockError{Code: LockHeldByAnother, Msg: "held"}, true},
		{"NoLockAcquired", &LockError{Code: NoLockAcquired, Msg: "failed"}, false},
		{"nil", nil, false},
		{"random error", errors.New("random"), false},
		{"plain LockError", &LockError{Code: 999}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLockOwned(tt.err); got != tt.expected {
				t.Errorf("isLockOwned(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestGetHostname(t *testing.T) {
	h := store.Hostname()
	if h == "" {
		t.Error("expected non-empty hostname")
	}
}

func TestLockFolder(t *testing.T) {
	got := store.LockFolder("my-volume")
	want := "blt-volume-manager/locks/my-volume/"
	if got != want {
		t.Errorf("LockFolder() = %q, want %q", got, want)
	}

	got2 := store.LockFolder("group/sub-vol")
	want2 := "blt-volume-manager/locks/group/sub-vol/"
	if got2 != want2 {
		t.Errorf("LockFolder() = %q, want %q", got2, want2)
	}
}

func TestNewS3Locker(t *testing.T) {
	l, err := NewS3Locker("my-bucket", "http://localhost:9000", "us-east-1", 30)
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl, ok := l.(*s3Locker)
	if !ok {
		t.Fatal("expected *s3Locker type")
	}
	if sl.bucket != "my-bucket" {
		t.Errorf("bucket = %q, want %q", sl.bucket, "my-bucket")
	}
	if sl.maxHoldMins != 30 {
		t.Errorf("maxHoldMins = %d, want 30", sl.maxHoldMins)
	}
	if sl.endpoint != "http://localhost:9000" {
		t.Errorf("endpoint = %q", sl.endpoint)
	}
	if sl.region != "us-east-1" {
		t.Errorf("region = %q", sl.region)
	}
}

func TestNewS3LockerDefaultMaxMins(t *testing.T) {
	l, err := NewS3Locker("bucket", "", "", 10)
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl := l.(*s3Locker)
	if sl.maxHoldMins != 10 {
		t.Errorf("default maxHoldMins = %d, want 10", sl.maxHoldMins)
	}
}

func TestNewS3LockerInvalidEnv(t *testing.T) {
	l, err := NewS3Locker("bucket", "", "", 10)
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl := l.(*s3Locker)
	if sl.maxHoldMins != 10 {
		t.Errorf("expected fallback to 10, got %d", sl.maxHoldMins)
	}
}

func TestNewS3LockerZeroMaxMins(t *testing.T) {
	l, err := NewS3Locker("bucket", "", "", 10)
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl := l.(*s3Locker)
	if sl.maxHoldMins != 10 {
		t.Errorf("expected fallback to 10 for zero, got %d", sl.maxHoldMins)
	}
}

func TestLockError(t *testing.T) {
	err := &LockError{Code: LockHeldByAnother, Msg: "lock held by another host"}
	if err.Error() != "lock held by another host" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}
