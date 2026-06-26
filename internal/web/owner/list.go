package owner

import (
	"errors"
	"net/http"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func HandleVolumeOwners(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.RespondError(w, errors.New("method not allowed"), http.StatusMethodNotAllowed)
		return
	}

	owners := make(map[string]map[string]any)

	if !s.HasBackend() {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	s3Store, err := s.GetOrCreateMetadataStore()
	if err != nil || s3Store == nil {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	objects, listErr := s3Store.ListObjects(metadata.OwnerPrefix)
	if listErr != nil {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	kept := metadata.FilterStaleOwnerObjects(s3Store, objects, metadata.DefaultOwnerTTL)

	grouped := make(map[string][]metadata.Object)
	for _, obj := range kept {
		vol, _, _, err := metadata.ParseOwnerKey(*obj.Key)
		if err != nil && !errors.Is(err, metadata.ErrOldOwnerKeyFormat) {
			continue
		}
		if vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	for vol, objs := range grouped {
		metadata.SortOwnerObjects(objs)
		key, ownerName, expiry := metadata.FilterValidOwnersByKey(s3Store, objs)
		if key != "" {
			result := map[string]any{
				"volume": vol,
				"owned":  true,
				"owner":  ownerName,
			}
			if expiry > 0 {
				remaining := expiry - time.Now().Unix()
				if remaining < 0 {
					remaining = 0
				}
				result["expires_in"] = remaining
			}
			owners[vol] = result
		}
	}

	server.RespondJSON(w, map[string]any{"owners": owners})
}
