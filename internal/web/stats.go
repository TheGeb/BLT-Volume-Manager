package web

import (
	"net/http"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/constants"
)

func (s *Server) refreshStats() {
	volNames := s.volumeNames()

	s.statsMu.Lock()
	s.statsCache = &statsCache{
		CachedAt:     time.Now().UTC().Format(time.RFC3339),
		TotalVolumes: len(volNames),
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
	snapshotStats := map[string]any{
		"total": 0, "hot": 0, "cold": 0, "excluded": 0, "volumes": 0,
		"newest": "", "oldest": "",
	}
	repoStats := map[string]any{}

	rst, err := rm.Stats()
	if err != nil {
		logError("stats_failed", err)
		repoStats["error"] = err.Error()
	} else if rst != nil {
		repoStats = map[string]any{
			"total_size":              rst.TotalSize,
			"total_file_count":        rst.TotalFileCount,
			"total_blob_count":        rst.TotalBlobCount,
			"total_uncompressed_size": rst.TotalUncompressedSize,
			"unique_blob_count":       rst.UniqueBlobCount,
		}
	}

	snaps, err := rm.ListSnapshots()
	if err == nil && snaps != nil {
		hot, cold, excluded := 0, 0, 0
		var newest, oldest time.Time
		for i, snap := range snaps {
			for _, tag := range snap.Tags {
				switch tag {
				case constants.BackupTagHot:
					hot++
				case constants.BackupTagCold:
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
		var hotVols, coldVols, excludedVols, otherVols []string
		if hot > 0 {
			hotVols = []string{volName}
		}
		if cold > 0 {
			coldVols = []string{volName}
		}
		if excluded > 0 {
			excludedVols = []string{volName}
		}
		if o := len(snaps) - hot - cold - excluded; o > 0 {
			otherVols = []string{volName}
		}
		snapshotStats = map[string]any{
			"total":            len(snaps),
			"hot":              hot,
			"cold":             cold,
			"excluded":         excluded,
			"newest":           newestStr,
			"oldest":           oldestStr,
			"hot_volumes":      hotVols,
			"cold_volumes":     coldVols,
			"excluded_volumes": excludedVols,
			"other_volumes":    otherVols,
		}
	}

	resp := map[string]any{
		"snapshots": snapshotStats,
		"repo":      repoStats,
		"volume":    volName,
	}

	s.statsMu.RLock()
	if s.statsCache != nil {
		resp["cached_at"] = s.statsCache.CachedAt
		resp["total_volumes"] = s.statsCache.TotalVolumes
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
	respondJSON(w, map[string]any{"volumes": volumes})
}
