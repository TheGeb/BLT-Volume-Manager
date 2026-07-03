package cfg

import (
	"fmt"
	"net/url"
	"strings"
)

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
	if c.S3Endpoint != "" {
		if _, err := url.Parse(c.S3Endpoint); err != nil {
			return fmt.Errorf("S3_ENDPOINT %q is not a valid URL: %w", c.S3Endpoint, err)
		}
		if !strings.Contains(c.S3Endpoint, "://") {
			return fmt.Errorf("S3_ENDPOINT %q must include a scheme (e.g. https://)", c.S3Endpoint)
		}
	}
	for _, ep := range c.EtcdEndpoints {
		if _, err := url.Parse(ep); err != nil {
			return fmt.Errorf("ETCD_ENDPOINTS entry %q is not a valid URL", ep)
		}
	}
	return nil
}
