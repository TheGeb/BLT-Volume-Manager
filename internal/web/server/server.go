package server

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

type StatsCache struct {
	CachedAt     string `json:"cached_at"`
	TotalVolumes int    `json:"total_volumes"`
}

type Server struct {
	ResticBase    string
	Config        cfg.Config
	metadataStore *metadata.Store
	metadataMu    sync.Mutex
	statsMu       sync.RWMutex
	statsCacheVal *StatsCache
	statsCacheAt  time.Time
	wg            sync.WaitGroup
}

func (s *Server) Shutdown() {
	s.wg.Wait()
}

func New(cfg cfg.Config) *Server {
	return &Server{ResticBase: cfg.ResticBase, Config: cfg}
}

func (s *Server) SetMetadataStore(st *metadata.Store) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	s.metadataStore = st
}

func (s *Server) HasBackend() bool {
	return s.Config.MetadataBackend != "" || s.Config.S3Bucket != ""
}

func (s *Server) MetadataStore() (*metadata.Store, error) {
	return s.initMetadataStore()
}

func (s *Server) StoreForVolume() (*metadata.Store, error) { // FIXME: This naming is awkward
	return s.MetadataStore()
}

func (s *Server) StoreForResticData() (metadata.ObjectStore, error) { // FIXME: name is a bit too verbose, and awkwardly returns metadata store
	if s.Config.S3Bucket == "" {
		return nil, fmt.Errorf("S3 bucket not configured")
	}
	return s.initMetadataStore()
}

func (s *Server) initMetadataStore() (*metadata.Store, error) {
	s.metadataMu.Lock()
	defer s.metadataMu.Unlock()
	if s.metadataStore != nil {
		return s.metadataStore, nil
	}
	raw, err := cfg.OpenMetadataBackend(s.Config)
	if err != nil {
		return nil, err
	}
	s.metadataStore = metadata.New(raw)
	return s.metadataStore, nil
}

func (s *Server) BackupStoreType() string {
	if strings.HasPrefix(s.ResticBase, "s3:") {
		return "s3"
	}
	return "local"
}

func (s *Server) VolumeManager(volName string) *restic.Manager {
	return restic.NewManager(s.ResticBase + "/restic/" + volName)
}

func (s *Server) NextVersionTags(volName string, major bool) []string {
	ms, err := s.MetadataStore()
	if err != nil || ms == nil {
		return nil
	}
	tags, err := ms.NextVersionTags(volName, major)
	if err != nil {
		return nil
	}
	return tags
}

func (s *Server) VolumeNames() []string {
	ms, err := s.MetadataStore()
	if err != nil || ms == nil {
		return nil
	}
	if names, err := ms.ListRegisteredVolumes(); err == nil {
		return names
	}
	return nil
}

func (s *Server) RefreshStats() {
	volNames := s.VolumeNames()

	s.statsMu.Lock()
	s.statsCacheVal = &StatsCache{
		CachedAt:     time.Now().UTC().Format(time.RFC3339),
		TotalVolumes: len(volNames),
	}
	s.statsCacheAt = time.Now()
	s.statsMu.Unlock()
}

func (s *Server) WithStatsLock(f func()) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	f()
}

func (s *Server) WithStatsRLock(f func()) {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	f()
}

func (s *Server) StatsCache() *StatsCache {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.statsCacheVal
}

func (s *Server) AddWork()  { s.wg.Add(1) }
func (s *Server) DoneWork() { s.wg.Done() }

func ValidVolumeName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..")
}
