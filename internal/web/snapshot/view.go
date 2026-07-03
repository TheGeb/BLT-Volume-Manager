package snapshot

import (
	"net/http"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func handleListSnapshotFiles(s *server.Server, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	nodes, err := rm.ListSnapshotFiles(rawID, path)
	if err != nil && fallbackHash != "" {
		if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
			nodes, err = rm.ListSnapshotFiles(snap.ID, path)
		}
	}
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	server.RespondJSON(w, nodes)
}

func handleDumpSnapshotFile(s *server.Server, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, fallbackHash string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}
	data, err := rm.DumpFile(rawID, path)
	if err != nil && fallbackHash != "" {
		if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
			data, err = rm.DumpFile(snap.ID, path)
		}
	}
	if err != nil {
		server.RespondError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func handleDiffSnapshots(s *server.Server, w http.ResponseWriter, r *http.Request, rm *restic.Manager, rawID, secondID, fallbackHash, diffFallback string) {
	if r.Method != http.MethodGet {
		http.Error(w, server.ErrMethodNotAllowed.Error(), http.StatusMethodNotAllowed)
		return
	}
	result, err := rm.DiffSnapshots(rawID, secondID)
	if err != nil {
		resolvedFirst, resolvedSecond := rawID, secondID
		if fallbackHash != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
				resolvedFirst = snap.ID
			}
		}
		if diffFallback != "" {
			if snap, lookupErr := rm.FindSnapshotByHash(diffFallback); lookupErr == nil {
				resolvedSecond = snap.ID
			}
		}
		result, err = rm.DiffSnapshots(resolvedFirst, resolvedSecond)
	}
	if err != nil {
		server.RespondError(w, err, http.StatusNotFound)
		return
	}
	server.RespondJSON(w, result)
}
