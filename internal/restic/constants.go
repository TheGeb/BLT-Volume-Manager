package restic

import "time"

const (
	BackupTagHot  = "hot"
	BackupTagCold = "cold"

	Dir = "restic"

	//TODO: Shorten timeouts and/or ensure web UI is aware when a long timeout is possible
	TimeoutShort  = 2 * time.Minute
	TimeoutMedium = 10 * time.Minute
	TimeoutLong   = 30 * time.Minute
)
