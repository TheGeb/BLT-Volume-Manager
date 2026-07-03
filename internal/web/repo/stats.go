package repo

import (
	"net/http"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type RepoStats struct {
	TotalSize             int64  `json:"total_size,omitempty"`
	TotalFileCount        int64  `json:"total_file_count,omitempty"`
	TotalBlobCount        int64  `json:"total_blob_count,omitempty"`
	TotalUncompressedSize int64  `json:"total_uncompressed_size,omitempty"`
	UniqueBlobCount       int64  `json:"unique_blob_count,omitempty"`
	Error                 string `json:"error,omitempty"`
}

type SnapshotStats struct {
	Total        int      `json:"total"`
	Hot          int      `json:"hot"`
	Cold         int      `json:"cold"`
	Newest       string   `json:"newest"`
	Oldest       string   `json:"oldest"`
	HotVolumes   []string `json:"hot_volumes,omitempty"`
	ColdVolumes  []string `json:"cold_volumes,omitempty"`
	OtherVolumes []string `json:"other_volumes,omitempty"`
}

type StatsResponse struct {
	Volume       string        `json:"volume"`
	Snapshots    SnapshotStats `json:"snapshots"`
	Repo         *RepoStats    `json:"repo"`
	CachedAt     string        `json:"cached_at,omitempty"`
	TotalVolumes int           `json:"total_volumes,omitempty"`
}

func buildRepoStats(rm *restic.Manager) *RepoStats {
	rst, err := rm.Stats()
	if err != nil {
		log.Error("stats_failed", err)
		return &RepoStats{Error: err.Error()}
	}
	if rst == nil {
		return nil
	}
	return &RepoStats{
		TotalSize:             rst.TotalSize,
		TotalFileCount:        rst.TotalFileCount,
		TotalBlobCount:        rst.TotalBlobCount,
		TotalUncompressedSize: rst.TotalUncompressedSize,
		UniqueBlobCount:       rst.UniqueBlobCount,
	}
}

func buildSnapshotStats(rm *restic.Manager, volName string) SnapshotStats {
	snaps, err := rm.ListSnapshots()
	if err != nil || snaps == nil {
		return SnapshotStats{}
	}
	hot, cold := 0, 0
	var newest, oldest time.Time
	for i, snap := range snaps {
		for _, tag := range snap.Tags {
			switch tag {
			case restic.BackupTagHot:
				hot++
			case restic.BackupTagCold:
				cold++
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
	var hotVols, coldVols, otherVols []string
	if hot > 0 {
		hotVols = []string{volName}
	}
	if cold > 0 {
		coldVols = []string{volName}
	}
	if o := len(snaps) - hot - cold; o > 0 {
		otherVols = []string{volName}
	}
	return SnapshotStats{
		Total:        len(snaps),
		Hot:          hot,
		Cold:         cold,
		Newest:       newestStr,
		Oldest:       oldestStr,
		HotVolumes:   hotVols,
		ColdVolumes:  coldVols,
		OtherVolumes: otherVols,
	}
}

func Stats(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	rm := s.ResticManager(volName)

	resp := StatsResponse{
		Volume:    volName,
		Snapshots: buildSnapshotStats(rm, volName),
		Repo:      buildRepoStats(rm),
	}

	if c := s.StatsCache(); c != nil {
		resp.CachedAt = c.CachedAt
		resp.TotalVolumes = c.TotalVolumes
	}

	server.RespondJSON(w, resp)
}

func RefreshStats(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	s.RefreshStats()
	cached := s.StatsCache()
	server.RespondJSON(w, cached)
}
