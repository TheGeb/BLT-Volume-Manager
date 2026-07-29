package repo

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

var ErrNilStats = errors.New("stats: nil result")

type RepoStats struct {
	TotalSize             int64 `json:"total_size,omitempty"`
	TotalFileCount        int64 `json:"total_file_count,omitempty"`
	TotalBlobCount        int64 `json:"total_blob_count,omitempty"`
	TotalUncompressedSize int64 `json:"total_uncompressed_size,omitempty"`
	UniqueBlobCount       int64 `json:"unique_blob_count,omitempty"`
}

type StatsResponse struct {
	Volume       string     `json:"volume"`
	Repo         *RepoStats `json:"repo"`
	CachedAt     string     `json:"cached_at,omitempty"`
	TotalVolumes int        `json:"total_volumes,omitempty"`
}

func buildRepoStats(ctx context.Context, rm *restic.Manager) (*RepoStats, error) {
	rst, err := rm.Stats(ctx)
	if err != nil {
		log.Error("stats_failed", err)
		return nil, fmt.Errorf("restic stats: %w", err)
	}
	if rst == nil {
		return nil, ErrNilStats
	}
	return &RepoStats{
		TotalSize:             rst.TotalSize,
		TotalFileCount:        rst.TotalFileCount,
		TotalBlobCount:        rst.TotalBlobCount,
		TotalUncompressedSize: rst.TotalUncompressedSize,
		UniqueBlobCount:       rst.UniqueBlobCount,
	}, nil
}

func Stats(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodGet) {
		return
	}
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	rm := s.ResticManager(volName)

	repoStats, err := buildRepoStats(ctx, rm)
	if err != nil && !errors.Is(err, ErrNilStats) {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}

	resp := StatsResponse{
		Volume: volName,
		Repo:   repoStats,
	}

	if c := s.StatsCache(); c != nil {
		resp.CachedAt = c.CachedAt
		resp.TotalVolumes = c.TotalVolumes
	}

	server.RespondJSON(w, resp)
}

func RefreshStats(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if !server.RequireMethod(w, r, http.MethodPost) {
		return
	}
	s.RefreshStats(r.Context())
	cached := s.StatsCache()
	server.RespondJSON(w, cached)
}
