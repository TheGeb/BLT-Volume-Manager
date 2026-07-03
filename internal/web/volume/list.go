package volume

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func ListVolumes(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	volumes := s.VolumeNames()
	if volumes == nil {
		volumes = []string{}
	}
	server.RespondJSON(w, map[string]any{"volumes": volumes})
}
