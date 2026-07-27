package web

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func registerVersionRoute(s *server.BLTService, inner *http.ServeMux) {
	inner.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		metaBackend := s.Config.MetadataBackend
		if metaBackend == "" {
			if s.Config.S3Bucket != "" {
				metaBackend = "s3"
			} else {
				metaBackend = "none"
			}
		}

		server.RespondJSON(w, map[string]any{
			"version":          app.Version,
			"commit":           app.Commit,
			"date":             app.Date,
			"metadata_backend": metaBackend,
			"s3_endpoint":      s.Config.S3Endpoint,
			"s3_bucket":        s.Config.S3Bucket,
			"etcd_endpoints":   s.Config.EtcdEndpoints,
		})
	})
}
