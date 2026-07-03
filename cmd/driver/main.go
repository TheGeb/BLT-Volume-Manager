package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/driver"
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

	if err := os.MkdirAll(dataDir, app.DefaultDirPerm); err != nil {
		log.Errorf("create_data_dir_failed", err, "data_dir=%s error=%v", dataDir, err)
		return 1
	}

	conf, err := cfg.FromEnv(dataDir)
	if err != nil {
		log.Errorf("load_config_failed", err, "data_dir=%s", dataDir)
		return 1
	}
	if err := cfg.ValidateConfig(conf); err != nil {
		log.Error("config_validation_failed", err)
		return 1
	}

	if _, err := cfg.OpenMetadataBackend(conf); err != nil {
		log.Error("metadata_backend_config_error", err)
		return 1
	}

	drv := driver.New(conf, ctx)
	h := volume.NewHandler(drv)

	log.Infof("starting_plugin", "socket=%s data_dir=%s", socketPath, conf.DataDir)
	if err := cleanupSocket(socketPath); err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("socket_cleanup_before_start_failed", err, "path=%s", socketPath)
			return 1
		}
	}

	socketErr := make(chan error, 1)
	go func() {
		socketErr <- h.ServeUnix(socketPath, 0)
	}()

	select {
	case err := <-socketErr:
		if err != nil {
			log.Errorf("serve_unix_failed", err, "socket=%s", socketPath)
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

func cleanupSocket(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	conn, err := net.Dial("unix", path)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("socket already in use: %s", path)
	}

	if err := os.Remove(path); err != nil {
		return err
	}
	log.Info("removed_stale_socket")
	return nil
}
