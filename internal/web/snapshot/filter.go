package snapshot

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type VersionRange struct {
	Major int
	Minor int
}

type SnapshotFilter struct {
	TimeFrom, TimeTo           *time.Time
	TimeOfDayFrom, TimeOfDayTo *int
	VersionFrom, VersionTo     *VersionRange
	Query                      *string
}

func ParseVersionParam(s string) (major, minor int, ok bool) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || maj < 0 || min < 0 {
		return 0, 0, false
	}
	return maj, min, true
}

func ParseVersionTag(tags []string) (major, minor int, ok bool) {
	for _, t := range tags {
		if rest, found := strings.CutPrefix(t, "v"); found {
			if maj, min, ok := ParseVersionParam(rest); ok {
				return maj, min, true
			}
		}
	}
	return 0, 0, false
}

func ApplySnapshotFilter(snaps []restic.Snapshot, f *SnapshotFilter) []restic.Snapshot {
	if f == nil {
		return snaps
	}
	out := make([]restic.Snapshot, 0, len(snaps))
	for _, sn := range snaps {
		if f.TimeFrom != nil && sn.Time.Before(*f.TimeFrom) {
			continue
		}
		if f.TimeTo != nil && sn.Time.After(*f.TimeTo) {
			continue
		}
		if f.TimeOfDayFrom != nil || f.TimeOfDayTo != nil {
			snSeconds := sn.Time.Hour()*3600 + sn.Time.Minute()*60 + sn.Time.Second()
			if f.TimeOfDayFrom != nil && snSeconds < *f.TimeOfDayFrom {
				continue
			}
			if f.TimeOfDayTo != nil && snSeconds > *f.TimeOfDayTo {
				continue
			}
		}
		if f.VersionFrom != nil || f.VersionTo != nil {
			maj, min, ok := ParseVersionTag(sn.Tags)
			if !ok {
				continue
			}
			if f.VersionFrom != nil {
				if maj < f.VersionFrom.Major || (maj == f.VersionFrom.Major && min < f.VersionFrom.Minor) {
					continue
				}
			}
			if f.VersionTo != nil {
				if maj > f.VersionTo.Major || (maj == f.VersionTo.Major && min > f.VersionTo.Minor) {
					continue
				}
			}
		}
		if f.Query != nil && *f.Query != "" {
			q := strings.ToLower(*f.Query)
			idMatch := strings.Contains(strings.ToLower(sn.ID), q)
			shortMatch := strings.Contains(strings.ToLower(sn.ShortID), q)
			tagMatch := false
			for _, t := range sn.Tags {
				if strings.Contains(strings.ToLower(t), q) {
					tagMatch = true
					break
				}
			}
			hostMatch := strings.Contains(strings.ToLower(sn.Hostname), q)
			if !idMatch && !shortMatch && !tagMatch && !hostMatch {
				continue
			}
		}
		out = append(out, sn)
	}
	return out
}

func parsePagination(r *http.Request) (offset, limit int) {
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	return
}

func ParseSnapshotListOpts(r *http.Request) (*restic.ListSnapshotsOpts, *SnapshotFilter, int, int) {
	hosts := r.URL.Query()["host"]
	tags := r.URL.Query()["tag"]

	offset, limit := parsePagination(r)

	var filter *SnapshotFilter
	ensureFilter := func() {
		if filter == nil {
			filter = &SnapshotFilter{}
		}
	}

	if tf := r.URL.Query().Get("timestampFrom"); tf != "" {
		if n, err := strconv.ParseInt(tf, 10, 64); err == nil {
			t := time.UnixMilli(n)
			ensureFilter()
			filter.TimeFrom = &t
		}
	}
	if tf := r.URL.Query().Get("timestampTo"); tf != "" {
		if n, err := strconv.ParseInt(tf, 10, 64); err == nil {
			t := time.UnixMilli(n)
			ensureFilter()
			filter.TimeTo = &t
		}
	}
	if tod := r.URL.Query().Get("timeOfDayFrom"); tod != "" {
		if n, err := strconv.Atoi(tod); err == nil {
			ensureFilter()
			filter.TimeOfDayFrom = &n
		}
	}
	if tod := r.URL.Query().Get("timeOfDayTo"); tod != "" {
		if n, err := strconv.Atoi(tod); err == nil {
			ensureFilter()
			filter.TimeOfDayTo = &n
		}
	}
	if vf := r.URL.Query().Get("versionFrom"); vf != "" {
		if maj, min, ok := ParseVersionParam(vf); ok {
			ensureFilter()
			filter.VersionFrom = &VersionRange{Major: maj, Minor: min}
		}
	}
	if vt := r.URL.Query().Get("versionTo"); vt != "" {
		if maj, min, ok := ParseVersionParam(vt); ok {
			ensureFilter()
			filter.VersionTo = &VersionRange{Major: maj, Minor: min}
		}
	}

	if q := r.URL.Query().Get("query"); q != "" {
		ensureFilter()
		filter.Query = &q
	}

	latest := 0
	if limit > 0 && filter == nil {
		latest = offset + limit + 1
	}

	var opts *restic.ListSnapshotsOpts
	if len(hosts) > 0 || latest > 0 || len(tags) > 0 {
		opts = &restic.ListSnapshotsOpts{Hosts: hosts, Latest: latest, Tags: tags}
	}
	return opts, filter, offset, limit
}
