package web

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/owner"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/repo"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/snapshot"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/volume"
)

func Register(s *server.BLTService, mux *http.ServeMux) error {
	inner := http.NewServeMux()

	registerHealth(s, inner)
	registerVersionRoute(s, inner)
	registerRepoRoutes(s, inner)
	registerSnapshotRoutes(s, inner)
	registerVolumeRoutes(s, inner)
	registerStatsRoutes(s, inner)
	registerDevRoutes(s, inner)

	uiFS, err := initUI()
	if err != nil {
		return err
	}
	registerUIRoutes(inner, uiFS)

	mux.Handle("/", server.NoSniff(server.Gzip(server.Logging(inner))))
	return nil
}

func registerHealth(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		// TODO: More robust health check - actually serving files (and can access backends? Or is that out of scope)
		if s.Config.ResticBase == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func registerRepoRoutes(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/repo/init", func(w http.ResponseWriter, r *http.Request) {
		repo.InitRepo(s, w, r)
	})
	inner.HandleFunc("/api/repo/status", func(w http.ResponseWriter, r *http.Request) {
		repo.RepoStatus(s, w, r)
	})
	inner.HandleFunc("/api/repo/check", func(w http.ResponseWriter, r *http.Request) {
		repo.CheckRepo(s, w, r)
	})
	inner.HandleFunc("/api/repo/repair", func(w http.ResponseWriter, r *http.Request) {
		repo.RepairRepo(s, w, r)
	})
}

func registerSnapshotRoutes(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/snapshots", func(w http.ResponseWriter, r *http.Request) {
		snapshot.ListSnapshots(s, w, r)
	})
	inner.HandleFunc("/api/snapshot/", func(w http.ResponseWriter, r *http.Request) {
		snapshot.SnapshotRouter(s, w, r)
	})
	inner.HandleFunc("/api/snapshot-view/", func(w http.ResponseWriter, r *http.Request) {
		snapshot.SnapshotFileRouter(s, w, r)
	})
	inner.HandleFunc("/api/snapshots/delete-batch", func(w http.ResponseWriter, r *http.Request) {
		snapshot.BatchDeleteSnapshots(s, w, r)
	})
	inner.HandleFunc("/api/snapshots/hosts", func(w http.ResponseWriter, r *http.Request) {
		snapshot.ListSnapshotHosts(s, w, r)
	})
}

func registerVolumeRoutes(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/volume/", func(w http.ResponseWriter, r *http.Request) {
		volume.VolumeRouter(s, w, r)
	})
	inner.HandleFunc("/api/volumes", func(w http.ResponseWriter, r *http.Request) {
		volume.ListVolumes(s, w, r)
	})
	inner.HandleFunc("/api/volumes/owners", func(w http.ResponseWriter, r *http.Request) {
		owner.ListVolumeOwners(s, w, r)
	})
}

func registerStatsRoutes(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		repo.Stats(s, w, r)
	})
	inner.HandleFunc("/api/stats/refresh", func(w http.ResponseWriter, r *http.Request) {
		repo.RefreshStats(s, w, r)
	})
}

func registerDevRoutes(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/dev-mode", func(w http.ResponseWriter, r *http.Request) {
		DevMode(s, w, r)
	})
	if os.Getenv("BLT_DEV_MODE") != "" {
		inner.HandleFunc("/api/dummy-volume", func(w http.ResponseWriter, r *http.Request) {
			CreateDummyVolume(s, w, r)
		})
		inner.HandleFunc("/api/dummy-snapshot", func(w http.ResponseWriter, r *http.Request) {
			CreateDummySnapshot(s, w, r)
		})
	}
}

func initUI() (fs.FS, error) {
	var verifyOnce sync.Once
	verifyStaticFiles := func() error {
		var err error
		verifyOnce.Do(func() {
			_, err = StaticFiles.ReadDir("static")
		})
		if err != nil {
			return fmt.Errorf("web assets not found. Run: make ui: %w", err)
		}
		return nil
	}
	if err := verifyStaticFiles(); err != nil {
		return nil, err
	}

	uiFS, err := fs.Sub(StaticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize web static files: %w", err)
	}
	return uiFS, nil
}

func registerUIRoutes(inner *http.ServeMux, uiFS fs.FS) {
	rootFileServer := http.FileServer(http.FS(uiFS))
	uiHandler := http.StripPrefix("/ui", rootFileServer)

	inner.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ui/", http.StatusFound)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		rootFileServer.ServeHTTP(w, r)
	})

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
		if _, err := w.Write(data); err != nil {
			log.Error("write_ui_index_failed", err)
		}
	})
	inner.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})
}
