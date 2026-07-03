package server

import (
	"errors"
	"sync"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

var ErrNoBackend = errors.New("metadata backend not available")

type StatsCache struct {
	CachedAt     string `json:"cached_at"`
	TotalVolumes int    `json:"total_volumes"`
}

type Service struct {
	ResticBase string
	Config     cfg.Config

	stores       *metadata.Stores
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
	return &Service{ResticBase: cfg.ResticBase, Config: cfg}
}

func (s *Service) SetStores(stores *metadata.Stores) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	s.stores = stores
}

func (s *Service) initBackend() error {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if s.stores != nil {
		return nil
	}
	stores, err := cfg.OpenMetadataBackend(s.Config)
	if err != nil {
		return err
	}
	s.stores = stores
	return nil
}

func (s *Service) VolumeStore() (*metadata.VolumeStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.stores.Volumes, nil
}

func (s *Service) RestorePointStore() (*metadata.RestorePointStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.stores.RestorePoints, nil
}

func (s *Service) VersionStore() (*metadata.VersionStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.stores.Versions, nil
}

func (s *Service) OwnerStore() (*metadata.OwnerStore, error) {
	if err := s.initBackend(); err != nil {
		return nil, err
	}
	return s.stores.Owners, nil
}

func (s *Service) RegisterVolume(name string) error {
	vs, err := s.VolumeStore()
	if err != nil {
		return err
	}
	return vs.Register(name)
}

func (s *Service) SetRestorePoint(volume, snapshotID string) error {
	rs, err := s.RestorePointStore()
	if err != nil {
		return err
	}
	return rs.Set(volume, snapshotID)
}

func (s *Service) DeleteRestorePoint(volume string) error {
	rs, err := s.RestorePointStore()
	if err != nil {
		return err
	}
	return rs.Delete(volume)
}

func (s *Service) FindRestorePointByName(volName string) (string, error) {
	rs, err := s.RestorePointStore()
	if err != nil {
		return "", err
	}
	return rs.FindByName(volName)
}

func (s *Service) DeleteVolumeData(volumeName string) error {
	vs, err := s.VolumeStore()
	if err != nil {
		return err
	}
	return vs.DeleteVolumeData(volumeName)
}

func (s *Service) ResticManager(volName string) *restic.Manager {
	return restic.NewManager(s.ResticBase + "/restic/" + volName)
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
