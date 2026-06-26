package cfg

import "fmt"

func ValidateConfig(c Config) error {
	if c.ResticBase == "" {
		return fmt.Errorf("RESTIC_REPOSITORY must be set")
	}
	return nil
}
