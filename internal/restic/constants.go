package restic

import "time"

const (
	BackupTagHot  = "hot"
	BackupTagCold = "cold"

	Dir = "restic"

	TimeoutShort  = 2 * time.Minute
	TimeoutMedium = 10 * time.Minute
	TimeoutLong   = 30 * time.Minute
)
