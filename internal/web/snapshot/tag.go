package snapshot

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func handleTagSnapshot(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID string) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "missing tag", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		handleAddTag(s, w, r, rm, snapshotID, tag)
	case http.MethodDelete:
		handleRemoveTag(s, w, r, rm, snapshotID, tag)
	default:
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
	}
}

func handleAddTag(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID, tag string) {
	if tag == "restore-point" {
		if err := s.SetRestorePoint(r.URL.Query().Get("volume"), snapshotID); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	} else {
		if err := rm.TagSnapshot(snapshotID, tag); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	}
	resp, err := BuildSnapshotListResponse(s, r.URL.Query().Get("volume"), nil, nil, 0, 0)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp.Status = "tag added"
	server.RespondJSON(w, resp)
}

func handleRemoveTag(s *server.Service, w http.ResponseWriter, r *http.Request, rm *restic.Manager, snapshotID, tag string) {
	if tag == "restore-point" {
		if err := s.DeleteRestorePoint(r.URL.Query().Get("volume")); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	} else {
		if err := rm.UntagSnapshot(snapshotID, tag); err != nil {
			server.RespondError(w, err, http.StatusInternalServerError)
			return
		}
	}
	resp, err := BuildSnapshotListResponse(s, r.URL.Query().Get("volume"), nil, nil, 0, 0)
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	resp.Status = "tag removed"
	server.RespondJSON(w, resp)
}
