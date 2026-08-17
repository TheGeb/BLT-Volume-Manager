package store

import (
	"context"
	"testing"
	"time"
)

func TestOwnerProposalKeyRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		volume   string
		owner    string
		creation int64
		expiry   int64
	}{
		{"permanent", "vol-a", "host-1", 1700000000, 0},
		{"time-limited", "vol-a", "host-1", 1700000000, 1700003600},
		{"owner with dashes", "vol-a", "node-a-7", 1700000000, 1700001800},
		{"nested volume", "group/vol", "host-1", 1700000000, 1700001800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := OwnerProposalKey(tt.volume, tt.owner, tt.creation, tt.expiry)
			if err != nil {
				t.Fatalf("OwnerProposalKey: %v", err)
			}
			vol, owner, creation, expiry, err := ParseOwnerKey(key)
			if err != nil {
				t.Fatalf("ParseOwnerKey: %v", err)
			}
			if vol != tt.volume || owner != tt.owner || creation != tt.creation || expiry != tt.expiry {
				t.Errorf("round trip = (%q,%q,%d,%d), want (%q,%q,%d,%d)",
					vol, owner, creation, expiry, tt.volume, tt.owner, tt.creation, tt.expiry)
			}
		})
	}
}

func TestOwnerProposalKeyInvalidExpiry(t *testing.T) {
	t.Parallel()
	if _, err := OwnerProposalKey("vol-a", "host-1", 1700003600, 1700000000); err == nil {
		t.Fatal("expected error when expiry precedes creation")
	}
}

func TestWriteOwnerLockS3(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	expiry := time.Now().Add(time.Hour).Unix()

	key, err := WriteOwnerLock(context.Background(), b, "vol-a", "host-1", expiry)
	if err != nil {
		t.Fatalf("WriteOwnerLock: %v", err)
	}
	vol, owner, _, gotExpiry, err := ParseOwnerKey(key)
	if err != nil {
		t.Fatalf("ParseOwnerKey: %v", err)
	}
	if vol != "vol-a" || owner != "host-1" || gotExpiry != expiry {
		t.Errorf("written lock = (%q,%q,%d), want (%q,%q,%d)", vol, owner, gotExpiry, "vol-a", "host-1", expiry)
	}
}

func TestOwnerLockKey(t *testing.T) {
	t.Parallel()
	want := "blt-volume-manager/owners/vol-a/lock"
	if got := OwnerLockKey("vol-a"); got != want {
		t.Errorf("OwnerLockKey = %q, want %q", got, want)
	}
}
