package web

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/example/blt-volume-manager/internal/appconfig"
	"github.com/example/blt-volume-manager/internal/restic"
	"github.com/example/blt-volume-manager/internal/store"
)

//go:embed static/*
var staticFiles embed.FS

var staticFilesOK sync.Once

func verifyStaticFiles() {
	staticFilesOK.Do(func() {
		if _, err := staticFiles.ReadDir("static"); err != nil {
			panic("web assets not found. Run: make ui")
		}
	})
}

type statsCache struct {
	CachedAt     string `json:"cached_at"`
	TotalVolumes int    `json:"total_volumes"`
}

type Server struct {
	dataDir      string
	resticBase   string
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	s3Store      store.S3Store
	s3StoreOnce  sync.Once
	s3StoreErr   error
	s3StoreMu    sync.RWMutex
	s3StoreCache map[string]store.S3Store
	statsMu      sync.RWMutex
	statsCache   *statsCache
	statsCacheAt time.Time
}

func NewServer(cfg appconfig.Config) *Server {
	return &Server{dataDir: cfg.DataDir, resticBase: cfg.ResticBase, s3Bucket: cfg.S3Bucket, s3Endpoint: cfg.S3Endpoint, s3Region: cfg.S3Region}
}

func (s *Server) getOrCreateS3Store() (store.S3Store, error) {
	if s.s3Bucket == "" {
		return nil, nil //nolint:nilnil // S3 not configured is not an error
	}
	s.s3StoreOnce.Do(func() {
		s.s3Store, s.s3StoreErr = store.NewS3Store(store.S3StoreConfig{
			S3Bucket:   s.s3Bucket,
			S3Endpoint: s.s3Endpoint,
			Region:     s.s3Region,
			Logger:     s3LogFn(),
		})
	})
	return s.s3Store, s.s3StoreErr
}

func (s *Server) getOrCreateS3StoreWithPrefix(prefix string) (store.S3Store, error) {
	if s.s3Bucket == "" {
		return nil, nil //nolint:nilnil // S3 not configured is not an error
	}

	s.s3StoreMu.RLock()
	if s.s3StoreCache != nil {
		if cached, ok := s.s3StoreCache[prefix]; ok {
			s.s3StoreMu.RUnlock()
			return cached, nil
		}
	}
	s.s3StoreMu.RUnlock()

	s.s3StoreMu.Lock()
	defer s.s3StoreMu.Unlock()

	// Double-check after acquiring write lock
	if s.s3StoreCache != nil {
		if cached, ok := s.s3StoreCache[prefix]; ok {
			return cached, nil
		}
	}

	s3Store, err := store.NewS3Store(store.S3StoreConfig{
		S3Bucket:       s.s3Bucket,
		S3VolumePrefix: prefix,
		S3Endpoint:     s.s3Endpoint,
		Region:         s.s3Region,
		Logger:         s3LogFn(),
	})
	if err != nil {
		return nil, err
	}

	if s.s3StoreCache == nil {
		s.s3StoreCache = make(map[string]store.S3Store)
	}
	s.s3StoreCache[prefix] = s3Store

	return s3Store, nil
}

func (s *Server) volumeManager(volName string) *restic.Manager {
	m := restic.NewManager(s.resticBase + "/restic/" + volName)
	if s3, err := s.getOrCreateS3Store(); err == nil && s3 != nil {
		m.SetS3Store(s3)
	}
	return m
}

func (s *Server) volumeNames() []string {
	s3, err := s.getOrCreateS3StoreWithPrefix(store.VolumePrefix)
	if err != nil || s3 == nil {
		return nil
	}
	if names, err := s3.ListVolumeMarkers(); err == nil {
		return names
	}
	return nil
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
	inner.HandleFunc("/api/volumes", s.handleVolumes)
	inner.HandleFunc("/api/repo/check", s.handleCheck)
	inner.HandleFunc("/api/repo/repair", s.handleRepair)
	if os.Getenv("BLT_ENABLE_TEST_ENDPOINTS") != "" {
		inner.HandleFunc("/api/test/create-volume", s.handleTestCreateVolume)
	}

	verifyStaticFiles()
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

	// SPA at /ui/ -- serve files directly, fallback to index.html for client-side routes
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
		_, _ = w.Write(data)
	})
	inner.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})

	mux.Handle("/", loggingMiddleware(inner))
}
