package snapshot

import (
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
)

type WithVolume struct {
	restic.Snapshot
	Volume string `json:"volume"`
}

func FindSnapshotByVersion(rm *restic.Manager, version string) (string, error) {
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
	for _, s := range snapshots[1:] { //TODO: warn if there are multiple matches?
		if s.Time.After(newest.Time) {
			newest = s
		}
	}
	return newest.ID, nil
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
