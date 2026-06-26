package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/cfg"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg.LoadEnv()

	ctx, stop := app.WithShutdown()
	defer stop()

	var httpAddr string
	var dataDir string
	var showVersion bool
	flag.StringVar(&httpAddr, "http-addr", ":8080", "HTTP address for the BLT Volume Manager UI")
	flag.StringVar(&dataDir, "data-dir", "/var/lib/docker-volumes", "root directory for volumes")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(app.VersionString())
		return 0
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

	mux := http.NewServeMux()
	webSrv := server.New(conf)
	web.Register(webSrv, mux)

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Infof("serving_http_ui", "address=%s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("http_server_failed", err, "address=%s error=%v", httpAddr, err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	webSrv.Shutdown()

	return 0
}
