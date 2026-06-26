package owner

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func OwnerRouter(s *server.Server, w http.ResponseWriter, r *http.Request, volumeName string) {
	switch r.Method {
	case http.MethodGet:
		status, err := VolumeOwner(s, volumeName)
		if err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, status)

	case http.MethodPost:
		var reqBody struct {
			Owner             string `json:"owner"`
			OwnerDurationMins int    `json:"owner_duration_mins"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
				return
			}
		}
		ownerData, err := CreateVolumeOwner(s, volumeName, reqBody.Owner, reqBody.OwnerDurationMins)
		if err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, ownerData)

	case http.MethodDelete:
		if err := DeleteVolumeOwners(s, volumeName); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, map[string]string{"status": "owners deleted"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
