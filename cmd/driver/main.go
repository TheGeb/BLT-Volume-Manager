package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheGeb/BLT-Volume-Manager/internal/appconfig"
	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
	"github.com/TheGeb/BLT-Volume-Manager/internal/driver"
	"github.com/TheGeb/BLT-Volume-Manager/internal/version"
	"github.com/docker/go-plugins-helpers/volume"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var dataDir string
	var socketPath string
	var showVersion bool
	flag.StringVar(&dataDir, "data-dir", "/var/lib/docker-volumes", "root directory for volumes")
	flag.StringVar(&socketPath, "socket", "/run/docker/plugins/blt-volume-manager.sock", "unix socket for docker plugin")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version.String())
		return 0
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		applog.Errorf("create_data_dir_failed", err, "data_dir=%s error=%v", dataDir, err)
		return 1
	}

	cfg, err := appconfig.FromEnv(dataDir)
	if err != nil {
		applog.Errorf("load_config_failed", err, "error=%v", err)
		return 1
	}
	if cfg.ResticBase == "" {
		applog.Error("restic_repository_required", fmt.Errorf("RESTIC_REPOSITORY must be set"))
		return 1
	}
	if cfg.S3Bucket != "" {
		applog.Info("s3_configured")
	}

	drv := driver.NewDriver(cfg)
	h := volume.NewHandler(drv)

	applog.Infof("starting_plugin", "socket=%s data_dir=%s", socketPath, cfg.DataDir)
	socketErr := make(chan error, 1)
	go func() {
		socketErr <- h.ServeUnix(socketPath, 0)
	}()

	select {
	case err := <-socketErr:
		if err != nil {
			applog.Errorf("serve_unix_failed", err, "error=%v", err)
			return 1
		}
	case <-ctx.Done():
		applog.Info("shutting_down")
		_ = os.Remove(socketPath)
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		applog.Warnf("socket_cleanup_failed", "path=%s", socketPath)
	}
	return 0
}
