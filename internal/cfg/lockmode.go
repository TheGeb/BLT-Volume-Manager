package cfg

import "strings"

// LockMode controls when a volume owner lock is acquired.
type LockMode string

const (
	// LockModeCreate holds a permanent lock from volume creation until the
	// volume is removed.
	LockModeCreate LockMode = "create"
	// LockModeMount acquires a TTL lock whenever a container mounts the volume.
	LockModeMount LockMode = "mount"
)

// String returns the canonical string form of the lock mode.
func (m LockMode) String() string {
	return string(m)
}

// ParseLockMode parses and normalizes a BLT_LOCK_MODE value, returning the
// empty LockMode ("" ) if the value is not a recognized mode.
func ParseLockMode(s string) LockMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "create":
		return LockModeCreate
	case "mount":
		return LockModeMount
	default:
		return ""
	}
}

// Valid reports whether m is a recognized lock mode.
func (m LockMode) Valid() bool {
	return m == LockModeCreate || m == LockModeMount
}
