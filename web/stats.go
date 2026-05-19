package web

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/example/blt-volume-manager/store"
)

func (s *Server) statsLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.refreshStats()
	}
}

func (s *Server) refreshStats() {
	snapshotStats := map[string]interface{}{
		"total": 0, "hot": 0, "cold": 0, "excluded": 0, "volumes": 0,
		"newest": "", "oldest": "",
	}
	repoStats := map[string]interface{}{}
	lockStats := map[string]interface{}{
		"total_volumes": 0, "active": 0, "expired": 0, "unlocked": 0,
	}
	pillSet := map[string]bool{}

	rst, err := s.restic.Stats()
	if err == nil && rst != nil {
		repoStats = map[string]interface{}{
			"total_size":              rst.TotalSize,
			"total_file_count":        rst.TotalFileCount,
			"total_blob_count":        rst.TotalBlobCount,
			"total_uncompressed_size": rst.TotalUncompressedSize,
			"compressed_size":         rst.CompressedSize,
			"unique_blob_count":       rst.UniqueBlobCount,
			"unique_blob_size":        rst.UniqueBlobSize,
		}
	}

	snapshots, err := s.restic.ListSnapshots()
	if err == nil && snapshots != nil {
		hot, cold, excluded := 0, 0, 0
		volSet := map[string]bool{}
		hotVols := map[string]bool{}
		coldVols := map[string]bool{}
		excludedVols := map[string]bool{}
		var newest, oldest time.Time
		for i, snap := range snapshots {
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
			for _, path := range snap.Paths {
				if v := volumeNameFromPath(path); v != "" {
					volSet[v] = true
					pillSet[v] = true
					for _, tag := range snap.Tags {
						switch tag {
						case "hot":
							hotVols[v] = true
						case "cold":
							coldVols[v] = true
						case "excluded":
							excludedVols[v] = true
						}
					}
				}
			}
			if i == 0 || snap.Time.After(newest) {
				newest = snap.Time
			}
			if i == 0 || snap.Time.Before(oldest) {
				oldest = snap.Time
			}
		}

		volList := func(m map[string]bool) []string {
			var out []string
			for v := range m {
				out = append(out, v)
			}
			sort.Strings(out)
			return out
		}

		otherVols := map[string]bool{}
		for v := range volSet {
			if !hotVols[v] && !coldVols[v] && !excludedVols[v] {
				otherVols[v] = true
			}
		}

		snapshotStats = map[string]interface{}{
			"total":            len(snapshots),
			"hot":              hot,
			"cold":             cold,
			"excluded":         excluded,
			"volumes":          len(volSet),
			"newest":           newest.Format(time.RFC3339),
			"oldest":           oldest.Format(time.RFC3339),
			"hot_volumes":      volList(hotVols),
			"cold_volumes":     volList(coldVols),
			"excluded_volumes": volList(excludedVols),
			"other_volumes":    volList(otherVols),
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
		"snapshots":     snapshotStats,
		"repo":          repoStats,
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

	log.Println("[stats] refreshed")
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.statsMu.RLock()
	cached := s.statsCache
	s.statsMu.RUnlock()
	respondJSON(w, cached)
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
	s.pillsMu.RUnlock()
	respondJSON(w, map[string]interface{}{"volumes": pills})
}

func (s *Server) handleVolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var vols []string
	if s.s3Bucket != "" {
		rw, err := store.NewS3Store(store.S3StoreOpts{
			AwsBucketName: s.s3Bucket,
			S3Endpoint:    s.s3Endpoint,
			Region:        s.s3Region,
		})
		if err == nil {
			prefixes, err := rw.ListCommonPrefixes("volume-locks/", "/")
			if err == nil {
				for _, p := range prefixes {
					name := strings.TrimSuffix(strings.TrimPrefix(p, "volume-locks/"), "/")
					if name != "" {
						vols = append(vols, name)
					}
				}
			}
		}
	}
	sort.Strings(vols)
	respondJSON(w, vols)
}
