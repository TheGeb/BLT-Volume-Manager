package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/appconfig"
	"github.com/TheGeb/BLT-Volume-Manager/internal/applog"
	"github.com/TheGeb/BLT-Volume-Manager/internal/version"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var httpAddr string
	var showVersion bool
	flag.StringVar(&httpAddr, "http-addr", ":8080", "HTTP address for the BLT Volume Manager UI")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version.String())
		return 0
	}

	cfg, err := appconfig.FromEnv("/var/lib/docker-volumes")
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

	mux := http.NewServeMux()
	webSrv := web.NewServer(cfg)
	webSrv.Register(mux)

	srv := &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		applog.Infof("serving_http_ui", "address=%s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			applog.Errorf("http_server_failed", err, "address=%s error=%v", httpAddr, err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	webSrv.Shutdown()

	return 0
}
