package owner

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func ListVolumeOwners(s *server.Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.RespondError(w, server.ErrMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	owners := make(map[string]map[string]any)

	s3Store, err := s.MetadataStore() // FIXME: Remove S3 naming for generic metadata store objects
	if err != nil || s3Store == nil {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	objects, listErr := s3Store.ListObjects(metadata.OwnerPrefix) // FIXME: Move to owner file and do not have consumers pass the prefix
	if listErr != nil {
		server.RespondJSON(w, map[string]any{"owners": owners})
		return
	}

	// TODO: refactor to group by active/stale, then remove all stale ones async and proceed acting on active ones?
	kept := metadata.RemoveStaleObjects(s3Store, objects, metadata.DefaultOwnerTTL)

	grouped := make(map[string][]metadata.Object)
	for _, obj := range kept {
		vol, _, _, err := metadata.ParseOwnerKey(*obj.Key)
		if err != nil {
			continue
		}
		if vol == "" {
			continue
		}
		grouped[vol] = append(grouped[vol], obj)
	}

	for vol, objs := range grouped {
		metadata.SortOwnersByExpiry(objs)
		key, ownerName, expiry := metadata.FindOwner(objs)
		if key != "" {
			result := map[string]any{
				"volume": vol,
				"owner":  ownerName,
			}
			if expiry > 0 {
				result["expiry"] = expiry
			}
			owners[vol] = result
		}
	}

	server.RespondJSON(w, map[string]any{"owners": owners})
}
