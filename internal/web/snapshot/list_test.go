package snapshot

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
)

func TestParseVersionParam(t *testing.T) {
	tests := []struct {
		input  string
		major  int
		minor  int
		wantOK bool
	}{
		{"1.2", 1, 2, true},
		{"0.0", 0, 0, true},
		{"12.34", 12, 34, true},
		{"", 0, 0, false},
		{"1", 0, 0, false},
		{"1.2.3", 0, 0, false},
		{"abc.def", 0, 0, false},
		{"-1.2", 0, 0, false},
		{"1.-2", 0, 0, false},
	}
	for _, tt := range tests {
		maj, min, ok := ParseVersionParam(tt.input)
		if ok != tt.wantOK {
			t.Errorf("ParseVersionParam(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
		}
		if maj != tt.major || min != tt.minor {
			t.Errorf("ParseVersionParam(%q) = (%d,%d), want (%d,%d)", tt.input, maj, min, tt.major, tt.minor)
		}
	}
}

func TestParseVersionTag(t *testing.T) {
	tests := []struct {
		tags   []string
		major  int
		minor  int
		wantOK bool
	}{
		{[]string{"v1.2"}, 1, 2, true},
		{[]string{"v0.0"}, 0, 0, true},
		{[]string{"v12.34"}, 12, 34, true},
		{[]string{"backup:hot", "v3.4"}, 3, 4, true},
		{[]string{}, 0, 0, false},
		{[]string{"backup:hot"}, 0, 0, false},
		{[]string{"vabc"}, 0, 0, false},
		{[]string{"1.2"}, 0, 0, false},
	}
	for _, tt := range tests {
		maj, min, ok := ParseVersionTag(tt.tags)
		if ok != tt.wantOK {
			t.Errorf("ParseVersionTag(%v) ok=%v, want %v", tt.tags, ok, tt.wantOK)
		}
		if maj != tt.major || min != tt.minor {
			t.Errorf("ParseVersionTag(%v) = (%d,%d), want (%d,%d)", tt.tags, maj, min, tt.major, tt.minor)
		}
	}
}

func TestApplySnapshotFilter_Nil(t *testing.T) {
	snaps := []restic.Snapshot{
		{ID: "a", Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", Time: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := ApplySnapshotFilter(snaps, nil)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestApplySnapshotFilter_TimeFrom(t *testing.T) {
	t1 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	snaps := []restic.Snapshot{
		{ID: "a", Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", Time: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{TimeFrom: &t1})
	if len(got) != 1 || got[0].ID != "b" {
		t.Errorf("expected [b], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_TimeTo(t *testing.T) {
	t1 := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	snaps := []restic.Snapshot{
		{ID: "a", Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", Time: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{TimeTo: &t1})
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("expected [a], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_TimeOfDay(t *testing.T) {
	morning := 8 * 3600
	afternoon := 14 * 3600
	snaps := []restic.Snapshot{
		{ID: "a", Time: time.Date(2024, 1, 1, 6, 0, 0, 0, time.UTC)},
		{ID: "b", Time: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)},
		{ID: "c", Time: time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC)},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{TimeOfDayFrom: &morning, TimeOfDayTo: &afternoon})
	if len(got) != 1 || got[0].ID != "b" {
		t.Errorf("expected [b], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_VersionRange(t *testing.T) {
	snaps := []restic.Snapshot{
		{ID: "a", Tags: []string{"v1.0"}},
		{ID: "b", Tags: []string{"v1.5"}},
		{ID: "c", Tags: []string{"v2.0"}},
		{ID: "d", Tags: []string{"backup:hot"}},
	}
	from := &VersionRange{Major: 1, Minor: 3}
	to := &VersionRange{Major: 2, Minor: 0}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{VersionFrom: from, VersionTo: to})
	if len(got) != 2 || got[0].ID != "b" || got[1].ID != "c" {
		t.Errorf("expected [b,c], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_Query(t *testing.T) {
	q := "abc"
	snaps := []restic.Snapshot{
		{ID: "abc123", ShortID: "abc123"},
		{ID: "def456", ShortID: "def456"},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{Query: &q})
	if len(got) != 1 || got[0].ID != "abc123" {
		t.Errorf("expected [abc123], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_QueryMatchTag(t *testing.T) {
	q := "prod"
	snaps := []restic.Snapshot{
		{ID: "a", Tags: []string{"env:prod"}},
		{ID: "b", Tags: []string{"env:dev"}},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{Query: &q})
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("expected [a], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_QueryMatchHostname(t *testing.T) {
	q := "server1"
	snaps := []restic.Snapshot{
		{ID: "a", Hostname: "server1"},
		{ID: "b", Hostname: "server2"},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{Query: &q})
	if len(got) != 1 || got[0].ID != "a" {
		t.Errorf("expected [a], got %v", ids(got))
	}
}

func TestApplySnapshotFilter_QueryCaseInsensitive(t *testing.T) {
	q := "ABC"
	snaps := []restic.Snapshot{
		{ID: "abc123", ShortID: "abc123"},
	}
	got := ApplySnapshotFilter(snaps, &SnapshotFilter{Query: &q})
	if len(got) != 1 {
		t.Errorf("expected 1 result, got %d", len(got))
	}
}

func TestParseSnapshotListOpts(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantHosts  []string
		wantTags   []string
		wantOffset int
		wantLimit  int
		wantFilter bool
	}{
		{"no params", "/api/snapshots?volume=test", nil, nil, 0, 0, false},
		{"hosts", "/api/snapshots?volume=test&host=h1&host=h2", []string{"h1", "h2"}, nil, 0, 0, false},
		{"tags", "/api/snapshots?volume=test&tag=v1.0&tag=backup:hot", nil, []string{"v1.0", "backup:hot"}, 0, 0, false},
		{"offset limit", "/api/snapshots?volume=test&offset=10&limit=5", nil, nil, 10, 5, false},
		{"timeFrom", "/api/snapshots?volume=test&timeFrom=1700000000000", nil, nil, 0, 0, true},
		{"timeOfDay", "/api/snapshots?volume=test&timeOfDayFrom=3600", nil, nil, 0, 0, true},
		{"versionFrom", "/api/snapshots?volume=test&versionFrom=1.0", nil, nil, 0, 0, true},
		{"query", "/api/snapshots?volume=test&query=search", nil, nil, 0, 0, true},
		{"negative offset", "/api/snapshots?volume=test&offset=-1", nil, nil, 0, 0, false},
		{"zero limit", "/api/snapshots?volume=test&limit=0", nil, nil, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.query, nil)
			opts, filter, offset, limit := ParseSnapshotListOpts(r)

			if tt.wantOffset != offset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
			if tt.wantLimit != limit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if tt.wantFilter && filter == nil {
				t.Error("expected non-nil filter")
			}
			if !tt.wantFilter && filter != nil {
				t.Error("expected nil filter")
			}
			if opts == nil && (len(tt.wantHosts) > 0 || len(tt.wantTags) > 0) {
				t.Error("expected non-nil opts")
			}
			if opts != nil {
				if len(tt.wantHosts) > 0 {
					if len(opts.Hosts) != len(tt.wantHosts) {
						t.Errorf("hosts = %v, want %v", opts.Hosts, tt.wantHosts)
					}
				}
				if len(tt.wantTags) > 0 {
					if len(opts.Tags) != len(tt.wantTags) {
						t.Errorf("tags = %v, want %v", opts.Tags, tt.wantTags)
					}
				}
			}
		})
	}
}

func ids(snaps []restic.Snapshot) []string {
	out := make([]string, len(snaps))
	for i, s := range snaps {
		out[i] = s.ID
	}
	return out
}
