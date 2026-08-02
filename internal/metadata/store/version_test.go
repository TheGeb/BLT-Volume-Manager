package store

import (
	"context"
	"sync"
	"testing"
)

// --- VersionStore tests with orderedBackend (S3 path) ---

func newVersionStore(t *testing.T) *VersionStore {
	t.Helper()
	return NewVersionStore(newOrderedBackend())
}

func TestVersionStore_NextTags_VolumeIsolation(t *testing.T) {
	t.Parallel()
	vs := newVersionStore(t)
	ctx := context.Background()

	v1Tags, err := vs.NextTags(ctx, "vol1", false)
	if err != nil {
		t.Fatalf("vol1 NextTags error: %v", err)
	}

	v2Tags, err := vs.NextTags(ctx, "vol2", false)
	if err != nil {
		t.Fatalf("vol2 NextTags error: %v", err)
	}

	if v1Tags[1] != "v0.1" || v2Tags[1] != "v0.1" {
		t.Error("volumes should each start at v0.1")
	}

	// vol1 increments independently
	v1Tags, err = vs.NextTags(ctx, "vol1", true)
	if err != nil {
		t.Fatalf("vol1 major NextTags error: %v", err)
	}
	if v1Tags[1] != "v1.0" {
		t.Errorf("expected v1.0, got %q", v1Tags[1])
	}

	// vol2 should still be at v0.1
	v2Tags, err = vs.NextTags(ctx, "vol2", false)
	if err != nil {
		t.Fatalf("vol2 NextTags error: %v", err)
	}
	if v2Tags[1] != "v0.2" {
		t.Errorf("expected v0.2, got %q", v2Tags[1])
	}
}

func TestVersionStore_ReadCounter_NotFound(t *testing.T) {
	t.Parallel()
	vs := newVersionStore(t)
	ctx := context.Background()

	_, err := vs.ReadCounter(ctx, "nonexistent")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestVersionStore_WriteCounter_RoundTrip(t *testing.T) {
	t.Parallel()
	vs := newVersionStore(t)
	ctx := context.Background()

	vc := VersionCounter{Major: 2, Minor: 5}
	if err := vs.WriteCounter(ctx, "myvol", vc); err != nil {
		t.Fatalf("WriteCounter error: %v", err)
	}

	got, err := vs.ReadCounter(ctx, "myvol")
	if err != nil {
		t.Fatalf("ReadCounter error: %v", err)
	}
	if got.Major != 2 || got.Minor != 5 {
		t.Errorf("read back %+v, want {Major:2 Minor:5}", got)
	}
}

func TestVersionStore_NextTags_Format(t *testing.T) {
	t.Parallel()
	vs := newVersionStore(t)
	ctx := context.Background()

	tests := []struct {
		major     bool
		wantMajor string
		wantFull  string
	}{
		{false, "v0", "v0.1"},
		{false, "v0", "v0.2"},
		{true, "v1", "v1.0"},
		{false, "v1", "v1.1"},
		{true, "v2", "v2.0"},
	}

	for _, tt := range tests {
		tags, err := vs.NextTags(ctx, "fmtvol", tt.major)
		if err != nil {
			t.Fatalf("NextTags(major=%v) error: %v", tt.major, err)
		}
		if tags[0] != tt.wantMajor {
			t.Errorf("major tag = %q, want %q", tags[0], tt.wantMajor)
		}
		if tags[1] != tt.wantFull {
			t.Errorf("full tag = %q, want %q", tags[1], tt.wantFull)
		}
	}
}

func TestVersionStore_NextTags_Concurrent(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	vs := NewVersionStore(b)
	ctx := context.Background()

	const workers = 10
	var (
		mu   sync.Mutex
		tags []string
		wg   sync.WaitGroup
	)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			verTags, verErr := vs.NextTags(ctx, "convol", false)
			if verErr != nil {
				return
			}
			mu.Lock()
			tags = append(tags, verTags[1])
			mu.Unlock()
		}()
	}
	wg.Wait()

	// With the S3 backend, the read-then-write pattern is NOT atomic, so
	// duplicate version tags are expected under concurrent writes. This
	// documents the S3-only limitation: version allocation is not safe
	// across independent writers unless a Coordinator (etcd) is configured.
	t.Logf("S3 backend: got %d unique tags from %d workers (duplicates expected)", len(uniqueStrings(tags)), workers)
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
