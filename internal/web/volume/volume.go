package volume

import (
	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func CleanupVolumeData(s *server.Service, volumeName string) {
	if volumeName == "" {
		return
	}
	if err := s.DeleteVolumeData(volumeName); err != nil {
		log.Error("cleanup_volume_data_failed", err)
	}
}
