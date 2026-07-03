package cfg

import "fmt"

func ValidateConfig(c Config) error {
	if c.ResticBase == "" {
		return fmt.Errorf("RESTIC_REPOSITORY must be set")
	}
	if c.OwnerMaxMins < 0 {
		return fmt.Errorf("OWNER_MAX_MINS must be >= 0, got %d", c.OwnerMaxMins)
	}
	if c.MetadataBackend != "" && c.MetadataBackend != "s3" && c.MetadataBackend != "etcd" {
		return fmt.Errorf("BLT_METADATA_BACKEND must be \"s3\" or \"etcd\", got %q", c.MetadataBackend)
	}
	return nil
}
