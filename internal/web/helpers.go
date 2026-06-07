package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func requireVolumeParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	vol := r.URL.Query().Get("volume")
	if vol == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return "", false
	}
	return vol, true
}

type VersionRange struct {
	Major int
	Minor int
}

type SnapshotFilter struct {
	TimeFrom, TimeTo           *time.Time
	TimeOfDayFrom, TimeOfDayTo *int
	VersionFrom, VersionTo     *VersionRange
}

func parseVersionParam(s string) (major, minor int, ok bool) {
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

func parseVersionTag(tags []string) (major, minor int, ok bool) {
	for _, t := range tags {
		if rest, found := strings.CutPrefix(t, "v"); found {
			if maj, min, ok := parseVersionParam(rest); ok {
				return maj, min, true
			}
		}
	}
	return 0, 0, false
}

func applySnapshotFilter(snaps []restic.Snapshot, f *SnapshotFilter) []restic.Snapshot {
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
			maj, min, ok := parseVersionTag(sn.Tags)
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
		out = append(out, sn)
	}
	return out
}

func (s *Server) snapshotListResponse(volName string, opts *restic.ListSnapshotsOpts, filter *SnapshotFilter, offset, limit int) (map[string]any, error) {
	rm := s.volumeManager(volName)
	snaps, err := rm.ListSnapshotsWithOpts(opts)
	if err != nil {
		return nil, err
	}

	snaps = applySnapshotFilter(snaps, filter)

	rawLen := len(snaps)
	hasMore := false
	if limit > 0 {
		switch {
		case offset+limit <= rawLen:
			hasMore = rawLen > offset+limit
			snaps = snaps[offset : offset+limit]
		case offset < rawLen:
			snaps = snaps[offset:]
		default:
			snaps = nil
		}
	} else if offset > 0 && offset < rawLen {
		snaps = snaps[offset:]
	}

	restorePointID := ""
	if id, err := rm.FindRestorePointByName(volName); err == nil {
		restorePointID = id
	}

	result := make([]SnapshotWithVolume, 0, len(snaps))
	for _, snap := range snaps {
		fullHash := rm.GenerateHash(snap)
		snap.FallbackHash = fullHash[:len(snap.ShortID)]
		result = append(result, SnapshotWithVolume{Snapshot: snap, Volume: volName})
	}

	return map[string]any{
		"snapshots":      result,
		"restorePointID": restorePointID,
		"hasMore":        hasMore,
	}, nil
}

func parseSnapshotListOpts(r *http.Request) (*restic.ListSnapshotsOpts, *SnapshotFilter, int, int) {
	hosts := r.URL.Query()["host"]
	tags := r.URL.Query()["tag"]

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	limit := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	var filter *SnapshotFilter
	ensureFilter := func() {
		if filter == nil {
			filter = &SnapshotFilter{}
		}
	}

	if tf := r.URL.Query().Get("timeFrom"); tf != "" {
		if n, err := strconv.ParseInt(tf, 10, 64); err == nil {
			t := time.UnixMilli(n)
			ensureFilter()
			filter.TimeFrom = &t
		}
	}
	if tf := r.URL.Query().Get("timeTo"); tf != "" {
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
		if maj, min, ok := parseVersionParam(vf); ok {
			ensureFilter()
			filter.VersionFrom = &VersionRange{Major: maj, Minor: min}
		}
	}
	if vt := r.URL.Query().Get("versionTo"); vt != "" {
		if maj, min, ok := parseVersionParam(vt); ok {
			ensureFilter()
			filter.VersionTo = &VersionRange{Major: maj, Minor: min}
		}
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

func respondError(w http.ResponseWriter, err error, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
		logError("request_error", err)
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		logError("encode_error_response_failed", err)
	}
}

func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logError("encode_response_failed", err)
	}
}
