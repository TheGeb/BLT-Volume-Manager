package locker

import (
	"errors"
	"os"
	"testing"
)

func TestIsLockOwned(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"LockAlreadyOwned", &LockError{Code: LockAlreadyOwned, Msg: "held"}, true},
		{"NoLockAcquired", &LockError{Code: NoLockAcquired, Msg: "failed"}, false},
		{"nil", nil, false},
		{"random error", errors.New("random"), false},
		{"plain LockError", &LockError{Code: 999}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLockOwned(tt.err); got != tt.expected {
				t.Errorf("IsLockOwned(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestGetHostname(t *testing.T) {
	h := getHostname()
	if h == "" {
		t.Error("expected non-empty hostname")
	}
}

func TestLockFolder(t *testing.T) {
	s := &s3Locker{}
	got := s.lockFolder("my-volume")
	want := "blt-volume-manager/locks/my-volume/"
	if got != want {
		t.Errorf("lockFolder() = %q, want %q", got, want)
	}

	got2 := s.lockFolder("group/sub-vol")
	want2 := "blt-volume-manager/locks/group/sub-vol/"
	if got2 != want2 {
		t.Errorf("lockFolder() = %q, want %q", got2, want2)
	}
}

func TestNewS3Locker(t *testing.T) {
	os.Setenv("S3_LOCK_MAX_MINS", "30")
	defer os.Unsetenv("S3_LOCK_MAX_MINS")

	l, err := NewS3Locker("my-bucket", "http://localhost:9000", "us-east-1")
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
	os.Unsetenv("S3_LOCK_MAX_MINS")

	l, err := NewS3Locker("bucket", "", "")
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl := l.(*s3Locker)
	if sl.maxHoldMins != 10 {
		t.Errorf("default maxHoldMins = %d, want 10", sl.maxHoldMins)
	}
}

func TestNewS3LockerInvalidEnv(t *testing.T) {
	os.Setenv("S3_LOCK_MAX_MINS", "invalid")
	defer os.Unsetenv("S3_LOCK_MAX_MINS")

	l, err := NewS3Locker("bucket", "", "")
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl := l.(*s3Locker)
	if sl.maxHoldMins != 10 {
		t.Errorf("expected fallback to 10, got %d", sl.maxHoldMins)
	}
}

func TestNewS3LockerZeroMaxMins(t *testing.T) {
	os.Setenv("S3_LOCK_MAX_MINS", "0")
	defer os.Unsetenv("S3_LOCK_MAX_MINS")

	l, err := NewS3Locker("bucket", "", "")
	if err != nil {
		t.Fatalf("NewS3Locker failed: %v", err)
	}

	sl := l.(*s3Locker)
	if sl.maxHoldMins != 10 {
		t.Errorf("expected fallback to 10 for zero, got %d", sl.maxHoldMins)
	}
}

func TestLockError(t *testing.T) {
	err := &LockError{Code: LockAlreadyOwned, Msg: "lock held by another host"}
	if err.Error() != "lock held by another host" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}
