package repo

import (
	"net/http"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func HandleStats(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, "missing volume query parameter", http.StatusBadRequest)
		return
	}

	rm := s.VolumeManager(volName)
	snapshotStats := map[string]any{
		"total": 0, "hot": 0, "cold": 0, "excluded": 0, "volumes": 0,
		"newest": "", "oldest": "",
	}
	repoStats := map[string]any{}

	rst, err := rm.Stats()
	if err != nil {
		log.Error("stats_failed", err)
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
				case restic.BackupTagHot:
					hot++
				case restic.BackupTagCold:
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

	if c := s.StatsCache(); c != nil {
		resp["cached_at"] = c.CachedAt
		resp["total_volumes"] = c.TotalVolumes
	}

	server.RespondJSON(w, resp)
}

func HandleStatsRefresh(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.RefreshStats()
	cached := s.StatsCache()
	server.RespondJSON(w, cached)
}
