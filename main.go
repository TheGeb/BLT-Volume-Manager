package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/example/blt-volume-manager/internal/appconfig"
	"github.com/example/blt-volume-manager/internal/applog"
	"github.com/example/blt-volume-manager/internal/driver"
	"github.com/example/blt-volume-manager/internal/web"
)

func main() {
	var dataDir string
	var socketPath string
	var httpAddr string
	var httpOnly bool
	flag.StringVar(&dataDir, "data-dir", "/var/lib/docker-volumes", "root directory for volumes")
	flag.StringVar(&socketPath, "socket", "/run/docker/plugins/blt-volume-manager.sock", "unix socket for docker plugin (empty disables plugin)")
	flag.StringVar(&httpAddr, "http-addr", ":8080", "HTTP address for the BLT Volume Manager UI")
	flag.BoolVar(&httpOnly, "http-only", false, "start only the HTTP UI and do not launch the Docker volume plugin")
	flag.Parse()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		applog.Errorf("create_data_dir_failed", err, "data_dir=%s error=%v", dataDir, err)
		os.Exit(1)
	}

	cfg, err := appconfig.FromEnv(dataDir)
	if err != nil {
		applog.Errorf("load_config_failed", err, "error=%v", err)
		os.Exit(1)
	}
	if cfg.ResticBase == "" {
		applog.Error("restic_repository_required", fmt.Errorf("RESTIC_REPOSITORY must be set"))
		os.Exit(1)
	}
	if cfg.S3Bucket != "" {
		applog.Info("s3_configured")
	}

	drv := driver.NewDriver(cfg)
	if httpAddr != "" {
		mux := http.NewServeMux()
		web.NewServer(cfg).Register(mux)
		go func() {
			applog.Infof("serving_http_ui", "address=%s", httpAddr)
			if err := http.ListenAndServe(httpAddr, mux); err != nil {
				applog.Errorf("http_server_failed", err, "address=%s error=%v", httpAddr, err)
			}
		}()
	}

	if httpOnly || socketPath == "" {
		if !httpOnly {
			applog.Info("skipping_docker_plugin_http_only_mode")
		}
		select {}
	}

	h := volume.NewHandler(drv)
	applog.Infof("starting_plugin", "socket=%s data_dir=%s", socketPath, cfg.DataDir)
	if err := h.ServeUnix(socketPath, 0); err != nil {
		applog.Errorf("serve_unix_failed", err, "error=%v", err)
		os.Exit(1)
	}
}
