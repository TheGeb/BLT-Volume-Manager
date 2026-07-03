package owner

import (
	"fmt"

	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

type OwnerInfo struct {
	Volume string `json:"volume"`
	Owner  string `json:"owner"`
	Expiry int64  `json:"expiry,omitempty"`
}

type CreateOwnerResponse struct {
	Volume    string `json:"volume"`
	Owner     string `json:"owner"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

func VolumeOwner(s *server.Service, volumeName string) (*OwnerInfo, error) {
	os, err := s.OwnerStore()
	if err != nil {
		return nil, err
	}
	vo, err := os.FindForVolume(volumeName)
	if err != nil {
		return nil, err
	}
	return &OwnerInfo{Volume: vo.Volume, Owner: vo.Owner, Expiry: vo.Expiry}, nil
}

func IsVolumeOwned(s *server.Service, volumeName string) (bool, string, error) {
	status, err := VolumeOwner(s, volumeName)
	if err != nil {
		return false, "", err
	}
	if status.Owner != "" {
		return true, status.Owner, nil
	}
	return false, "", nil
}

func CreateVolumeOwner(s *server.Service, volumeName, ownerName string, ownerDurationMins int) (*CreateOwnerResponse, error) {
	os, err := s.OwnerStore()
	if err != nil {
		return nil, err
	}
	expiry, err := os.AcquireForVolume(volumeName, ownerName, ownerDurationMins)
	if err != nil {
		return nil, err
	}
	return &CreateOwnerResponse{
		Volume:    volumeName,
		Owner:     ownerName,
		ExpiresAt: expiry,
	}, nil
}

func DeleteVolumeOwners(s *server.Service, volumeName string) error {
	os, err := s.OwnerStore()
	if err != nil {
		return fmt.Errorf("initialize metadata store: %w", err)
	}
	return os.DeleteForVolume(volumeName)
}
