package store

import "time"

type LockOwner struct {
	Name       string `json:"name"`
	ExpiryTime int64  `json:"expiry_time"`
}

func (l *LockOwner) GetRemainingTimeInSeconds() int64 {
	return l.ExpiryTime - time.Now().Unix()
}

type RestorePoint struct {
	SnapshotID   string `json:"snapshotID"`
	FallbackHash string `json:"fallbackHash"`
}
