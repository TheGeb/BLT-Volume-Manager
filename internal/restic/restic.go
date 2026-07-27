package restic

import (
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/restic/cli"
)

// Manager provides operations against a single restic repository.
type Manager struct {
	repo   string
	runner *cli.Runner
}

// Snapshot is a restic snapshot as returned by the JSON API.
type Snapshot struct {
	ID           string    `json:"id"`
	ShortID      string    `json:"short_id"`
	Time         time.Time `json:"time"`
	Tree         string    `json:"tree"`
	Tags         []string  `json:"tags"`
	Paths        []string  `json:"paths"`
	Hostname     string    `json:"hostname"`
	FallbackHash string    `json:"fallbackHash,omitempty"`
}

const (
	BackupTagHot  = "hot"
	BackupTagCold = "cold"

	Dir = "restic"

	// TODO: Shorten timeouts and/or ensure web UI is aware when a long timeout is possible
	TimeoutShort  = 2 * time.Minute
	TimeoutMedium = 10 * time.Minute
	TimeoutLong   = 30 * time.Minute
)

// NewManager creates a Manager for the given repo path.
func NewManager(repo string) *Manager {
	return &Manager{repo: repo, runner: &cli.Runner{Repo: repo}}
}

// WithTags prepends base to extra and returns the combined tag slice.
// This is a helper for constructing restic tag arguments.
func WithTags(base string, extra ...string) []string {
	return append([]string{base}, extra...)
}

func (m *Manager) Repo() string { return m.repo }
