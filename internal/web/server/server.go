package server

import (
	"errors"
	"sync"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

var ErrNoBackend = errors.New("metadata backend not available")

type StatsCache struct {
	CachedAt     string `json:"cached_at"`
	TotalVolumes int    `json:"total_volumes"`
}

type Service struct { // TODO: Rename to BLTService
	Config cfg.Config

	metadata     *metadata.Metadata
	metadataMu   sync.Mutex
	statsMu      sync.RWMutex
	statsCache   *StatsCache
	statsCacheAt time.Time
	wg           sync.WaitGroup
}

func (s *Service) Shutdown() {
	s.wg.Wait()
}

func New(cfg cfg.Config) *Service {
	return &Service{Config: cfg}
}

func (s *Service) SetMetadata(meta *metadata.Metadata) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	s.metadata = meta
}

func (s *Service) initBackend() error {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if s.metadata != nil {
		return nil
	}
	md, err := cfg.OpenMetadataBackend(s.Config)
	if err != nil {
		return err
	}
	s.metadata = md
	return nil
}

func (s *Service) VolumeStore() (*store.RegisteredVolumeStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.metadata.Volumes, nil
}

func (s *Service) RestorePointStore() (*store.RestorePointStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.metadata.RestorePoints, nil
}

func (s *Service) VersionStore() (*store.VersionStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.metadata.Versions, nil
}

func (s *Service) OwnerStore() (*store.OwnerStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.metadata.Owners, nil
}

func (s *Service) RegisterVolume(name string) error {
	vs, err := s.VolumeStore()
	if err != nil {
		return err
	}
	return vs.Register(name)
}

func (s *Service) SetRestorePoint(volume, snapshotID string) error {
	rps, err := s.RestorePointStore()
	if err != nil {
		return err
	}
	return rps.Set(volume, snapshotID)
}

func (s *Service) DeleteRestorePoint(volume string) error {
	rps, err := s.RestorePointStore()
	if err != nil {
		return err
	}
	return rps.Delete(volume)
}

func (s *Service) FindRestorePointByName(volName string) (string, error) {
	rps, err := s.RestorePointStore()
	if err != nil {
		return "", err
	}
	return rps.FindByName(volName)
}

func (s *Service) DeleteMetadata(volumeName string) error {
	return s.metadata.DeleteMetadata(volumeName)
}

func (s *Service) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(s.Config.ResticBase + "/restic/" + volName)
}

func (s *Service) NextVersionTags(volName string, major bool) []string {
	vs, err := s.VersionStore()
	if err != nil {
		return nil
	}
	tags, err := vs.NextTags(volName, major)
	if err != nil {
		return nil
	}
	return tags
}

func (s *Service) VolumeNames() []string {
	vs, err := s.VolumeStore()
	if err != nil {
		return nil
	}
	names, err := vs.List()
	if err != nil {
		return nil
	}
	return names
}

func (s *Service) RefreshStats() {
	volNames := s.VolumeNames()

	s.statsMu.Lock()
	s.statsCache = &StatsCache{
		CachedAt:     time.Now().UTC().Format(time.RFC3339),
		TotalVolumes: len(volNames),
	}
	s.statsCacheAt = time.Now()
	s.statsMu.Unlock()
}

func (s *Service) WithStatsLock(f func()) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	f()
}

func (s *Service) WithStatsRLock(f func()) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	f()
}

func (s *Service) StatsCache() *StatsCache {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.statsCache
}

func (s *Service) AddWork()  { s.wg.Add(1) }
func (s *Service) DoneWork() { s.wg.Done() }
