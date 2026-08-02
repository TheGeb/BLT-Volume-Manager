package cfg

import (
	"os"

	"github.com/joho/godotenv"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
)

const configFileEnvVar = "BLT_CONFIG_FILE"

// LoadEnv loads ".env" from the working directory (if present) and then the
// file referenced by BLT_CONFIG_FILE (if set). godotenv never overrides
// variables that are already set, so the process environment wins over both
// files.
func LoadEnv() {
	_ = godotenv.Load()
	if f := os.Getenv(configFileEnvVar); f != "" {
		warnOnLoosePerms(f)
		_ = godotenv.Load(f)
	}
}

func warnOnLoosePerms(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	if permsTooLoose(fi.Mode().Perm()) {
		log.Warnf("config_file_permissions_loose",
			"config file %s is accessible by other users (mode %#o); consider chmod 600", path, fi.Mode().Perm())
	}
}

// permsTooLoose reports whether any "other" permission bit is set, i.e. the
// file is readable/writable by users other than its owner.
func permsTooLoose(perm os.FileMode) bool {
	return perm&0o007 != 0
}
