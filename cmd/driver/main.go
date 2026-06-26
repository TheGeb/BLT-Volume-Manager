package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/cfg"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/driver"
	"github.com/docker/go-plugins-helpers/volume"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg.LoadEnv()

	ctx, stop := app.WithShutdown()
	defer stop()

	var dataDir string
	var socketPath string
	var showVersion bool
	flag.StringVar(&dataDir, "data-dir", "/var/lib/docker-volumes", "root directory for volumes")
	flag.StringVar(&socketPath, "socket", "/run/docker/plugins/blt-volume-manager.sock", "unix socket for docker plugin")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(app.VersionString())
		return 0
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Errorf("create_data_dir_failed", err, "data_dir=%s error=%v", dataDir, err)
		return 1
	}

	conf, err := cfg.FromEnv(dataDir)
	if err != nil {
		log.Errorf("load_config_failed", err, "error=%v", err)
		return 1
	}
	if err := cfg.ValidateConfig(conf); err != nil {
		log.Error("config_validation_failed", err)
		return 1
	}
	if conf.MetadataBackend != "" || conf.S3Bucket != "" {
		log.Info("metadata_backend_configured")
	}

	drv := driver.New(conf, ctx)
	h := volume.NewHandler(drv)

	log.Infof("starting_plugin", "socket=%s data_dir=%s", socketPath, conf.DataDir)
	socketErr := make(chan error, 1)
	go func() {
		socketErr <- h.ServeUnix(socketPath, 0)
	}()

	select {
	case err := <-socketErr:
		if err != nil {
			log.Errorf("serve_unix_failed", err, "error=%v", err)
			return 1
		}
	case <-ctx.Done():
		log.Info("shutting_down")
	}

	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Warnf("socket_cleanup_failed", "path=%s", socketPath)
	}
	return 0
}
