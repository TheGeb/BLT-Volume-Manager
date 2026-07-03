package owner

import (
	"errors"
	"net/http"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func ListVolumeOwners(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	owners := make(map[string]map[string]any)

	if !s.HasBackend() { //FIXME: No backend should return error
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	s3Store, err := s.MetadataStore() //FIXME: Remove S3 naming for generic metadata store objects
	if err != nil || s3Store == nil {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	objects, listErr := s3Store.ListObjects(metadata.OwnerPrefix) //FIXME: Move to owner file and do not have consumers pass the prefix
	if listErr != nil {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}
	
	//TODO: refactor to group by active/stale, then remove all stale ones async and proceed acting on active ones?
	kept := metadata.RemoveStaleObjects(s3Store, objects, metadata.DefaultOwnerTTL)

	grouped := make(map[string][]metadata.Object)
	for _, obj := range kept {
		vol, _, _, err := metadata.ParseOwnerKey(*obj.Key)
		if err != nil && !errors.Is(err, metadata.ErrOldOwnerKeyFormat) { //FIXME: remove "old format" checks
			continue
		}
		if vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	for vol, objs := range grouped {
		metadata.SortOwnerObjects(objs)
		key, ownerName, expiry := metadata.FindOwner(s3Store, objs)
		if key != "" {
			result := map[string]any{
				"volume": vol,
				"owned":  true, //FIXME: owned is redundant when owner is already present
				"owner":  ownerName,
			}
			if expiry > 0 {
				remaining := expiry - time.Now().Unix()
				if remaining < 0 {
					remaining = 0
				}
				result["expires_in"] = remaining //FIXME: Returning expiry is preferred instead of stale "expires in"
			}
			owners[vol] = result
		}
	}

	server.RespondJSON(w, map[string]any{"owners": owners})
}
