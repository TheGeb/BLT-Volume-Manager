package web

import (
	"net/http"
	"time"
)

func (s *Server) refreshStats() {
	volNames := s.volumeNames()

	s.statsMu.Lock()
	s.statsCache = map[string]interface{}{
		"cached_at":     time.Now().UTC().Format(time.RFC3339),
		"total_volumes": len(volNames),
	}
	s.statsCacheAt = time.Now()
	s.statsMu.Unlock()

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

func (s *Server) handleVolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volumes := s.volumeNames()
	if volumes == nil {
		volumes = []string{}
	}
	respondJSON(w, map[string]interface{}{"volumes": volumes})
}
