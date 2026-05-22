package web

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/example/blt-volume-manager/restic"
)

//go:embed static/*
var staticFiles embed.FS

type Driver interface {
	ResticManager(volName string) *restic.Manager
	VolumeNames() []string
}

type Server struct {
	dataDir      string
	resticBase   string
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	driver       Driver
	statsMu      sync.RWMutex
	statsCache   map[string]interface{}
	statsCacheAt time.Time
	pillsMu      sync.RWMutex
	pillsCache   []string
	pillsCacheAt time.Time
}

func NewServer(dataDir string, resticBase string, s3Bucket string, s3Endpoint string, s3Region string, drv Driver) *Server {
	return &Server{dataDir: dataDir, resticBase: resticBase, s3Bucket: s3Bucket, s3Endpoint: s3Endpoint, s3Region: s3Region, driver: drv}
}

func (s *Server) volumeManager(volName string) *restic.Manager {
	return s.driver.ResticManager(volName)
}

func (s *Server) volumeNames() []string {
	return s.driver.VolumeNames()
}

func (s *Server) Register(mux *http.ServeMux) {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/repo/init", s.handleRepoInit)
	inner.HandleFunc("/api/repo/status", s.handleRepoStatus)
	inner.HandleFunc("/api/snapshots", s.handleSnapshots)
	inner.HandleFunc("/api/snapshot/", s.handleSnapshotAction)
	inner.HandleFunc("/api/snapshot-view/", s.handleSnapshotView)
	inner.HandleFunc("/api/volume/", s.handleVolumeAction)
	inner.HandleFunc("/api/stats", s.handleStats)
	inner.HandleFunc("/api/stats/refresh", s.handleStatsRefresh)
	inner.HandleFunc("/api/pills", s.handlePills)
	inner.HandleFunc("/api/repo/check", s.handleCheck)
	inner.HandleFunc("/api/repo/repair", s.handleRepair)
	inner.HandleFunc("/api/test/create-volume", s.handleTestCreateVolume)

	uiFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	inner.Handle("/", http.FileServer(http.FS(uiFS)))

	mux.Handle("/", loggingMiddleware(inner))
}
