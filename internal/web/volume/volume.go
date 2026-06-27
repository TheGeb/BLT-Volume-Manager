package volume

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/app/log"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/restic"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func CleanupVolumeData(s *server.Server, volumeName string) {
	if volumeName == "" {
		return
	}
	if s.HasBackend() {
		if volStore, err := s.StoreForVolume(); err == nil {
			if err := volStore.DeleteObjectsWithPrefix(metadata.OwnerFolder(volumeName)); err != nil {
				log.Error("delete_owner_objects_failed", err)
			}
			if err := volStore.DeleteVolumeMarker(volumeName); err != nil {
				log.Error("delete_volume_marker_failed", err)
			}
			if err := volStore.DeleteRestorePoint(volumeName); err != nil {
				log.Error("delete_restore_point_failed", err)
			}
		}
		if s.IsS3Backend() {
			if dataStore, err := s.StoreForResticData(); err == nil && dataStore != nil {
				if err := dataStore.DeleteObjectsWithPrefix(restic.Dir + "/" + volumeName + "/"); err != nil {
					log.Error("delete_restic_data_failed", err)
				}
			}
		}
	}
	if !s.IsS3Backend() {
		repoPath := filepath.Join(s.ResticBase, restic.Dir, volumeName)
		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			log.Error("resolve_repo_path_failed", err)
			return
		}
		basePath, err := filepath.Abs(filepath.Join(s.ResticBase, restic.Dir))
		if err != nil {
			log.Error("resolve_base_path_failed", err)
			return
		}
		if !strings.HasPrefix(absPath+string(filepath.Separator), basePath+string(filepath.Separator)) {
			log.Error("path_traversal", fmt.Errorf("path %q escapes base %q", absPath, basePath))
			return
		}
		if err := os.RemoveAll(absPath); err != nil {
			log.Error("delete_restic_repo_failed", err)
		}
	}
}
