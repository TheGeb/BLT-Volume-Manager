package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/go-plugins-helpers/volume"
	"github.com/example/blt-volume-manager/driver"
	"github.com/example/blt-volume-manager/web"
)

func main() {
	var dataDir string
	var socketPath string
	var httpAddr string
	var httpOnly bool
	flag.StringVar(&dataDir, "data-dir", "/var/lib/docker-volumes", "root directory for volumes")
	flag.StringVar(&socketPath, "socket", "/run/docker/plugins/s3vol.sock", "unix socket for docker plugin (empty disables plugin)")
	flag.StringVar(&httpAddr, "http-addr", ":8080", "HTTP address for the BLT Volume Manager UI")
	flag.BoolVar(&httpOnly, "http-only", false, "start only the HTTP UI and do not launch the Docker volume plugin")
	flag.Parse()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}
	dataDir, _ = filepath.Abs(dataDir)

	lockBucket := deriveLockBucket()
	s3Endpoint := deriveS3Endpoint()
	s3Region := os.Getenv("S3_REGION")

	if lockBucket != "" {
		log.Printf("lock bucket=%q endpoint=%q region=%q", lockBucket, s3Endpoint, s3Region)
	}

	drv := driver.NewDriver(dataDir, lockBucket, s3Endpoint, s3Region)
	if httpAddr != "" {
		mux := http.NewServeMux()
		web.NewServer(drv.ResticManager(), lockBucket, s3Endpoint, s3Region).Register(mux)
		go func() {
			log.Printf("serving BLT Volume Manager on http://%s", httpAddr)
			if err := http.ListenAndServe(httpAddr, mux); err != nil {
				log.Fatalf("http server: %v", err)
			}
		}()
	}

	if httpOnly || socketPath == "" {
		if !httpOnly {
			log.Printf("socket path is empty; skipping Docker plugin and only serving HTTP UI")
		}
		select {}
	}

	h := volume.NewHandler(drv)
	log.Printf("starting plugin on %s, volumes at %s", socketPath, dataDir)
	if err := h.ServeUnix(socketPath, 0); err != nil {
		log.Fatalf("serve unix: %v", err)
	}
}

func deriveLockBucket() string {
	if b := os.Getenv("S3_LOCK_BUCKET"); b != "" {
		return b
	}

	if repo := os.Getenv("RESTIC_REPOSITORY"); repo != "" {
		repo = strings.TrimPrefix(repo, "s3:")
		if u, err := url.Parse(repo); err == nil && u.Path != "" {
			return strings.Trim(u.Path, "/")
		}
	}

	if ep := os.Getenv("S3_ENDPOINT"); ep != "" {
		if u, err := url.Parse(ep); err == nil && u.Path != "" {
			return strings.Trim(u.Path, "/")
		}
	}

	return ""
}

func deriveS3Endpoint() string {
	ep := os.Getenv("S3_ENDPOINT")
	if ep == "" {
		ep = strings.TrimPrefix(os.Getenv("RESTIC_REPOSITORY"), "s3:")
	}
	if ep == "" {
		return ""
	}
	if !strings.Contains(ep, "://") {
		ep = "https://" + ep
	}
	return ep
}
