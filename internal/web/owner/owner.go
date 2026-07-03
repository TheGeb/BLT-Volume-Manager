package owner

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/TheGeb/docker-s3-volume-plugin/internal/metadata"
	"github.com/TheGeb/docker-s3-volume-plugin/internal/web/server"
)

func VolumeOwner(s *server.Server, volumeName string) (map[string]any, error) {
	if !s.HasBackend() { //FIXME: can all these "has backend" checks be done on startup instead of all over the app?
		return nil, errors.New("metadata backend not configured: set META_BACKEND, METADATA_S3_BUCKET, or S3_ENDPOINT")
	}

	ms, err := s.StoreForVolume()
	if err != nil {
		return nil, err
	}

	folder := metadata.OwnerFolder(volumeName)
	objects, err := ms.ListObjects(folder) //FIXME: more generic store methods which should use domain based methods
	if err != nil {
		return nil, fmt.Errorf("list owner objects: %w", err)
	}

	metadata.SortOwnerObjects(objects)
	objects = metadata.RemoveStaleObjects(ms, objects, metadata.DefaultOwnerTTL) //FIXME: do async?

	result := map[string]any{
		"volume": volumeName,
		"owned":  false,
	}

	key, owner, expiry := metadata.FindOwner(ms, objects)
	if key != "" { //FIXME: similar issue to above, owned is redundant and use expiry instead of "expires in" which is stale and can be misleading
		result["owned"] = true
		result["owner"] = owner
		if expiry > 0 {
			remaining := expiry - time.Now().Unix()
			if remaining < 0 {
				remaining = 0
			}
			result["expires_in"] = remaining
		}
	}

	return result, nil
}

func IsVolumeOwned(s *server.Server, volumeName string) (bool, string, error) { //FIXME: consider rename like "ForVolume"?
	status, err := VolumeOwner(s, volumeName)
	if err != nil {
		return false, "", err
	}
	if owned, ok := status["owned"].(bool); ok && owned {
		owner, ok := status["owner"].(string)
		if !ok {
			owner = ""
		}
		return true, owner, nil
	}
	return false, "", nil
}

func CreateVolumeOwner(s *server.Server, volumeName, ownerName string, ownerDurationMins int) (map[string]any, error) {
	if !s.HasBackend() {
		return nil, errors.New("metadata backend not configured: set META_BACKEND, METADATA_S3_BUCKET, or S3_ENDPOINT")
	}

	ms, err := s.StoreForVolume()
	if err != nil {
		return nil, err
	}

	if ownerName == "" {
		ownerName = fmt.Sprintf("webadmin-%s-%d", metadata.Hostname(), os.Getpid()) //FIXME: remove default, perhaps return error instead
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
	if !s.HasBackend() {
		return errors.New("metadata backend not configured")
	}

	ms, err := s.StoreForVolume()
	if err != nil {
		return fmt.Errorf("initialize metadata store: %w", err)
	}

	return ms.DeleteOwnerObjects(metadata.OwnerFolder(volumeName))
}
