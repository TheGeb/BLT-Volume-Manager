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

type Server struct {
	restic       *restic.Manager
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	statsMu      sync.RWMutex
	statsCache   map[string]interface{}
	statsCacheAt time.Time
	pillsMu      sync.RWMutex
	pillsCache   []string
	pillsCacheAt time.Time
}

func NewServer(r *restic.Manager, s3Bucket string, s3Endpoint string, s3Region string) *Server {
	return &Server{restic: r, s3Bucket: s3Bucket, s3Endpoint: s3Endpoint, s3Region: s3Region}
}

func (s *Server) Register(mux *http.ServeMux) {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/repo/init", s.handleRepoInit)
	inner.HandleFunc("/api/repo/status", s.handleRepoStatus)
	inner.HandleFunc("/api/snapshots", s.handleSnapshots)
	inner.HandleFunc("/api/snapshot/", s.handleSnapshotAction)
	inner.HandleFunc("/api/volume/", s.handleVolumeAction)
	inner.HandleFunc("/api/stats", s.handleStats)
	inner.HandleFunc("/api/stats/refresh", s.handleStatsRefresh)
	inner.HandleFunc("/api/pills", s.handlePills)
	inner.HandleFunc("/api/repo/check", s.handleCheck)
	inner.HandleFunc("/api/repo/repair", s.handleRepair)

	uiFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	inner.Handle("/", http.FileServer(http.FS(uiFS)))

	mux.Handle("/", loggingMiddleware(inner))
}
