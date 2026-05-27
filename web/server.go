package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/example/blt-volume-manager/restic"
	"github.com/example/blt-volume-manager/store"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	dataDir      string
	resticBase   string
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	statsMu      sync.RWMutex
	statsCache   map[string]interface{}
	statsCacheAt time.Time
}

func NewServer(dataDir string, resticBase string, s3Bucket string, s3Endpoint string, s3Region string) *Server {
	return &Server{dataDir: dataDir, resticBase: resticBase, s3Bucket: s3Bucket, s3Endpoint: s3Endpoint, s3Region: s3Region}
}

func (s *Server) volumeManager(volName string) *restic.Manager {
	m := restic.NewManager(s.resticBase + "/restic/" + volName)
	if s.s3Bucket != "" {
		if rw, err := store.NewS3Store(store.S3StoreOpts{
			AwsBucketName: s.s3Bucket,
			S3Endpoint:    s.s3Endpoint,
			Region:        s.s3Region,
		}); err == nil {
			m.SetS3Store(rw)
		}
	}
	return m
}

func (s *Server) volumeNames() []string {
	if s.s3Bucket != "" {
		if rw, err := store.NewS3Store(store.S3StoreOpts{
			AwsBucketName:   s.s3Bucket,
			AwsVolumePrefix: store.VolumePrefix,
			S3Endpoint:      s.s3Endpoint,
			Region:          s.s3Region,
		}); err == nil {
			if names, err := rw.ListVolumeMarkers(); err == nil {
				return names
			}
		}
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
