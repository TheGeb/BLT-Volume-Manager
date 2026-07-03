package repo

import (
	"net/http"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func buildRepoStats(rm *restic.Manager) map[string]any {
	stats := map[string]any{}
	rst, err := rm.Stats()
	if err != nil {
		log.Error("stats_failed", err)
		stats["error"] = err.Error()
	} else if rst != nil {
		stats = map[string]any{
			"total_size":              rst.TotalSize,
			"total_file_count":        rst.TotalFileCount,
			"total_blob_count":        rst.TotalBlobCount,
			"total_uncompressed_size": rst.TotalUncompressedSize,
			"unique_blob_count":       rst.UniqueBlobCount,
		}
	}
	return stats
}

func buildSnapshotStats(rm *restic.Manager, volName string) map[string]any {
	stats := map[string]any{
		"total": 0, "hot": 0, "cold": 0, "excluded": 0, "volumes": 0,
		"newest": "", "oldest": "",
	}
	snaps, err := rm.ListSnapshots()
	if err != nil || snaps == nil {
		return stats
	}
	hot, cold, excluded := 0, 0, 0
	var newest, oldest time.Time
	for i, snap := range snaps {
		for _, tag := range snap.Tags {
			switch tag {
			case restic.BackupTagHot:
				hot++
			case restic.BackupTagCold:
				cold++
			case "excluded": //FIXME: Is this tag still valid? If so, add constant, otherwise remove
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
	stats = map[string]any{
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
	return stats
}

func Stats(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	rm := s.VolumeManager(volName)

	resp := map[string]any{
		"snapshots": buildSnapshotStats(rm, volName), //FIXME: Is this even used on the UI anymore? Might be able to completely remove
		"repo":      buildRepoStats(rm),
		"volume":    volName,
	}

	if c := s.StatsCache(); c != nil {
		resp["cached_at"] = c.CachedAt
		resp["total_volumes"] = c.TotalVolumes
	}

	server.RespondJSON(w, resp)
}

func RefreshStats(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	s.RefreshStats()
	cached := s.StatsCache()
	server.RespondJSON(w, cached)
}
