package constants

import "time"

// Backup types
const (
	BackupTagHot     = "hot"
	BackupTagCold    = "cold"
	BackupTagRestore = "restore-point"
)

// Snapshot suffixes
const (
	ColdSnapSuffix   = "-cold-snapshot"
	PreRestoreSuffix = "-pre-restore"
)

// Directory names
const (
	VolumesDir   = "volumes"
	LocksDir     = "locks"
	SnapshotsDir = "snapshots"
	ResticDir    = "restic"
)

// File names
const VolumeConfigFile = "volume.json"

// Time durations
const (
	HotBackupInterval   = 15 * time.Minute
	OrphanCheckInterval = 30 * time.Minute
	OrphanRetryMinAge   = 10 * time.Minute

	ResticTimeoutShort  = 2 * time.Minute
	ResticTimeoutMedium = 10 * time.Minute
	ResticTimeoutLong   = 30 * time.Minute
)

// Lock durations
const (
	DefaultLockMaxHoldMins    = 10
	DefaultLockTTL            = 24 * time.Hour
	DefaultLockAcquireTimeout = 5 * time.Second
)
