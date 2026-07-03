package restic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

func (m *Manager) GenerateHash(s Snapshot) string {
	paths := make([]string, len(s.Paths))
	copy(paths, s.Paths)
	sort.Strings(paths)

	data := s.Hostname + s.Time.Format(time.RFC3339Nano) + s.Tree + strings.Join(paths, ",")
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

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
