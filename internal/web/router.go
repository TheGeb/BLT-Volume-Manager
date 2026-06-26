package web

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/owner"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/repo"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/snapshot"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/volume"
)

func Register(s *server.Server, mux *http.ServeMux) {
	inner := http.NewServeMux()

	inner.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if s.ResticBase == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	inner.HandleFunc("/api/repo/init", func(w http.ResponseWriter, r *http.Request) {
		repo.InitRepo(s, w, r)
	})
	inner.HandleFunc("/api/repo/status", func(w http.ResponseWriter, r *http.Request) {
		repo.RepoStatus(s, w, r)
	})

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

	inner.HandleFunc("/api/volume/", func(w http.ResponseWriter, r *http.Request) {
		volume.VolumeRouter(s, w, r)
	})

	inner.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		repo.Stats(s, w, r)
	})
	inner.HandleFunc("/api/stats/refresh", func(w http.ResponseWriter, r *http.Request) {
		repo.RefreshStats(s, w, r)
	})

	inner.HandleFunc("/api/volumes", func(w http.ResponseWriter, r *http.Request) {
		volume.ListVolumes(s, w, r)
	})
	inner.HandleFunc("/api/volumes/owners", func(w http.ResponseWriter, r *http.Request) {
		owner.ListVolumeOwners(s, w, r)
	})

	inner.HandleFunc("/api/repo/check", func(w http.ResponseWriter, r *http.Request) {
		repo.CheckRepo(s, w, r)
	})
	inner.HandleFunc("/api/repo/repair", func(w http.ResponseWriter, r *http.Request) {
		repo.RepairRepo(s, w, r)
	})

	inner.HandleFunc("/api/dev-mode", func(w http.ResponseWriter, r *http.Request) {
		DevMode(s, w, r)
	})
	if os.Getenv("BLT_TEST_MODE") != "" {
		inner.HandleFunc("/api/dummy-volume", func(w http.ResponseWriter, r *http.Request) {
			CreateDummyVolume(s, w, r)
		})
		inner.HandleFunc("/api/dummy-snapshot", func(w http.ResponseWriter, r *http.Request) {
			CreateDummySnapshot(s, w, r)
		})
	}

	var verifyOnce sync.Once
	verifyStaticFiles := func() {
		verifyOnce.Do(func() {
			if _, err := StaticFiles.ReadDir("static"); err != nil {
				panic("web assets not found. Run: make ui")
			}
		})
	}
	verifyStaticFiles()

	uiFS, err := fs.Sub(StaticFiles, "static")
	if err != nil {
		panic(err)
	}
	rootFileServer := http.FileServer(http.FS(uiFS))
	uiHandler := http.StripPrefix("/ui", rootFileServer)

	inner.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/ui/", http.StatusFound)
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
		_, _ = w.Write(data)
	})
	inner.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/", http.StatusFound)
	})

	mux.Handle("/", server.NoSniff(server.Gzip(server.Logging(inner))))
}
