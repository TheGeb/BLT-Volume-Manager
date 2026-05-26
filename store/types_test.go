package store

import (
	"testing"
	"time"
)

func TestGetRemainingTimeinSeconds(t *testing.T) {
	future := LockOwner{Name: "test", ExpiryTime: time.Now().Add(1 * time.Hour).Unix()}
	if r := future.GetRemainingTimeinSeconds(); r <= 0 {
		t.Errorf("expected positive remaining, got %d", r)
	}

	past := LockOwner{Name: "test", ExpiryTime: time.Now().Add(-1 * time.Hour).Unix()}
	if r := past.GetRemainingTimeinSeconds(); r > 0 {
		t.Errorf("expected negative/zero remaining, got %d", r)
	}

	zero := LockOwner{Name: "test", ExpiryTime: time.Now().Unix()}
	r := zero.GetRemainingTimeinSeconds()
	if r > 10 || r < -10 {
		t.Errorf("expected remaining near 0, got %d", r)
	}
}
