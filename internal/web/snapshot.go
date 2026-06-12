package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type SnapshotWithVolume struct {
	restic.Snapshot
	Volume string `json:"volume"`
}

func findSnapshotByVersion(rm *restic.Manager, version string) (string, error) {
	tag := version
	if !strings.HasPrefix(version, "v") {
		tag = "v" + version
	}
	snapshots, err := rm.ListSnapshotsWithOpts(&restic.ListSnapshotsOpts{Tags: []string{tag}})
	if err != nil {
		return "", err
	}
	if len(snapshots) == 0 {
		return "", &snapshotNotFoundError{version: version}
	}
	newest := snapshots[0]
	for _, s := range snapshots[1:] {
		if s.Time.After(newest.Time) {
			newest = s
		}
	}
	return newest.ID, nil
}

type snapshotNotFoundError struct {
	version string
}

func (e *snapshotNotFoundError) Error() string {
	return "snapshot not found for version " + e.version
}

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

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := requireVolumeParam(w, r)
	if !ok {
		return
	}

	opts, filter, offset, limit := parseSnapshotListOpts(r)
	resp, err := s.snapshotListResponse(volName, opts, filter, offset, limit)
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, resp)
}

func (s *Server) handleSnapshotHosts(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := requireVolumeParam(w, r)
	if !ok {
		return
	}

	rm := s.volumeManager(volName)
	latest := 1
	if l := r.URL.Query().Get("latest"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			latest = n
		}
	}

	hosts, err := rm.ListSnapshotsGroupedByHost(latest)
	if err != nil {
		respondError(w, err, http.StatusInternalServerError)
		return
	}
	respondJSON(w, hosts)
}

func (s *Server) handleSnapshotAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/snapshot/")
	if trimmed == "sizes" {
		s.handleSnapshotSizes(w, r)
		return
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || (parts[1] != "tag" && parts[1] != "restore" && parts[1] != "delete") {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	volName, ok := requireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.volumeManager(volName)

	if parts[1] == "delete" {
		if !requireMethod(w, r, http.MethodDelete) {
			return
		}
		if err := rm.ForgetSnapshot(snapshotID); err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, map[string]string{"status": "snapshot deleted"})
		return
	}

	if parts[1] == "restore" {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		target := r.URL.Query().Get("path")
		if target == "" {
			http.Error(w, "missing path query parameter", http.StatusBadRequest)
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := rm.RestoreSnapshot(snapshotID, target); err != nil {
				logInfo("restore_failed: " + err.Error())
			} else {
				logInfo("restore_ok")
			}
		}()
		respondJSON(w, map[string]string{"status": "restore started – see server logs for results"})
		return
	}

	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if tag == "restore-point" {
			if err := rm.SetRestorePoint(snapshotID, volName); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.TagSnapshot(snapshotID, tag); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		resp, err := s.snapshotListResponse(volName, nil, nil, 0, 0)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		resp["status"] = "tag added"
		respondJSON(w, resp)
	case http.MethodDelete:
		if tag == "restore-point" {
			if err := rm.DeleteRestorePoint(volName); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		} else {
			if err := rm.UntagSnapshot(snapshotID, tag); err != nil {
				respondError(w, err, http.StatusInternalServerError)
				return
			}
		}
		resp, err := s.snapshotListResponse(volName, nil, nil, 0, 0)
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		resp["status"] = "tag removed"
		respondJSON(w, resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSnapshotView(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot-view/") {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/snapshot-view/")
	parts := strings.Split(rest, "/")

	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}
	rm := s.volumeManager(volName)

	rawID := parts[0]
	version := r.URL.Query().Get("tag")
	fallbackHash := r.URL.Query().Get("fallbackHash")
	action := parts[1]

	resolve := func(id, ver, fallback string) string {
		if ver != "" {
			if resolved, err := findSnapshotByVersion(rm, ver); err == nil {
				return resolved
			}
		}
		return id
	}

	rawID = resolve(rawID, version, fallbackHash)

	switch action {
	case "ls":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		nodes, err := rm.ListSnapshotFiles(rawID, path)
		if err != nil && fallbackHash != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
				nodes, err = rm.ListSnapshotFiles(snap.ID, path)
			}
		}
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		respondJSON(w, nodes)

	case "dump":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "missing path", http.StatusBadRequest)
			return
		}
		data, err := rm.DumpFile(rawID, path)
		if err != nil && fallbackHash != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
				data, err = rm.DumpFile(snap.ID, path)
			}
		}
		if err != nil {
			respondError(w, err, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		//nolint:gosec // Intentional dump of restic file contents as text/plain
		_, _ = w.Write(data)

	case "diff":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if len(parts) < 3 || parts[2] == "" {
			http.Error(w, "missing second snapshot id", http.StatusBadRequest)
			return
		}

		secondID := parts[2]
		diffVersion := r.URL.Query().Get("diffTag")
		diffFallback := r.URL.Query().Get("diffFallbackHash")

		secondID = resolve(secondID, diffVersion, diffFallback)

		result, err := rm.DiffSnapshots(rawID, secondID)
		if err != nil {
			resolvedFirst, resolvedSecond := rawID, secondID
			if fallbackHash != "" {
				if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
					resolvedFirst = snap.ID
				}
			}
			if diffFallback != "" {
				if snap, lookupErr := rm.FindSnapshotByHash(diffFallback); lookupErr == nil {
					resolvedSecond = snap.ID
				}
			}
			result, err = rm.DiffSnapshots(resolvedFirst, resolvedSecond)
		}
		if err != nil {
			respondError(w, err, http.StatusNotFound)
			return
		}
		respondJSON(w, result)

	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSnapshotSizes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Volume string   `json:"volume"`
		IDs    []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Volume == "" || len(req.IDs) == 0 {
		http.Error(w, "missing volume or ids", http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(req.Volume)
	result := map[string]int64{}
	for _, id := range req.IDs {
		stats, err := rm.SnapshotStats(id)
		if err != nil {
			continue
		}
		result[id] = stats.TotalSize
	}
	respondJSON(w, result)
}

type batchDeleteError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

type batchDeleteResponse struct {
	Deleted int                `json:"deleted"`
	Failed  int                `json:"failed"`
	Errors  []batchDeleteError `json:"errors"`
}

func (s *Server) handleSnapshotBatchDelete(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Volume string   `json:"volume"`
		IDs    []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Volume == "" || len(req.IDs) == 0 {
		http.Error(w, "missing volume or ids", http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(req.Volume)
	resp := batchDeleteResponse{Errors: []batchDeleteError{}}
	for _, id := range req.IDs {
		if err := rm.ForgetSnapshot(id); err != nil {
			resp.Failed++
			resp.Errors = append(resp.Errors, batchDeleteError{ID: id, Error: err.Error()})
		} else {
			resp.Deleted++
		}
	}
	respondJSON(w, resp)
}
