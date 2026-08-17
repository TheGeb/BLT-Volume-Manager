package etcd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
)

// A finite TTL lease surfaces its real epoch ExpiryTime via FindLock, while a
// permanent (keepalive-held) lock reports 0 so migration can distinguish them.
func TestAcquireLock_ExpiryPerTTL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cli := newEtcdClient(t, etcdAddr(t))
	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	if _, err := cli.AcquireLock(ctx, t.Name(), "h", time.Now().Add(120*time.Second).Unix()); err != nil {
		t.Fatalf("AcquireLock finite: %v", err)
	}
	_, _, _, finiteExpiry, _, ferr := cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock finite: %v", ferr)
	}
	now := time.Now().Unix()
	if finiteExpiry <= now || finiteExpiry > now+121 {
		t.Errorf("finite expiry = %d, want ~now+120", finiteExpiry)
	}

	_ = cli.ReleaseLockAny(ctx, lockKeyFor(t.Name()))
	if _, err := cli.AcquireLock(ctx, t.Name(), "h", -1); err != nil {
		t.Fatalf("AcquireLock permanent: %v", err)
	}
	_, _, _, permExpiry, _, ferr := cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock permanent: %v", ferr)
	}
	if permExpiry != 0 {
		t.Errorf("permanent expiry = %d, want 0", permExpiry)
	}
}

// A migrated lock is marked Migrated and can be taken over by release-then-
// acquire, whereas a live lock held by another owner refuses takeover.
func TestCheckAndUpdateLock_Etcd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cli := newEtcdClient(t, etcdAddr(t))
	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	s := store.NewOwnerStore(cli)

	// Live lock held by another owner must not be taken over.
	if _, err := cli.AcquireLock(ctx, t.Name(), "h-other", time.Now().Add(60*time.Second).Unix()); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if _, err := s.CheckAndUpdateLock(ctx, t.Name(), "h-me", time.Now().Add(60*time.Second).Unix()); !errors.Is(err, store.ErrLockConflict) {
		t.Fatalf("live takeover: want ErrLockConflict, got %v", err)
	}
	_, owner, _, _, _, ferr := cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock: %v", ferr)
	}
	if owner != "h-other" {
		t.Errorf("live lock owner = %q, want %q", owner, "h-other")
	}

	// Re-acquire as a fresh migrated lock, then take it over.
	if _, err := cli.AcquireLock(ctx, t.Name(), "h-me", time.Now().Add(60*time.Second).Unix()); err != nil {
		t.Fatalf("ReacquireLock: %v", err)
	}
	_ = cli.ReleaseLockAny(ctx, lockKeyFor(t.Name()))
	if err := cli.SetMigratedOwnerLock(ctx, t.Name(), "h-other", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatalf("SetMigratedOwnerLock: %v", err)
	}
	_, _, _, _, migrated, ferr := cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock migrated: %v", ferr)
	}
	if !migrated {
		t.Fatal("expected lock to be flagged migrated")
	}
	if _, err := s.CheckAndUpdateLock(ctx, t.Name(), "h-me", time.Now().Add(60*time.Second).Unix()); err != nil {
		t.Fatalf("migrated takeover: %v", err)
	}
	_, owner, _, _, migrated, ferr = cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock after takeover: %v", ferr)
	}
	if owner != "h-me" {
		t.Errorf("owner after takeover = %q, want %q", owner, "h-me")
	}
	if migrated {
		t.Error("lock should no longer be flagged migrated after takeover")
	}
}

// A CheckAndUpdateLock against a live lock held by the same owner refreshes it
// in place: the same key comes back with a re-stamped expiry, the lease is
// untouched (still alive), and the handoff flag - set when the same lock was
// previously migrated - is cleared so no other host can take it over.
func TestCheckAndUpdateLock_Etcd_RefreshesOwnLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cli := newEtcdClient(t, etcdAddr(t))
	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	s := store.NewOwnerStore(cli)

	firstExpiry := time.Now().Add(120 * time.Second).Unix()
	k1, err := s.CheckAndUpdateLock(ctx, t.Name(), "h-me", firstExpiry)
	if err != nil {
		t.Fatalf("CheckAndUpdateLock: %v", err)
	}

	// Renew with a later expiry - must succeed and return the same key.
	renewedExpiry := time.Now().Add(300 * time.Second).Unix()
	k2, err := s.CheckAndUpdateLock(ctx, t.Name(), "h-me", renewedExpiry)
	if err != nil {
		t.Fatalf("same-owner refresh must succeed, got %v", err)
	}
	if k2 != k1 {
		t.Errorf("refresh key = %q, want same key %q", k2, k1)
	}
	_, owner, _, expiry, migrated, ferr := cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock: %v", ferr)
	}
	if owner != "h-me" {
		t.Errorf("owner = %q, want h-me", owner)
	}
	if expiry < renewedExpiry-5 || expiry > renewedExpiry+5 {
		t.Errorf("expiry = %d, want ~%d", expiry, renewedExpiry)
	}
	if migrated {
		t.Error("refreshed lock must not remain flagged migrated")
	}
	if valid, verr := cli.LockIsValid(ctx, k2); verr != nil || !valid {
		t.Errorf("refreshed lock invalid: valid=%v err=%v", valid, verr)
	}
	// The keepalive-held lease must still be tracked (i.e. not revoked).
	cli.mu.Lock()
	_, tracked := cli.lastLeaseIDs[k1]
	cli.mu.Unlock()
	if !tracked {
		t.Error("lease tracked on renewal should still be tracked")
	}
}

// A migrated lock held by the same owner (a host that was restarted with the
// same owner identity) is refreshed in place and cleared of its handoff flag.
func TestCheckAndUpdateLock_Etcd_RefreshesOwnMigratedLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cli := newEtcdClient(t, etcdAddr(t))
	cleanKeys(t, cli, lockKeyFor(t.Name()))
	defer cleanKeys(t, cli, lockKeyFor(t.Name()))

	if err := cli.SetMigratedOwnerLock(ctx, t.Name(), "h-me", time.Now().Add(300*time.Second).Unix()); err != nil {
		t.Fatalf("SetMigratedOwnerLock: %v", err)
	}
	k, err := store.NewOwnerStore(cli).CheckAndUpdateLock(ctx, t.Name(), "h-me", time.Now().Add(300*time.Second).Unix())
	if err != nil {
		t.Fatalf("refresh own migrated lock: %v", err)
	}
	_, _, _, _, migrated, ferr := cli.FindLock(ctx, t.Name())
	if ferr != nil {
		t.Fatalf("FindLock: %v", ferr)
	}
	if migrated {
		t.Error("refreshed lock must not remain flagged migrated")
	}
	if valid, verr := cli.LockIsValid(ctx, k); verr != nil || !valid {
		t.Errorf("refreshed lock invalid: valid=%v err=%v", valid, verr)
	}
}
