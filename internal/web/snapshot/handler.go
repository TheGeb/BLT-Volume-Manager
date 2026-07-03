package snapshot

import (
	"net/http"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

const (
	snapshotOpTag     = "tag"
	snapshotOpRestore = "restore"
	snapshotOpDelete  = "delete"
)

func SnapshotRouter(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot/") {
		http.NotFound(w, r)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/snapshot/")
	if trimmed == "sizes" {
		SnapshotSizes(s, w, r)
		return
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || (parts[1] != snapshotOpTag && parts[1] != snapshotOpRestore && parts[1] != snapshotOpDelete) {
		http.NotFound(w, r)
		return
	}

	snapshotID := parts[0]
	volName, ok := server.RequireVolumeParam(w, r)
	if !ok {
		return
	}
	rm := s.ResticManager(volName)

	switch parts[1] {
	case snapshotOpDelete:
		handleDeleteSnapshot(s, w, r, rm, snapshotID)
	case snapshotOpRestore:
		handleRestoreSnapshot(s, w, r, rm, snapshotID)
	case snapshotOpTag:
		handleTagSnapshot(s, w, r, rm, snapshotID)
	}
}

func SnapshotFileRouter(s *server.Service, w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/snapshot-view/") {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/snapshot-view/")
	parts := strings.Split(rest, "/")

	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	volName := r.URL.Query().Get("volume")
	if volName == "" {
		http.Error(w, server.ErrMissingVolume.Error(), http.StatusBadRequest)
		return
	}
	rm := s.ResticManager(volName)

	rawID := parts[0]
	version := r.URL.Query().Get("tag")
	fallbackHash := r.URL.Query().Get("fallbackHash")
	action := parts[1]

	resolve := func(id, ver, fallback string) string {
		if ver != "" {
			if resolved, err := FindSnapshotByVersion(rm, ver, volName); err == nil {
				return resolved
			}
		}
		return id
	}

	rawID = resolve(rawID, version, fallbackHash)

	switch action {
	case "ls":
		handleListSnapshotFiles(s, w, r, rm, rawID, fallbackHash)
	case "dump":
		handleDumpSnapshotFile(s, w, r, rm, rawID, fallbackHash)
	case "diff":
		secondID := parts[2]
		diffVersion := r.URL.Query().Get("diffTag")
		diffFallback := r.URL.Query().Get("diffFallbackHash")
		secondID = resolve(secondID, diffVersion, diffFallback)
		handleDiffSnapshots(s, w, r, rm, rawID, secondID, fallbackHash, diffFallback)
	default:
		http.NotFound(w, r)
	}
}
