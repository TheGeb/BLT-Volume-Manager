package restic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
)

type Manager struct {
	repo string
}

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

func NewManager(repo string) *Manager { return &Manager{repo: repo} }

func WithTags(base string, extra ...string) []string {
	return append([]string{base}, extra...)
}

func (m *Manager) Repo() string { return m.repo }

// TODO: Consider moving to another file
func (m *Manager) FindSnapshotByHash(hash string) (*Snapshot, error) {
	snapshots, err := m.ListSnapshots()
	if err != nil {
		return nil, err
	}

	for _, s := range snapshots {
		fullHash := m.GenerateHash(s)
		shortHash := fullHash[:len(s.ShortID)]
		log.Debugf("comparing_hash", "hash=%x snapshot=%s", hash, s.ID)
		if shortHash == hash {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("snapshot not found for hash %s", hash)
}

func (m *Manager) GenerateHash(s Snapshot) string {
	paths := make([]string, len(s.Paths))
	copy(paths, s.Paths)
	sort.Strings(paths)

	data := s.Hostname + s.Time.Format(time.RFC3339Nano) + s.Tree + strings.Join(paths, ",")
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}
