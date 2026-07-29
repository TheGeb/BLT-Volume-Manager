package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/s3"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
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
	var healthCheck bool
	flag.StringVar(&httpAddr, "http-addr", ":8080", "HTTP address for the BLT Volume Manager UI")
	flag.StringVar(&dataDir, "data-dir", "/var/lib/docker-volumes", "root directory for volumes")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.BoolVar(&healthCheck, "health", false, "perform a health check against the running server and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(app.VersionString())
		return 0
	}

	if healthCheck {
		return runHealthCheck(httpAddr)
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

	b, err := cfg.OpenMetadataBackend(conf)
	if err != nil {
		log.Error("metadata_backend_config_error", err)
		return 1
	}

	mux := http.NewServeMux()

	// Restic data always lives in S3 regardless of the metadata backend.
	s3Client, err := s3.NewClient(s3.Config{
		Bucket:         conf.S3Bucket,
		Endpoint:       conf.S3Endpoint,
		Region:         conf.S3Region,
		ForcePathStyle: conf.S3ForcePathStyle,
	})
	if err != nil {
		log.Error("restic_s3_backend_error", err)
		return 1
	}

	webSrv := server.New(conf, b, server.WithResticBackend(restic.NewS3Backend(s3Client)))
	if err := web.Register(webSrv, mux); err != nil {
		log.Errorf("register_web_routes_failed", err, "http_addr=%s", httpAddr)
		return 1
	}

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

func runHealthCheck(addr string) int {
	host := addr
	if host[0] == ':' {
		host = "localhost" + host
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + host + "/api/health")
	if err != nil {
		log.Error("health_check_failed", err)
		return 1
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error("failed_to_close_response_body", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		log.Errorf("health_check_unhealthy", nil, "status=%d", resp.StatusCode)
		return 1
	}
	return 0
}
