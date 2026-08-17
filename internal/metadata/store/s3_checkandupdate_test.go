package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestS3CheckAndUpdateLock_RenewsOwnLock verifies mount-mode renewal: a
// CheckAndUpdateLock against a live lock held by the same owner re-anchors the
// lock (new proposal, old one removed) instead of reporting a conflict.
func TestS3CheckAndUpdateLock_RenewsOwnLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newOrderedBackend()
	s := NewOwnerStore(NewS3MetadataStore(b))
	expiry := time.Now().Add(time.Hour).Unix()

	k1, err := s.CheckAndUpdateLock(ctx, "vol", "host-1", expiry)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	k2, err := s.CheckAndUpdateLock(ctx, "vol", "host-1", expiry+3600)
	if err != nil {
		t.Fatalf("renewal must refresh the caller's own lock, got %v", err)
	}
	if k1 == k2 {
		t.Error("renewal should produce a fresh proposal key")
	}
	if _, err := b.ReadObject(ctx, k1); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("old proposal should be released, read err=%v", err)
	}
	if valid, err := s.LockIsValid(ctx, k2); err != nil || !valid {
		t.Errorf("renewed lock invalid: valid=%v err=%v", valid, err)
	}
	// A third party must still see host-1 as the owner.
	vo, err := s.FindForVolume(ctx, "vol")
	if err != nil {
		t.Fatalf("FindForVolume: %v", err)
	}
	if vo.Owner != "host-1" {
		t.Errorf("owner = %q, want host-1", vo.Owner)
	}
}

// TestS3CheckAndUpdateLock_TakesOverMigrated verifies the etcd->S3 cutover:
// when the winning proposal was written by the migration tool (a handoff with
// no live holder), a fresh host takes it over instead of getting a permanent
// conflict.
func TestS3CheckAndUpdateLock_TakesOverMigrated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newOrderedBackend()
	s := NewOwnerStore(NewS3MetadataStore(b))

	// Simulate the migration tool writing a refreshed handoff lock.
	migKey, err := WriteOwnerLock(ctx, b, "vol", "old-host", time.Now().Add(time.Hour).Unix())
	if err != nil {
		t.Fatalf("WriteOwnerLock: %v", err)
	}
	// Proposal keys encode the creation second, so space the attempts out to
	// get distinct keys (otherwise the duplicate overwrites the migrated one).
	time.Sleep(1100 * time.Millisecond)
	// Any duplicate proposal from the old host loses and must not matter.
	if _, err := AcquireOwnerLock(ctx, b, OwnerPrefix("vol"), "old-host", time.Now().Add(time.Hour).Unix()); err == nil {
		t.Fatal("expected conflict for duplicate live proposal from old host")
	}

	k, err := s.CheckAndUpdateLock(ctx, "vol", "new-host", time.Now().Add(time.Hour).Unix())
	if err != nil {
		t.Fatalf("migrated takeover must succeed, got %v", err)
	}
	if k == migKey {
		t.Error("takeover should return the new host's proposal key")
	}
	vo, err := s.FindForVolume(ctx, "vol")
	if err != nil {
		t.Fatalf("FindForVolume: %v", err)
	}
	if vo.Owner != "new-host" {
		t.Errorf("owner = %q, want new-host", vo.Owner)
	}
}

// TestS3CheckAndUpdateLock_LiveConflictRefused verifies a live lock held by
// another owner is never taken over, even when a stale migrated proposal with
// an earlier timestamp still sits in the keyspace.
func TestS3CheckAndUpdateLock_LiveConflictRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newOrderedBackend()
	s := NewOwnerStore(NewS3MetadataStore(b))

	// A migrated handoff for the volume from a previous cutover...
	if _, err := WriteOwnerLock(ctx, b, "vol", "old-host", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatalf("WriteOwnerLock: %v", err)
	}
	// ...followed by a live lock from the current holder.
	if _, err := s.CheckAndUpdateLock(ctx, "vol", "live-host", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatalf("live acquire: %v", err)
	}
	if _, err := s.CheckAndUpdateLock(ctx, "vol", "intruder", time.Now().Add(time.Hour).Unix()); !errors.Is(err, ErrLockConflict) {
		t.Fatalf("want ErrLockConflict against live lock, got %v", err)
	}
	vo, err := s.FindForVolume(ctx, "vol")
	if err != nil {
		t.Fatalf("FindForVolume: %v", err)
	}
	if vo.Owner != "live-host" {
		t.Errorf("owner = %q, want live-host", vo.Owner)
	}
}

// TestS3CheckAndUpdateLock_OwnLockSurvivesStaleProposals mirrors the renewal
// path under noise: unrelated expired proposals are cleaned, the renewal
// still re-anchors the caller's lock.
func TestS3CheckAndUpdateLock_OwnLockSurvivesStaleProposals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newOrderedBackend()
	s := NewOwnerStore(NewS3MetadataStore(b))

	// Noise: an expired loss from a dead host.
	staleKey, err := OwnerProposalKey("vol", "dead-host", time.Now().Add(-time.Hour).Unix(), time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.PutObject(ctx, staleKey, []byte(`{"name":"dead-host"}`)); err != nil {
		t.Fatal(err)
	}

	if _, err := s.CheckAndUpdateLock(ctx, "vol", "host-1", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	// Renewal must produce a distinct proposal key (creation-second encoding).
	time.Sleep(1100 * time.Millisecond)
	if _, err := s.CheckAndUpdateLock(ctx, "vol", "host-1", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatalf("renewal: %v", err)
	}
	objs, err := b.ListObjects(ctx, OwnerPrefix("vol"))
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	names := make(map[string]bool)
	for _, o := range objs {
		names[*o.Key] = true
	}
	if len(names) != 1 {
		t.Errorf("expected exactly the renewed proposal to remain, got %d keys: %v", len(names), keysOf(names))
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
