package migration

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/cfg"
	"github.com/TheGeb/BLT-Volume-Manager/internal/migrate"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

// backendSpec describes one metadata backend as entered in the web UI. S3
// credentials are always taken from the server's environment (the same AWS
// credentials the server itself uses); only connection settings are accepted.
type backendSpec struct {
	Type           string   `json:"type"`
	Bucket         string   `json:"bucket"`
	Endpoint       string   `json:"endpoint"`
	Region         string   `json:"region"`
	ForcePathStyle *bool    `json:"force_path_style,omitempty"`
	EtcdEndpoints  []string `json:"etcd_endpoints,omitempty"`
}

func (b backendSpec) config() (cfg.Config, error) {
	forcePath := true
	if b.ForcePathStyle != nil {
		forcePath = *b.ForcePathStyle
	}
	c := cfg.Config{
		MetadataBackend:  b.Type,
		S3Bucket:         b.Bucket,
		S3Endpoint:       b.Endpoint,
		S3Region:         b.Region,
		S3ForcePathStyle: forcePath,
		EtcdEndpoints:    b.EtcdEndpoints,
	}
	switch b.Type {
	case "s3":
		if c.S3Bucket == "" {
			return cfg.Config{}, fmt.Errorf("s3 backend requires bucket")
		}
	case "etcd":
		if len(c.EtcdEndpoints) == 0 {
			return cfg.Config{}, fmt.Errorf("etcd backend requires etcd_endpoints")
		}
	default:
		return cfg.Config{}, fmt.Errorf("backend type must be s3 or etcd, got %q", b.Type)
	}
	return c, nil
}

// RunMigration handles POST /api/migrate, copying metadata between the
// requested source and destination backends.
func RunMigration(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DryRun bool        `json:"dry_run"`
		From   backendSpec `json:"from"`
		To     backendSpec `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
		return
	}

	fromCfg, err := req.From.config()
	if err != nil {
		server.RespondError(w, err, http.StatusBadRequest)
		return
	}
	toCfg, err := req.To.config()
	if err != nil {
		server.RespondError(w, err, http.StatusBadRequest)
		return
	}

	res, err := migrate.MigrateBackends(r.Context(), fromCfg, toCfg, req.DryRun)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, res)
}
