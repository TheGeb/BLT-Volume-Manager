package owner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type OwnerEntry struct {
	Volume string `json:"volume"`
	Owner  string `json:"owner"`
	Expiry int64  `json:"expiry,omitempty"`
}

type OwnersListResponse struct {
	Owners map[string]OwnerEntry `json:"owners"`
}

func OwnerRouter(s *server.BLTService, w http.ResponseWriter, r *http.Request, volumeName string) {
	ctx := r.Context()
	os := s.OwnerStore()

	switch r.Method {
	case http.MethodGet:
		status, err := VolumeOwner(ctx, os, volumeName)
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
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil && err != io.EOF {
				server.RespondError(w, fmt.Errorf("invalid JSON: %w", err), http.StatusBadRequest)
				return
			}
		}
		ownerData, err := CreateVolumeOwner(ctx, os, volumeName, reqBody.Owner, reqBody.OwnerDurationMins)
		if err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, ownerData)

	case http.MethodDelete:
		if err := DeleteVolumeOwners(ctx, os, volumeName); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
		server.RespondJSON(w, server.StatusResponse{Status: "owners deleted"})

	default:
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
	}
}

func ListVolumeOwners(s *server.BLTService, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	grouped, err := s.OwnerStore().ListAllGrouped(r.Context())
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}

	owners := make(map[string]OwnerEntry, len(grouped))
	for vol, vo := range grouped {
		entry := OwnerEntry{Volume: vol, Owner: vo.Owner}
		if vo.Expiry > 0 {
			entry.Expiry = vo.Expiry
		}
		owners[vol] = entry
	}

	server.RespondJSON(w, OwnersListResponse{Owners: owners})
}
