package restic

import "time"

const (
	BackupTagHot  = "hot"
	BackupTagCold = "cold"

	ResticDir = "restic"

	ResticTimeoutShort  = 2 * time.Minute
	ResticTimeoutMedium = 10 * time.Minute
	ResticTimeoutLong   = 30 * time.Minute
)
