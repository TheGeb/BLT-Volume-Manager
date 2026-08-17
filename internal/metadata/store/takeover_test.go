package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeOwnerLock is a minimal in-memory OwnerLock whose AcquireLock refuses
// when a lock already exists, enabling exercise of the CheckAndUpdateLock
// migrated-lock takeover path.
type fakeOwnerLock struct {
	key       string
	entry     OwnerEntry
	hasKey    bool
	releases  int
	conflicts int
}

func (f *fakeOwnerLock) AcquireLock(_ context.Context, volume, owner string, _ int64) (string, error) {
	if f.hasKey {
		f.conflicts++
		return "", ErrLockConflict
	}
	f.key = OwnerPrefix(volume) + "lock"
	f.entry = OwnerEntry{Name: owner}
	f.hasKey = true
	return f.key, nil
}

func (f *fakeOwnerLock) CheckAndUpdateLock(ctx context.Context, volume, owner string, expiry int64) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		key, err := f.AcquireLock(ctx, volume, owner, expiry)
		if err == nil {
			return key, nil
		}
		if !errors.Is(err, ErrLockConflict) {
			return "", err
		}
		if !f.hasKey {
			continue
		}
		if !f.entry.Migrated {
			return "", ErrLockConflict
		}
		_ = f.ReleaseLockAny(ctx, f.key)
	}
	return "", errors.New("too many conflicts")
}

func (f *fakeOwnerLock) LockIsValid(_ context.Context, key string) (bool, error) {
	return f.hasKey && key == f.key, nil
}

func (f *fakeOwnerLock) ReleaseLock(_ context.Context, key string) error {
	if key == f.key {
		f.hasKey = false
	}
	return nil
}

func (f *fakeOwnerLock) ReleaseLockAny(_ context.Context, key string) error {
	if key == f.key {
		f.hasKey = false
		f.releases++
	}
	return nil
}

func (f *fakeOwnerLock) FindForVolume(_ context.Context, _ string) (*VolumeOwner, error) {
	return &VolumeOwner{}, nil
}

func (f *fakeOwnerLock) ListAllLocks(_ context.Context) (map[string]VolumeOwner, error) {
	return map[string]VolumeOwner{}, nil
}

func (f *fakeOwnerLock) DeleteForVolume(_ context.Context, _ string) error { return nil }

func TestCheckAndUpdateLock_LiveConflict(t *testing.T) {
	t.Parallel()
	f := &fakeOwnerLock{
		key:    "k",
		entry:  OwnerEntry{Name: "other", Migrated: false},
		hasKey: true,
	}
	s := NewOwnerStore(f)

	expiry := time.Now().Add(time.Hour).Unix()
	_, err := s.CheckAndUpdateLock(context.Background(), "v", "me", expiry)
	if !errors.Is(err, ErrLockConflict) {
		t.Fatalf("got err=%v, want ErrLockConflict", err)
	}
	// A live lock must not be released.
	if f.releases != 0 {
		t.Errorf("live lock was released (%d), want 0", f.releases)
	}
	if !f.hasKey || f.entry.Name != "other" {
		t.Errorf("live lock was disturbed: hasKey=%v owner=%q", f.hasKey, f.entry.Name)
	}
}

func TestCheckAndUpdateLock_TakeoverMigrated(t *testing.T) {
	t.Parallel()
	f := &fakeOwnerLock{
		key:    "k",
		entry:  OwnerEntry{Name: "other", Migrated: true},
		hasKey: true,
	}
	s := NewOwnerStore(f)

	expiry := time.Now().Add(time.Hour).Unix()
	key, err := s.CheckAndUpdateLock(context.Background(), "v", "me", expiry)
	if err != nil {
		t.Fatalf("CheckAndUpdateLock error: %v", err)
	}
	if f.releases != 1 {
		t.Errorf("migrated lock releases = %d, want 1", f.releases)
	}
	if key != f.key || !f.hasKey {
		t.Fatalf("lock not re-acquired after takeover")
	}
	if f.entry.Name != "me" {
		t.Errorf("owner after takeover = %q, want %q", f.entry.Name, "me")
	}
}

func TestCheckAndUpdateLock_FreeAcquiresDirectly(t *testing.T) {
	t.Parallel()
	f := &fakeOwnerLock{}
	s := NewOwnerStore(f)

	expiry := time.Now().Add(time.Hour).Unix()
	key, err := s.CheckAndUpdateLock(context.Background(), "v", "me", expiry)
	if err != nil {
		t.Fatalf("CheckAndUpdateLock error: %v", err)
	}
	if f.releases != 0 {
		t.Errorf("unexpected releases = %d, want 0", f.releases)
	}
	if key != f.key || f.entry.Name != "me" {
		t.Errorf("unexpected acquistion state")
	}
}

var _ OwnerLock = (*fakeOwnerLock)(nil)
