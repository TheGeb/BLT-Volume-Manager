package volume

import (
	"net/http"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func HandleVolumes(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	volumes := s.VolumeNames()
	if volumes == nil {
		volumes = []string{}
	}
	server.RespondJSON(w, map[string]any{"volumes": volumes})
}
