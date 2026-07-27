package server

import (
	"context"
	"sync"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type StatsCache struct {
	CachedAt     string `json:"cached_at"`
	TotalVolumes int    `json:"total_volumes"`
}

// BLTService is the central service object for the web API, holding config, metadata stores, and stats cache.
type BLTService struct {
	Config cfg.Config

	owners   *store.OwnerStore
	volumes  *store.RegisteredVolumeStore
	versions *store.VersionStore
	restores *store.RestorePointStore

	statsMu      sync.RWMutex
	statsCache   *StatsCache
	statsCacheAt time.Time
	wg           sync.WaitGroup
}

func (s *BLTService) Shutdown() {
	s.wg.Wait()
}

func New(cfg cfg.Config, b store.Backend) *BLTService {
	return &BLTService{
		Config:   cfg,
		owners:   store.NewOwnerStore(b),
		volumes:  store.NewRegisteredVolumeStore(b),
		versions: store.NewVersionStore(b),
		restores: store.NewRestorePointStore(b),
	}
}

func (s *BLTService) SetStores(owners *store.OwnerStore, volumes *store.RegisteredVolumeStore, versions *store.VersionStore, restores *store.RestorePointStore) {
	s.owners = owners
	s.volumes = volumes
	s.versions = versions
	s.restores = restores
}

func (s *BLTService) OwnerStore() *store.OwnerStore { return s.owners }

func (s *BLTService) RestoreStore() *store.RestorePointStore { return s.restores }

func (s *BLTService) VolumeStore() *store.RegisteredVolumeStore { return s.volumes }

func (s *BLTService) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(s.Config.ResticBase + "/restic/" + volName)
}

func (s *BLTService) NextVersionTags(ctx context.Context, volName string, major bool) []string {
	tags, err := s.versions.NextTags(ctx, volName, major)
	if err != nil {
		return nil
	}
	return tags
}

func (s *BLTService) VolumeNames(ctx context.Context) []string {
	names, err := s.volumes.List(ctx)
	if err != nil {
		return nil
	}
	return names
}

func (s *BLTService) DeleteVolumeData(ctx context.Context, volumeName string) error {
	if err := s.volumes.Delete(ctx, volumeName); err != nil {
		return err
	}
	if err := s.owners.DeleteForVolume(ctx, volumeName); err != nil {
		return err
	}
	if err := s.restores.Delete(ctx, volumeName); err != nil {
		return err
	}
	return s.ResticManager(volumeName).PurgeSnapshots(ctx)
}

func (s *BLTService) RegisterVolume(ctx context.Context, volumeName string) error {
	return s.volumes.Register(ctx, volumeName)
}

func (s *BLTService) RefreshStats(ctx context.Context) {
	volNames := s.VolumeNames(ctx)

	s.statsMu.Lock()
	s.statsCache = &StatsCache{
		CachedAt:     time.Now().UTC().Format(time.RFC3339),
		TotalVolumes: len(volNames),
	}
	s.statsCacheAt = time.Now()
	s.statsMu.Unlock()
}

func (s *BLTService) WithStatsLock(f func()) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	f()
}

func (s *BLTService) WithStatsRLock(f func()) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	f()
}

func (s *BLTService) StatsCache() *StatsCache {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.statsCache
}

func (s *BLTService) AddWork()  { s.wg.Add(1) }
func (s *BLTService) DoneWork() { s.wg.Done() }
