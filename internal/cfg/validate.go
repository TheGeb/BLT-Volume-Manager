package cfg

import "fmt"

func ValidateConfig(c Config) error {
	//TODO: More validation
	if c.ResticBase == "" {
		return fmt.Errorf("RESTIC_REPOSITORY must be set")
	}
	return nil
}
