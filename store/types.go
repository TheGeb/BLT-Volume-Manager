package store

import "time"

type LockOwner struct {
    Name       string `json:"name"`
    ExpiryTime int64  `json:"expiry_time"`
}

func (l *LockOwner) GetRemainingTimeinSeconds() int64 {
    return l.ExpiryTime - time.Now().Unix()
}


