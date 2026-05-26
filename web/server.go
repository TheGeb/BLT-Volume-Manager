package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
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
	rootFileServer := http.FileServer(http.FS(uiFS))
	uiHandler := http.StripPrefix("/ui", rootFileServer)

	// Serve root-level static assets (e.g. /assets/*, /base.css, /index.html)
	inner.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ui/", http.StatusFound)
			return
		}
		rootFileServer.ServeHTTP(w, r)
	})

	// SPA at /ui/ — serve files directly, fallback to index.html for client-side routes
	inner.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/ui")
		if path == "" || path == "/" {
			uiHandler.ServeHTTP(w, r)
			return
		}
		if _, err := uiFS.Open(strings.TrimPrefix(path, "/")); err == nil {
			uiHandler.ServeHTTP(w, r)
			return
		}
		data, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	})
	inner.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})

	mux.Handle("/", loggingMiddleware(inner))
}
