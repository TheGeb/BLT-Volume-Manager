package restic

import (
	"context"
	"encoding/json"
	"sort"
)

type HostSnapshots struct {
	Host      string     `json:"host"`
	Snapshots []Snapshot `json:"snapshots"`
}

func (m *Manager) SnapshotHosts(latest int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), TimeoutShort)
	defer cancel()

	out, err := m.runner.HostSnapshots(ctx)
	if err != nil {
		return nil, err
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	hostSet := make(map[string]bool)
	for _, item := range raw {
		var group struct {
			GroupKey struct {
				Hostname string `json:"hostname"`
			} `json:"group_key"`
		}
		if json.Unmarshal(item, &group) == nil && group.GroupKey.Hostname != "" {
			hostSet[group.GroupKey.Hostname] = true
		}
	}

	hosts := make([]string, 0, len(hostSet))
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts, nil
}
