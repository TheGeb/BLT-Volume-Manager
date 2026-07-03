package app

import "os"

const (
	// DefaultDirPerm is the default permission mode for directories:
	// owner (rwx), group (r-x), other (r-x).
	DefaultDirPerm os.FileMode = 0o755
	// DefaultFilePerm is the default permission mode for files:
	// owner (rw-), group (r--), other (r--).
	DefaultFilePerm os.FileMode = 0o644
)
