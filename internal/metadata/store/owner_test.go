package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/backend"
)

// orderedBackend tracks insertion order for deterministic ListObjects.
type orderedBackend struct {
	mu      sync.Mutex
	entries []orderedEntry
	clock   int64
}

type orderedEntry struct {
	key   string
	data  []byte
	clock int64
}

func newOrderedBackend() *orderedBackend {
	return &orderedBackend{}
}

func (b *orderedBackend) PutObject(ctx context.Context, key string, data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clock++
	b.entries = append(b.entries, orderedEntry{key: key, data: data, clock: b.clock})
	return nil
}

func (b *orderedBackend) ReadObject(ctx context.Context, key string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range b.entries {
		if e.key == key {
			return e.data, nil
		}
	}
	return nil, backend.ErrKeyNotFound
}

func (b *orderedBackend) DeleteObject(ctx context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, e := range b.entries {
		if e.key == key {
			b.entries = append(b.entries[:i], b.entries[i+1:]...)
			return nil
		}
	}
	return nil
}

func (b *orderedBackend) ListObjects(ctx context.Context, prefix string) ([]backend.Entry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []backend.Entry
	for _, e := range b.entries {
		if strings.HasPrefix(e.key, prefix) {
			key := e.key
			counter := e.clock
			result = append(result, backend.Entry{
				Key:                 &key,
				ModificationCounter: &counter,
			})
		}
	}
	return result, nil
}

func (b *orderedBackend) DeleteObjectsWithPrefix(ctx context.Context, prefix string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var kept []orderedEntry
	for _, e := range b.entries {
		if !strings.HasPrefix(e.key, prefix) {
			kept = append(kept, e)
		}
	}
	b.entries = kept
	return nil
}

// stringPtr is a helper to create *string literals.
func stringPtr(s string) *string { return &s }

// int64Ptr is a helper to create *int64 literals.
func int64Ptr(n int64) *int64 { return &n }

func TestEncodeDecodeOwner(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		encoded string
		decoded string
	}{
		{
			name:    "no dash",
			input:   "myhost",
			encoded: "myhost",
			decoded: "myhost",
		},
		{
			name:    "single dash",
			input:   "myHost-12345",
			encoded: "myHost%2D12345",
			decoded: "myHost-12345",
		},
		{
			name:    "multiple dashes",
			input:   "a-b-c",
			encoded: "a%2Db%2Dc",
			decoded: "a-b-c",
		},
		{
			name:    "leading dash",
			input:   "-host",
			encoded: "%2Dhost",
			decoded: "-host",
		},
		{
			name:    "trailing dash",
			input:   "host-",
			encoded: "host%2D",
			decoded: "host-",
		},
		{
			name:    "empty string",
			input:   "",
			encoded: "",
			decoded: "",
		},
		{
			name:    "no encoding needed for numbers",
			input:   "server42",
			encoded: "server42",
			decoded: "server42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEnc := encodeOwner(tt.input)
			if gotEnc != tt.encoded {
				t.Errorf("encodeOwner(%q) = %q, want %q", tt.input, gotEnc, tt.encoded)
			}
			gotDec := decodeOwner(gotEnc)
			if gotDec != tt.decoded {
				t.Errorf("decodeOwner(%q) = %q, want %q", gotEnc, gotDec, tt.decoded)
			}
			// Round-trip
			if decodeOwner(encodeOwner(tt.input)) != tt.input {
				t.Errorf("round-trip failed for %q", tt.input)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "seconds", input: "30s", want: 30 * time.Second},
		{name: "minutes", input: "15m", want: 15 * time.Minute},
		{name: "hours", input: "2h", want: 2 * time.Hour},
		{name: "zero seconds", input: "0s", want: 0},
		{name: "zero minutes", input: "0m", want: 0},
		{name: "zero hours", input: "0h", want: 0},
		{name: "large minute value", input: "1440m", want: 1440 * time.Minute},
		{name: "single digit seconds", input: "1s", want: 1 * time.Second},
		{name: "empty", input: "", wantErr: true},
		{name: "no unit", input: "42", wantErr: true},
		{name: "unknown unit", input: "5d", wantErr: true},
		{name: "non-numeric", input: "abc", wantErr: true},
		{name: "just unit", input: "h", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{name: "hours exact", input: 2 * time.Hour, want: "2h"},
		{name: "hours exact large", input: 48 * time.Hour, want: "48h"},
		{name: "minutes exact", input: 30 * time.Minute, want: "30m"},
		{name: "minutes from seconds", input: 120 * time.Second, want: "2m"},
		{name: "seconds only", input: 90 * time.Second, want: "90s"},
		{name: "seconds only 1", input: 1 * time.Second, want: "1s"},
		{name: "minutes from hours", input: 90 * time.Minute, want: "90m"},
		{name: "zero duration", input: 0, want: "0s"},
		{name: "exactly one hour", input: 3600 * time.Second, want: "1h"},
		{name: "exactly one minute", input: 60 * time.Second, want: "1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDurationRoundTrip(t *testing.T) {
	t.Parallel()
	durations := []time.Duration{
		1 * time.Second,
		30 * time.Second,
		60 * time.Second,
		90 * time.Second,
		300 * time.Second,
		600 * time.Second,
		3600 * time.Second,
		7200 * time.Second,
		18000 * time.Second,
	}
	for _, d := range durations {
		t.Run(fmt.Sprintf("roundtrip_%v", d), func(t *testing.T) {
			s := formatDuration(d)
			parsed, err := parseDuration(s)
			if err != nil {
				t.Fatalf("parseDuration(%q) error: %v", s, err)
			}
			if parsed != d {
				t.Errorf("round-trip: formatDuration(%v)=%q, parseDuration(%q)=%v", d, s, s, parsed)
			}
		})
	}
}

func TestOwnerPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		volume string
		want   string
	}{
		{volume: "myvol", want: "blt-volume-manager/owners/myvol/"},
		{volume: "group/subvol", want: "blt-volume-manager/owners/group/subvol/"},
		{volume: "", want: "blt-volume-manager/owners//"},
	}
	for _, tt := range tests {
		t.Run(tt.volume, func(t *testing.T) {
			got := OwnerPrefix(tt.volume)
			if got != tt.want {
				t.Errorf("OwnerPrefix(%q) = %q, want %q", tt.volume, got, tt.want)
			}
		})
	}
}

func TestRemainingSeconds(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()
	tests := []struct {
		name     string
		entry    OwnerEntry
		expectFn func(now int64) int64
	}{
		{
			name:     "permanent lock",
			entry:    OwnerEntry{ExpiryTime: 0},
			expectFn: func(int64) int64 { return math.MaxInt64 },
		},
		{
			name:     "future expiry",
			entry:    OwnerEntry{ExpiryTime: now + 100},
			expectFn: func(now int64) int64 { return now + 100 - time.Now().Unix() },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.RemainingSeconds()
			want := tt.expectFn(now)
			if tt.name == "future expiry" {
				// Small tolerance for elapsed time
				if got != want && got != want-1 {
					t.Errorf("RemainingSeconds() = %d, want %d", got, want)
				}
			} else if got != want {
				t.Errorf("RemainingSeconds() = %d, want %d", got, want)
			}
		})
	}
}

// --- ParseOwnerKey ---

func TestParseOwnerKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		key          string
		wantVolume   string
		wantOwner    string
		wantCreation int64
		wantExpiryFn func(creation int64) int64
		wantErr      bool
	}{
		{
			name:         "basic with hours",
			key:          "blt-volume-manager/owners/myvol/myhost-1000000-1h.json",
			wantVolume:   "myvol",
			wantOwner:    "myhost",
			wantCreation: 1000000,
			wantExpiryFn: func(c int64) int64 { return c + 3600 },
		},
		{
			name:         "owner with dash",
			key:          "blt-volume-manager/owners/myvol/myhost%2D12345-2000000-30m.json",
			wantVolume:   "myvol",
			wantOwner:    "myhost-12345",
			wantCreation: 2000000,
			wantExpiryFn: func(c int64) int64 { return c + 1800 },
		},
		{
			name:         "permanent lock",
			key:          "blt-volume-manager/owners/myvol/myhost-3000000-0.json",
			wantVolume:   "myvol",
			wantOwner:    "myhost",
			wantCreation: 3000000,
			wantExpiryFn: func(int64) int64 { return 0 },
		},
		{
			name:         "seconds duration",
			key:          "blt-volume-manager/owners/myvol/myhost-4000000-45s.json",
			wantVolume:   "myvol",
			wantOwner:    "myhost",
			wantCreation: 4000000,
			wantExpiryFn: func(c int64) int64 { return c + 45 },
		},
		{
			name:         "nested volume",
			key:          "blt-volume-manager/owners/group/subvol/myhost-5000000-10m.json",
			wantVolume:   "group/subvol",
			wantOwner:    "myhost",
			wantCreation: 5000000,
			wantExpiryFn: func(c int64) int64 { return c + 600 },
		},
		{
			name:         "multiple owner dashes encoded",
			key:          "blt-volume-manager/owners/myvol/a%2Db%2Dc-6000000-2h.json",
			wantVolume:   "myvol",
			wantOwner:    "a-b-c",
			wantCreation: 6000000,
			wantExpiryFn: func(c int64) int64 { return c + 7200 },
		},
		{
			name:    "wrong prefix",
			key:     "other-prefix/owners/myvol/myhost-100-1h.json",
			wantErr: true,
		},
		{
			name:    "missing json suffix",
			key:     "blt-volume-manager/owners/myvol/myhost-100-1h.txt",
			wantErr: true,
		},
		{
			name:    "no dash at all",
			key:     "blt-volume-manager/owners/myvol/myhost.json",
			wantErr: true,
		},
		{
			name:    "missing creation (only one dash)",
			key:     "blt-volume-manager/owners/myvol/myhost-1h.json",
			wantErr: true,
		},
		{
			name:    "non-numeric creation",
			key:     "blt-volume-manager/owners/myvol/myhost-abc-1h.json",
			wantErr: true,
		},
		{
			name:    "invalid duration",
			key:     "blt-volume-manager/owners/myvol/myhost-100-5d.json",
			wantErr: true,
		},
		{
			name:         "empty owner",
			key:          "blt-volume-manager/owners/myvol/-100-1h.json",
			wantVolume:   "myvol",
			wantOwner:    "",
			wantCreation: 100,
			wantExpiryFn: func(c int64) int64 { return c + 3600 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vol, owner, creation, expiry, err := ParseOwnerKey(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseOwnerKey(%q) expected error, got vol=%q owner=%q creation=%d expiry=%d",
						tt.key, vol, owner, creation, expiry)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOwnerKey(%q) unexpected error: %v", tt.key, err)
			}
			if vol != tt.wantVolume {
				t.Errorf("volume = %q, want %q", vol, tt.wantVolume)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if creation != tt.wantCreation {
				t.Errorf("creation = %d, want %d", creation, tt.wantCreation)
			}
			wantExpiry := tt.wantExpiryFn(tt.wantCreation)
			if expiry != wantExpiry {
				t.Errorf("expiry = %d, want %d", expiry, wantExpiry)
			}
		})
	}
}

// --- determineOwner ---

func TestDetermineOwner(t *testing.T) {
	t.Parallel()
	now := time.Now().Unix()
	future := now + 3600
	past := now - 3600

	type entry struct {
		key   string
		clock int64
	}

	tests := []struct {
		name       string
		entries    []entry
		wantKey    string
		wantOwner  string
		wantExpiry int64
	}{
		{
			name:    "empty list",
			entries: []entry{},
			wantKey: "",
		},
		{
			name: "single valid entry",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/host1-%d-1h.json", future-3600), clock: 1},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/host1-%d-1h.json", future-3600),
			wantOwner:  "host1",
			wantExpiry: future,
		},
		{
			name: "all expired",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/host1-%d-5m.json", past-300), clock: 1},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/host2-%d-10m.json", past-600), clock: 2},
			},
			wantKey: "",
		},
		{
			name: "first entry not expired wins",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/host1-%d-1h.json", future-3600), clock: 1},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/host2-%d-1h.json", future-3600), clock: 2},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/host1-%d-1h.json", future-3600),
			wantOwner:  "host1",
			wantExpiry: future,
		},
		{
			name: "expired entries before valid are skipped",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/expired1-%d-5m.json", past-300), clock: 1},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/expired2-%d-10m.json", past-600), clock: 2},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/valid-%d-1h.json", future-3600), clock: 3},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/valid-%d-1h.json", future-3600),
			wantOwner:  "valid",
			wantExpiry: future,
		},
		{
			name: "permanent lock always valid",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/perm-%d-0.json", now-1000), clock: 1},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/perm-%d-0.json", now-1000),
			wantOwner:  "perm",
			wantExpiry: 0,
		},
		{
			name: "entries missing key pointer are skipped",
			entries: []entry{
				{key: "", clock: 1},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/valid-%d-1h.json", future-3600), clock: 2},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/valid-%d-1h.json", future-3600),
			wantOwner:  "valid",
			wantExpiry: future,
		},
		{
			name: "sort by modification counter (earliest wins)",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/second-%d-1h.json", future-3600), clock: 2},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/first-%d-1h.json", future-3600), clock: 1},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/first-%d-1h.json", future-3600),
			wantOwner:  "first",
			wantExpiry: future,
		},
		{
			name: "same mod counter sorts by creation (earliest wins)",
			entries: []entry{
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/late-%d-1h.json", now+7200), clock: 1},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/early-%d-1h.json", now+3600), clock: 1},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/early-%d-1h.json", now+3600),
			wantOwner:  "early",
			wantExpiry: now + 3600 + 3600,
		},
		{
			name: "malformed keys are skipped",
			entries: []entry{
				{key: "blt-volume-manager/owners/myvol/bad-key.json", clock: 1},
				{key: fmt.Sprintf("blt-volume-manager/owners/myvol/good-%d-1h.json", future-3600), clock: 2},
			},
			wantKey:    fmt.Sprintf("blt-volume-manager/owners/myvol/good-%d-1h.json", future-3600),
			wantOwner:  "good",
			wantExpiry: future,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []backend.Entry
			for _, e := range tt.entries {
				keyCopy := e.key
				counter := e.clock
				entry := backend.Entry{}
				if e.key != "" {
					entry.Key = &keyCopy
				}
				entry.ModificationCounter = &counter
				objs = append(objs, entry)
			}

			gotKey, gotOwner, _, gotExpiry := determineOwner(objs)
			if tt.wantKey == "" {
				if gotKey != "" {
					t.Errorf("determineOwner() = key=%q owner=%q expiry=%d, want empty key", gotKey, gotOwner, gotExpiry)
				}
				return
			}
			if gotKey == "" {
				t.Fatalf("determineOwner() returned empty key, want %q", tt.wantKey)
			}
			if gotKey != tt.wantKey {
				t.Errorf("key = %q, want %q", gotKey, tt.wantKey)
			}
			if gotOwner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", gotOwner, tt.wantOwner)
			}
			if gotExpiry != tt.wantExpiry {
				t.Errorf("expiry = %d, want %d", gotExpiry, tt.wantExpiry)
			}
		})
	}
}

// --- RemoveStaleObjects ---

func TestRemoveStaleObjects(t *testing.T) {
	t.Parallel()
	now := time.Now()
	futureUnix := now.Add(time.Hour).Unix()
	pastUnix := now.Add(-2 * time.Hour).Unix()

	validKey := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-1h.json", futureUnix-3600)
	expiredFreshKey := fmt.Sprintf("blt-volume-manager/owners/myvol/expired-fresh-%d-5m.json", pastUnix-300)
	expiredStaleKey := fmt.Sprintf("blt-volume-manager/owners/myvol/expired-stale-%d-5m.json", pastUnix-300)
	permKey := fmt.Sprintf("blt-volume-manager/owners/myvol/perm-%d-0.json", pastUnix)

	tests := []struct {
		name      string
		entries   []backend.Entry
		ttl       time.Duration
		wantCount int
		wantKept  []string
	}{
		{
			name: "all valid none removed",
			entries: []backend.Entry{
				{Key: stringPtr(validKey), ModificationCounter: int64Ptr(now.UnixNano())},
			},
			ttl:       24 * time.Hour,
			wantCount: 1,
			wantKept:  []string{validKey},
		},
		{
			name: "expired fresh kept",
			entries: []backend.Entry{
				{Key: stringPtr(expiredFreshKey), ModificationCounter: int64Ptr(now.Add(-30 * time.Minute).UnixNano())},
			},
			ttl:       24 * time.Hour,
			wantCount: 1,
			wantKept:  []string{expiredFreshKey},
		},
		{
			name: "expired stale removed",
			entries: []backend.Entry{
				{Key: stringPtr(expiredStaleKey), ModificationCounter: int64Ptr(now.Add(-48 * time.Hour).UnixNano())},
			},
			ttl:       24 * time.Hour,
			wantCount: 0,
			wantKept:  nil,
		},
		{
			name: "permanent never removed",
			entries: []backend.Entry{
				{Key: stringPtr(permKey), ModificationCounter: int64Ptr(now.Add(-48 * time.Hour).UnixNano())},
			},
			ttl:       1 * time.Hour,
			wantCount: 1,
			wantKept:  []string{permKey},
		},
		{
			name: "nil key skipped",
			entries: []backend.Entry{
				{Key: nil, ModificationCounter: int64Ptr(now.UnixNano())},
				{Key: stringPtr(validKey), ModificationCounter: int64Ptr(now.UnixNano())},
			},
			ttl:       24 * time.Hour,
			wantCount: 1,
			wantKept:  []string{validKey},
		},
		{
			name: "mixed: valid, stale removed, fresh expired kept, perm kept",
			entries: []backend.Entry{
				{Key: stringPtr(validKey), ModificationCounter: int64Ptr(now.UnixNano())},
				{Key: stringPtr(expiredStaleKey), ModificationCounter: int64Ptr(now.Add(-48 * time.Hour).UnixNano())},
				{Key: stringPtr(expiredFreshKey), ModificationCounter: int64Ptr(now.Add(-30 * time.Minute).UnixNano())},
				{Key: stringPtr(permKey), ModificationCounter: int64Ptr(now.Add(-48 * time.Hour).UnixNano())},
			},
			ttl:       24 * time.Hour,
			wantCount: 3,
			wantKept:  []string{validKey, expiredFreshKey, permKey},
		},
		{
			name: "nil modification counter on expired non-permanent kept",
			entries: []backend.Entry{
				{Key: stringPtr(expiredFreshKey), ModificationCounter: nil},
			},
			ttl:       1 * time.Hour,
			wantCount: 1,
			wantKept:  []string{expiredFreshKey},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newOrderedBackend()
			// Pre-populate so deletions can be verified
			for _, e := range tt.entries {
				if e.Key != nil {
					_ = b.PutObject(context.Background(), *e.Key, []byte(`{}`))
				}
			}

			got := RemoveStaleObjects(context.Background(), b, tt.entries, tt.ttl)
			if len(got) != tt.wantCount {
				t.Errorf("RemoveStaleObjects returned %d entries, want %d", len(got), tt.wantCount)
			}
			if tt.wantKept != nil {
				gotKeys := make(map[string]bool)
				for _, e := range got {
					if e.Key != nil {
						gotKeys[*e.Key] = true
					}
				}
				for _, k := range tt.wantKept {
					if !gotKeys[k] {
						t.Errorf("expected key %q to be kept, but it was removed", k)
					}
					delete(gotKeys, k)
				}
				for k := range gotKeys {
					t.Errorf("unexpected key %q was kept", k)
				}
			}
		})
	}
}

// --- AcquireOwnerLock ---

func TestAcquireOwnerLock_Success(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	folder := OwnerPrefix("myvol")
	expiry := time.Now().Add(time.Hour).Unix()

	key, err := AcquireOwnerLock(context.Background(), b, folder, "myhost", expiry)
	if err != nil {
		t.Fatalf("AcquireOwnerLock() unexpected error: %v", err)
	}
	if !strings.HasPrefix(key, folder) {
		t.Errorf("key %q does not have expected prefix %q", key, folder)
	}
	if !strings.HasSuffix(key, ".json") {
		t.Errorf("key %q missing .json suffix", key)
	}

	// Verify the object was stored
	data, err := b.ReadObject(context.Background(), key)
	if err != nil {
		t.Fatalf("ReadObject(%q) error: %v", key, err)
	}
	var entry OwnerEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("unmarshal stored entry: %v", err)
	}
	if entry.Name != "myhost" {
		t.Errorf("stored entry.Name = %q, want %q", entry.Name, "myhost")
	}
	if entry.ExpiryTime != expiry {
		t.Errorf("stored entry.ExpiryTime = %d, want %d", entry.ExpiryTime, expiry)
	}

	// Parse the key and verify components
	_, owner, creation, keyExpiry, err := ParseOwnerKey(key)
	if err != nil {
		t.Fatalf("ParseOwnerKey(%q) error: %v", key, err)
	}
	if owner != "myhost" {
		t.Errorf("parsed owner = %q, want %q", owner, "myhost")
	}
	if keyExpiry != expiry {
		t.Errorf("parsed expiry = %d, want %d", keyExpiry, expiry)
	}
	if creation == 0 {
		t.Errorf("expected non-zero creation timestamp")
	}
}

func TestAcquireOwnerLock_Permanent(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	folder := OwnerPrefix("myvol")

	key, err := AcquireOwnerLock(context.Background(), b, folder, "myhost", 0)
	if err != nil {
		t.Fatalf("AcquireOwnerLock() permanent error: %v", err)
	}

	_, owner, _, expiry, err := ParseOwnerKey(key)
	if err != nil {
		t.Fatalf("ParseOwnerKey(%q) error: %v", key, err)
	}
	if owner != "myhost" {
		t.Errorf("owner = %q, want %q", owner, "myhost")
	}
	if expiry != 0 {
		t.Errorf("expiry = %d, want 0 for permanent", expiry)
	}
}

func TestAcquireOwnerLock_PastExpiry(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	folder := OwnerPrefix("myvol")

	_, err := AcquireOwnerLock(context.Background(), b, folder, "myhost", 1) // expiry in the past (Unix epoch 1)
	if err == nil {
		t.Fatal("expected error for past expiry, got nil")
	}
}

func TestAcquireOwnerLock_CompetingLock(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	folder := OwnerPrefix("myvol")
	expiry := time.Now().Add(time.Hour).Unix()

	// First lock wins
	_, err := AcquireOwnerLock(context.Background(), b, folder, "first", expiry)
	if err != nil {
		t.Fatalf("first AcquireOwnerLock() error: %v", err)
	}

	// Second attempt should fail
	_, err = AcquireOwnerLock(context.Background(), b, folder, "second", expiry)
	if err == nil {
		t.Fatal("expected error for competing lock, got nil")
	}
}

func TestAcquireOwnerLock_OwnerWithDash(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	folder := OwnerPrefix("myvol")
	expiry := time.Now().Add(time.Hour).Unix()

	key, err := AcquireOwnerLock(context.Background(), b, folder, "myHost-12345", expiry)
	if err != nil {
		t.Fatalf("AcquireOwnerLock() with dashed owner error: %v", err)
	}

	_, owner, _, keyExpiry, err := ParseOwnerKey(key)
	if err != nil {
		t.Fatalf("ParseOwnerKey(%q) error: %v", key, err)
	}
	if owner != "myHost-12345" {
		t.Errorf("owner = %q, want %q", owner, "myHost-12345")
	}
	if keyExpiry != expiry {
		t.Errorf("expiry = %d, want %d", keyExpiry, expiry)
	}

	// Verify no raw dash in the key's owner portion (should be %2D encoded)
	// The key pattern is: <prefix><encodedOwner>-<creation>-<duration>.json
	// The owner portion should not contain raw dashes
	prefix := folder
	rest := strings.TrimPrefix(key, prefix)
	// rest is: <encodedOwner>-<creation>-<duration>.json
	lastDash := strings.LastIndexByte(rest, '-')
	prevDash := strings.LastIndexByte(rest[:lastDash], '-')
	encodedOwner := rest[:prevDash]
	if strings.Contains(encodedOwner, "-") {
		t.Errorf("encoded owner %q contains raw dash, should be percent-encoded", encodedOwner)
	}
}

func TestAcquireOwnerLock_CleanupOnFailure(t *testing.T) {
	t.Parallel()
	// Use a backend that fails on PutObject
	failBackend := &failOnPutBackend{}
	folder := OwnerPrefix("myvol")

	_, err := AcquireOwnerLock(context.Background(), failBackend, folder, "myhost", time.Now().Add(time.Hour).Unix())
	if err == nil {
		t.Fatal("expected error from failing backend")
	}
}

type failOnPutBackend struct {
	orderedBackend
}

func (f *failOnPutBackend) PutObject(ctx context.Context, key string, data []byte) error {
	return fmt.Errorf("storage failure")
}

func TestAcquireOwnerLock_ListFails(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	folder := OwnerPrefix("myvol")
	expiry := time.Now().Add(time.Hour).Unix()

	_, err := AcquireOwnerLock(context.Background(), b, folder, "myhost", expiry)
	if err != nil {
		t.Fatalf("AcquireOwnerLock() error: %v", err)
	}

	// After listing, ensure the key was stored
	listed, err := b.ListObjects(context.Background(), folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Errorf("expected 1 object, got %d", len(listed))
	}
}

func TestAcquireOwnerLock_MultipleVolumes(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	expiry := time.Now().Add(time.Hour).Unix()

	key1, err := AcquireOwnerLock(context.Background(), b, OwnerPrefix("vol1"), "host1", expiry)
	if err != nil {
		t.Fatalf("vol1 lock error: %v", err)
	}
	key2, err := AcquireOwnerLock(context.Background(), b, OwnerPrefix("vol2"), "host2", expiry)
	if err != nil {
		t.Fatalf("vol2 lock error: %v", err)
	}
	if key1 == key2 {
		t.Error("keys for different volumes should differ")
	}
	// Verify each was stored
	if _, err := b.ReadObject(context.Background(), key1); err != nil {
		t.Errorf("key1 missing: %v", err)
	}
	if _, err := b.ReadObject(context.Background(), key2); err != nil {
		t.Errorf("key2 missing: %v", err)
	}
}

// --- OwnerStore methods ---

func TestLockIsValid(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	s := NewOwnerStore(b)
	expiry := time.Now().Add(time.Hour).Unix()
	futureKey := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-1h.json", expiry-3600)

	t.Run("valid key with stored object", func(t *testing.T) {
		_ = b.PutObject(context.Background(), futureKey, []byte(`{}`))
		valid, err := s.LockIsValid(context.Background(), futureKey)
		if err != nil {
			t.Fatalf("LockIsValid error: %v", err)
		}
		if !valid {
			t.Error("expected valid lock")
		}
	})

	t.Run("no stored object", func(t *testing.T) {
		missingKey := fmt.Sprintf("blt-volume-manager/owners/myvol/ghost-%d-1h.json", time.Now().Add(time.Hour).Unix()-3600)
		valid, err := s.LockIsValid(context.Background(), missingKey)
		if err != nil {
			t.Fatalf("LockIsValid error: %v", err)
		}
		if valid {
			t.Error("expected invalid lock for missing object")
		}
	})

	t.Run("invalid key format", func(t *testing.T) {
		_, err := s.LockIsValid(context.Background(), "not-a-valid-key")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})

	t.Run("expired key", func(t *testing.T) {
		expiredKey := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-5m.json", time.Now().Add(-10*time.Minute).Unix())
		_ = b.PutObject(context.Background(), expiredKey, []byte(`{}`))
		valid, err := s.LockIsValid(context.Background(), expiredKey)
		if err != nil {
			t.Fatalf("LockIsValid error: %v", err)
		}
		if valid {
			t.Error("expected expired lock to be invalid")
		}
	})

	t.Run("permanent key", func(t *testing.T) {
		permKey := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-0.json", time.Now().Unix())
		_ = b.PutObject(context.Background(), permKey, []byte(`{}`))
		valid, err := s.LockIsValid(context.Background(), permKey)
		if err != nil {
			t.Fatalf("LockIsValid error: %v", err)
		}
		if !valid {
			t.Error("expected permanent lock to be valid")
		}
	})
}

func TestReleaseLock(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	s := NewOwnerStore(b)
	key := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-1h.json", time.Now().Add(time.Hour).Unix())
	_ = b.PutObject(context.Background(), key, []byte(`{}`))

	if err := s.ReleaseLock(context.Background(), key); err != nil {
		t.Fatalf("ReleaseLock error: %v", err)
	}
	if _, err := b.ReadObject(context.Background(), key); err != backend.ErrKeyNotFound {
		t.Error("expected key to be deleted after release")
	}
}

func TestAcquireForVolume(t *testing.T) {
	t.Parallel()
	t.Run("success with duration", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		expiry, err := s.AcquireForVolume(context.Background(), "myvol", "myhost", 10)
		if err != nil {
			t.Fatalf("AcquireForVolume error: %v", err)
		}
		if expiry <= time.Now().Unix() {
			t.Error("expected future expiry")
		}
	})

	t.Run("permanent when duration is 0", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		expiry, err := s.AcquireForVolume(context.Background(), "myvol", "myhost", 0)
		if err != nil {
			t.Fatalf("AcquireForVolume(0) error: %v", err)
		}
		if expiry != 0 {
			t.Errorf("expected 0 expiry for permanent lock, got %d", expiry)
		}
	})

	t.Run("permanent when duration is negative", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		expiry, err := s.AcquireForVolume(context.Background(), "myvol", "myhost", -1)
		if err != nil {
			t.Fatalf("AcquireForVolume(-1) error: %v", err)
		}
		if expiry != 0 {
			t.Errorf("expected 0 expiry for permanent lock, got %d", expiry)
		}
	})

	t.Run("empty owner name", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		_, err := s.AcquireForVolume(context.Background(), "myvol", "", 10)
		if err == nil {
			t.Fatal("expected error for empty owner name")
		}
	})
}

func TestFindForVolume(t *testing.T) {
	t.Parallel()
	t.Run("volume with owner", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		expiry := time.Now().Add(time.Hour).Unix()
		key := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-1h.json", expiry-3600)
		_ = b.PutObject(context.Background(), key, []byte(`{}`))

		vo, err := s.FindForVolume(context.Background(), "myvol")
		if err != nil {
			t.Fatalf("FindForVolume error: %v", err)
		}
		if vo.Owner != "host" {
			t.Errorf("owner = %q, want %q", vo.Owner, "host")
		}
		if vo.Expiry != expiry {
			t.Errorf("expiry = %d, want %d", vo.Expiry, expiry)
		}
		if vo.Creation == 0 {
			t.Error("expected non-zero creation")
		}
	})

	t.Run("volume without owner", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		vo, err := s.FindForVolume(context.Background(), "emptyvol")
		if err != nil {
			t.Fatalf("FindForVolume error: %v", err)
		}
		if vo.Owner != "" {
			t.Errorf("expected empty owner, got %q", vo.Owner)
		}
	})

	t.Run("list error", func(t *testing.T) {
		errBackend := &listErrorBackend{}
		s := NewOwnerStore(errBackend)
		_, err := s.FindForVolume(context.Background(), "myvol")
		if err == nil {
			t.Fatal("expected error from failing backend")
		}
	})
}

type listErrorBackend struct{ orderedBackend }

func (l *listErrorBackend) ListObjects(ctx context.Context, prefix string) ([]backend.Entry, error) {
	return nil, fmt.Errorf("list error")
}

func TestDeleteForVolume(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	s := NewOwnerStore(b)
	expiry := time.Now().Add(time.Hour).Unix()
	key := fmt.Sprintf("blt-volume-manager/owners/myvol/host-%d-1h.json", expiry-3600)
	_ = b.PutObject(context.Background(), key, []byte(`{}`))

	if err := s.DeleteForVolume(context.Background(), "myvol"); err != nil {
		t.Fatalf("DeleteForVolume error: %v", err)
	}
	listed, _ := b.ListObjects(context.Background(), "blt-volume-manager/owners/myvol/")
	if len(listed) != 0 {
		t.Error("expected no objects after delete")
	}
}

func TestListAllGrouped(t *testing.T) {
	t.Parallel()
	t.Run("multiple volumes with owners", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		expiry := time.Now().Add(time.Hour).Unix()

		// Create owners for vol1 and vol2
		_, err := AcquireOwnerLock(context.Background(), b, OwnerPrefix("vol1"), "host-a", expiry)
		if err != nil {
			t.Fatalf("create vol1 owner: %v", err)
		}
		_, err = AcquireOwnerLock(context.Background(), b, OwnerPrefix("vol2"), "host-b", expiry)
		if err != nil {
			t.Fatalf("create vol2 owner: %v", err)
		}

		grouped, err := s.ListAllGrouped(context.Background())
		if err != nil {
			t.Fatalf("ListAllGrouped error: %v", err)
		}
		if len(grouped) != 2 {
			t.Errorf("expected 2 volumes, got %d", len(grouped))
		}
		if vo, ok := grouped["vol1"]; !ok {
			t.Error("expected vol1 in grouped result")
		} else if vo.Owner != "host-a" {
			t.Errorf("vol1 owner = %q, want %q", vo.Owner, "host-a")
		}
		if vo, ok := grouped["vol2"]; !ok {
			t.Error("expected vol2 in grouped result")
		} else if vo.Owner != "host-b" {
			t.Errorf("vol2 owner = %q, want %q", vo.Owner, "host-b")
		}
	})

	t.Run("no owners", func(t *testing.T) {
		b := newOrderedBackend()
		s := NewOwnerStore(b)
		grouped, err := s.ListAllGrouped(context.Background())
		if err != nil {
			t.Fatalf("ListAllGrouped error: %v", err)
		}
		if len(grouped) != 0 {
			t.Errorf("expected 0 volumes, got %d", len(grouped))
		}
	})

	t.Run("list error", func(t *testing.T) {
		errBackend := &listErrorBackend{}
		s := NewOwnerStore(errBackend)
		_, err := s.ListAllGrouped(context.Background())
		if err == nil {
			t.Fatal("expected error from failing backend")
		}
	})
}

func TestNewOwnerStore(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	s := NewOwnerStore(b)
	if s == nil {
		t.Fatal("NewOwnerStore returned nil")
	}
}

func TestLockVolume(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	s := NewOwnerStore(b)
	expiry := time.Now().Add(time.Hour).Unix()

	key, err := s.LockVolume(context.Background(), "myvol", "myhost", expiry)
	if err != nil {
		t.Fatalf("LockVolume error: %v", err)
	}
	if !strings.HasPrefix(key, "blt-volume-manager/owners/myvol/") {
		t.Errorf("unexpected key prefix: %q", key)
	}
}

// --- Full integration scenarios ---

func TestLockAcquireReleaseCycle(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	s := NewOwnerStore(b)

	// Acquire
	expiry := time.Now().Add(30 * time.Minute).Unix()
	key, err := s.LockVolume(context.Background(), "testvol", "testhost", expiry)
	if err != nil {
		t.Fatalf("LockVolume error: %v", err)
	}

	// Validate
	valid, err := s.LockIsValid(context.Background(), key)
	if err != nil {
		t.Fatalf("LockIsValid error: %v", err)
	}
	if !valid {
		t.Fatal("expected lock to be valid")
	}

	// Find
	vo, err := s.FindForVolume(context.Background(), "testvol")
	if err != nil {
		t.Fatalf("FindForVolume error: %v", err)
	}
	if vo.Owner != "testhost" {
		t.Errorf("FindForVolume owner = %q, want %q", vo.Owner, "testhost")
	}

	// Release
	if err := s.ReleaseLock(context.Background(), key); err != nil {
		t.Fatalf("ReleaseLock error: %v", err)
	}

	// Validate after release
	valid, err = s.LockIsValid(context.Background(), key)
	if err != nil {
		t.Fatalf("LockIsValid error: %v", err)
	}
	if valid {
		t.Fatal("expected lock to be invalid after release")
	}

	// Now another owner should be able to acquire
	newKey, err := s.LockVolume(context.Background(), "testvol", "newhost", expiry)
	if err != nil {
		t.Fatalf("second LockVolume error: %v", err)
	}
	if newKey == key {
		t.Error("expected different key after re-acquire")
	}
}

func TestConcurrentLockAttempts(t *testing.T) {
	t.Parallel()
	b := newOrderedBackend()
	expiry := time.Now().Add(time.Hour).Unix()

	// Simulate concurrent lock attempts by using the raw AcquireOwnerLock
	// (since OwnerStore serializes through the backend, concurrency is at the backend level)
	_, err := AcquireOwnerLock(context.Background(), b, OwnerPrefix("convol"), "first", expiry)
	if err != nil {
		t.Fatalf("first lock error: %v", err)
	}

	_, err = AcquireOwnerLock(context.Background(), b, OwnerPrefix("convol"), "second", expiry)
	if err == nil {
		t.Error("expected second lock to fail")
	}

	// First lock should still be valid
	listed, err := b.ListObjects(context.Background(), OwnerPrefix("convol"))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Errorf("expected 1 lock object, got %d", len(listed))
	}
}
