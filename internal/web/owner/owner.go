package owner

import (
	"fmt"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func VolumeOwner(s *server.Server, volumeName string) (map[string]any, error) {
	ms, err := s.StoreForVolume()
	if err != nil {
		return nil, err
	}

	folder := metadata.OwnerFolder(volumeName)
	objects, err := ms.ListObjects(folder) // FIXME: more generic store methods which should use domain based methods
	if err != nil {
		return nil, fmt.Errorf("list owner objects: %w", err)
	}

	metadata.SortOwnersByExpiry(objects)
	objects = metadata.RemoveStaleObjects(ms, objects, metadata.DefaultOwnerTTL) // FIXME: do async?

	result := map[string]any{
		"volume": volumeName,
		"owner":  "",
	}

	key, owner, expiry := metadata.FindOwner(objects)
	if key != "" {
		result["owner"] = owner
		if expiry > 0 {
			result["expiry"] = expiry
		}
	}

	return result, nil
}

func IsVolumeOwned(s *server.Server, volumeName string) (bool, string, error) { // FIXME: consider rename like "ForVolume"?
	status, err := VolumeOwner(s, volumeName)
	if err != nil {
		return false, "", err
	}
	owner, ok := status["owner"].(string)
	if ok && owner != "" {
		return true, owner, nil
	}
	return false, "", nil
}

func CreateVolumeOwner(s *server.Server, volumeName, ownerName string, ownerDurationMins int) (map[string]any, error) {
	ms, err := s.StoreForVolume()
	if err != nil {
		return nil, err
	}

	if ownerName == "" {
		return nil, fmt.Errorf("owner name is required")
	}
	var expiry int64
	if ownerDurationMins > 0 {
		expiry = time.Now().Add(time.Duration(ownerDurationMins) * time.Minute).Unix()
	}

	folder := metadata.OwnerFolder(volumeName)
	_, err = metadata.AcquireOwnerLock(ms, folder, ownerName, expiry)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"volume":     volumeName,
		"owner":      ownerName,
		"expires_at": expiry,
	}, nil
}

func DeleteVolumeOwners(s *server.Server, volumeName string) error {
	ms, err := s.StoreForVolume()
	if err != nil {
		return fmt.Errorf("initialize metadata store: %w", err)
	}

	return ms.DeleteOwnerObjects(metadata.OwnerFolder(volumeName))
}
