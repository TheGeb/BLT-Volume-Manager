package snapshot

import (
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func ResolveSnapshotID(rm *restic.Manager, rawID, version, volName string) string {
	if version != "" {
		if resolved, err := FindSnapshotByVersion(rm, version, volName); err == nil {
			return resolved
		}
	}
	return rawID
}

func withFallback[T any](rm *restic.Manager, rawID, fallbackHash string, fn func(string) (T, error)) (T, error) {
	result, err := fn(rawID)
	if err != nil && fallbackHash != "" {
		if snap, lookupErr := rm.FindSnapshotByHash(fallbackHash); lookupErr == nil {
			result, err = fn(snap.ID)
		}
	}
	return result, err
}

type WithVolume struct { // FIXME: Naming is a bit awkward
	restic.Snapshot
	Volume string `json:"volume"`
}

type SnapshotListResponse struct {
	Snapshots      []WithVolume `json:"snapshots"`
	RestorePointID string       `json:"restorePointID"`
	HasMore        bool         `json:"hasMore"`
	Status         string       `json:"status,omitempty"`
}

type snapshotNotFoundError struct {
	version string
}

func (e *snapshotNotFoundError) Error() string {
	return "snapshot not found for version " + e.version
}

type batchDeleteError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

type batchDeleteResponse struct {
	Deleted int                `json:"deleted"`
	Failed  int                `json:"failed"`
	Errors  []batchDeleteError `json:"errors"`
}

func FindSnapshotByVersion(rm *restic.Manager, version, volumeName string) (string, error) {
	tag := version
	if !strings.HasPrefix(version, "v") {
		tag = "v" + version
	}
	snapshots, err := rm.ListSnapshotsWithOpts(&restic.ListSnapshotsOpts{Tags: []string{tag}})
	if err != nil {
		return "", err
	}
	if len(snapshots) == 0 {
		return "", &snapshotNotFoundError{version: version}
	}
	newest := snapshots[0]
	if len(snapshots) > 1 {
		log.Warnf("multiple_snapshots_found", "version=%s count=%d volume=%s", version, len(snapshots), volumeName)
	}
	for _, s := range snapshots[1:] {
		if s.Time.After(newest.Time) {
			newest = s
		}
	}
	return newest.ID, nil
}

func BuildSnapshotListResponse(s *server.Service, volName string, opts *restic.ListSnapshotsOpts, filter *SnapshotFilter, offset, limit int) (*SnapshotListResponse, error) {
	rm := s.ResticManager(volName)
	snaps, err := rm.ListSnapshotsWithOpts(opts)
	if err != nil {
		return nil, err
	}

	snaps = ApplySnapshotFilter(snaps, filter)

	rawLen := len(snaps)
	hasMore := false
	if limit > 0 {
		switch {
		case offset+limit <= rawLen:
			hasMore = rawLen > offset+limit
			snaps = snaps[offset : offset+limit]
		case offset < rawLen:
			snaps = snaps[offset:]
		default:
			snaps = nil
		}
	} else if offset > 0 && offset < rawLen {
		snaps = snaps[offset:]
	}

	restorePointID := ""
	if id, err := s.FindRestorePointByName(volName); err == nil {
		restorePointID = id
	}

	result := make([]WithVolume, 0, len(snaps))
	for _, snap := range snaps {
		fullHash := rm.GenerateHash(snap)
		snap.FallbackHash = fullHash[:len(snap.ShortID)]
		result = append(result, WithVolume{Snapshot: snap, Volume: volName})
	}

	return &SnapshotListResponse{
		Snapshots:      result,
		RestorePointID: restorePointID,
		HasMore:        hasMore,
	}, nil
}
