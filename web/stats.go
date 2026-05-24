package web

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/store"
)

func (s *Server) refreshStats() {
	lockStats := map[string]interface{}{
		"total_volumes": 0, "active": 0, "expired": 0, "unlocked": 0,
	}
	pillSet := map[string]bool{}

	for _, volName := range s.volumeNames() {
		rm := s.volumeManager(volName)
		pillSet[volName] = true
		snaps, err := rm.ListSnapshots()
		if err != nil {
			continue
		}
		for _, snap := range snaps {
			for _, tag := range snap.Tags {
				switch tag {
				case "hot", "cold", "excluded":
				}
			}
		}
	}

	if s.s3Bucket != "" {
		rw, err := store.NewS3Store(store.S3StoreOpts{
			AwsBucketName: s.s3Bucket,
			S3Endpoint:    s.s3Endpoint,
			Region:        s.s3Region,
		})
		if err == nil {
			prefixes, err := rw.ListCommonPrefixes("volume-locks/", "/")
			if err == nil {
				volStatus := map[string]bool{}
				lockTTL := 24 * time.Hour
				for _, folder := range prefixes {
					name := strings.TrimSuffix(strings.TrimPrefix(folder, "volume-locks/"), "/")
					if name == "" {
						continue
					}
					pillSet[name] = true
					objects, err := rw.ListObjects(folder)
					if err != nil {
						continue
					}
					locked := false
					for _, obj := range objects {
						if obj.Key == nil {
							continue
						}
						if obj.LastModified != nil && time.Since(*obj.LastModified) > lockTTL {
							continue
						}
						raw, err := rw.ReadObject(*obj.Key)
						if err != nil || raw == nil {
							continue
						}
						var owner store.LockOwner
						if err := json.Unmarshal(raw, &owner); err != nil {
							continue
						}
						if owner.GetRemainingTimeinSeconds() > 0 {
							locked = true
							break
						}
					}
					volStatus[name] = locked
				}

				active, expired := 0, 0
				var activeVols, expiredVols []string
				for name, locked := range volStatus {
					if locked {
						active++
						activeVols = append(activeVols, name)
					} else {
						expired++
						expiredVols = append(expiredVols, name)
					}
				}
				sort.Strings(activeVols)
				sort.Strings(expiredVols)
				lockStats = map[string]interface{}{
					"total_volumes":   len(volStatus),
					"active":          active,
					"expired":         expired,
					"unlocked":        len(volStatus) - active - expired,
					"active_volumes":  activeVols,
					"expired_volumes": expiredVols,
				}
			}
		}
	}

	s.statsMu.Lock()
	s.statsCache = map[string]interface{}{
		"locks":         lockStats,
		"cached_at":     time.Now().UTC().Format(time.RFC3339),
		"total_volumes": len(pillSet),
	}
	s.statsCacheAt = time.Now()
	s.statsMu.Unlock()

	pills := make([]string, 0, len(pillSet))
	for v := range pillSet {
		pills = append(pills, v)
	}
	sort.Strings(pills)

	s.pillsMu.Lock()
	s.pillsCache = pills
	s.pillsCacheAt = time.Now()
	s.pillsMu.Unlock()

	logInfo("stats_refreshed")
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}

	rm := s.volumeManager(volName)
	snapshotStats := map[string]interface{}{
		"total": 0, "hot": 0, "cold": 0, "excluded": 0, "volumes": 0,
		"newest": "", "oldest": "",
	}
	repoStats := map[string]interface{}{}

	rst, err := rm.Stats()
	if err != nil {
		logError("stats_failed", err)
		repoStats["error"] = err.Error()
	} else if rst != nil {
		repoStats = map[string]interface{}{
			"total_size":              rst.TotalSize,
			"total_file_count":        rst.TotalFileCount,
			"total_blob_count":        rst.TotalBlobCount,
			"total_uncompressed_size": rst.TotalUncompressedSize,
			"compressed_size":         rst.TotalSize,
			"unique_blob_count":       rst.TotalBlobCount,
		}
	}

	snaps, err := rm.ListSnapshots()
	if err == nil && snaps != nil {
		hot, cold, excluded := 0, 0, 0
		var newest, oldest time.Time
		for i, snap := range snaps {
			for _, tag := range snap.Tags {
				switch tag {
				case "hot":
					hot++
				case "cold":
					cold++
				case "excluded":
					excluded++
				}
			}
			if i == 0 || snap.Time.After(newest) {
				newest = snap.Time
			}
			if i == 0 || snap.Time.Before(oldest) {
				oldest = snap.Time
			}
		}
		newestStr := ""
		oldestStr := ""
		if !newest.IsZero() {
			newestStr = newest.Format(time.RFC3339)
		}
		if !oldest.IsZero() {
			oldestStr = oldest.Format(time.RFC3339)
		}
		snapshotStats = map[string]interface{}{
			"total":    len(snaps),
			"hot":      hot,
			"cold":     cold,
			"excluded": excluded,
			"volumes":  1,
			"newest":   newestStr,
			"oldest":   oldestStr,
			"hot_volumes":      func() []string { if hot > 0 { return []string{volName} }; return nil }(),
			"cold_volumes":     func() []string { if cold > 0 { return []string{volName} }; return nil }(),
			"excluded_volumes": func() []string { if excluded > 0 { return []string{volName} }; return nil }(),
			"other_volumes":    func() []string { o := len(snaps) - hot - cold - excluded; if o > 0 { return []string{volName} }; return nil }(),
		}
	}

	resp := map[string]interface{}{
		"snapshots": snapshotStats,
		"repo":      repoStats,
		"volume":    volName,
	}

	s.statsMu.RLock()
	if s.statsCache != nil {
		resp["locks"] = s.statsCache["locks"]
		resp["cached_at"] = s.statsCache["cached_at"]
		resp["total_volumes"] = s.statsCache["total_volumes"]
	}
	s.statsMu.RUnlock()

	respondJSON(w, resp)
}

func (s *Server) handleStatsRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.refreshStats()
	s.statsMu.RLock()
	cached := s.statsCache
	s.statsMu.RUnlock()
	respondJSON(w, cached)
}

func (s *Server) handlePills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.pillsMu.RLock()
	pills := s.pillsCache
	at := s.pillsCacheAt
	s.pillsMu.RUnlock()
	if len(pills) == 0 {
		s.refreshStats()
		s.pillsMu.RLock()
		pills = s.pillsCache
		at = s.pillsCacheAt
		s.pillsMu.RUnlock()
	}
	resp := map[string]interface{}{"volumes": pills}
	if !at.IsZero() {
		resp["cached_at"] = at.UTC().Format(time.RFC3339)
	}
	respondJSON(w, resp)
}
