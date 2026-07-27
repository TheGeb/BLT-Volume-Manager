package owner

import (
	"context"

	"github.com/TheGeb/BLT-Volume-Manager/internal/metadata/store"
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

func VolumeOwner(ctx context.Context, os *store.OwnerStore, volumeName string) (*OwnerInfo, error) {
	vo, err := os.FindForVolume(ctx, volumeName)
	if err != nil {
		return nil, err
	}
	return &OwnerInfo{Volume: vo.Volume, Owner: vo.Owner, Expiry: vo.Expiry}, nil
}

func IsVolumeOwned(ctx context.Context, os *store.OwnerStore, volumeName string) (bool, string, error) {
	status, err := VolumeOwner(ctx, os, volumeName)
	if err != nil {
		return false, "", err
	}
	if status.Owner != "" {
		return true, status.Owner, nil
	}
	return false, "", nil
}

func CreateVolumeOwner(ctx context.Context, os *store.OwnerStore, volumeName, ownerName string, ownerDurationMins int) (*CreateOwnerResponse, error) {
	expiry, err := os.AcquireForVolume(ctx, volumeName, ownerName, ownerDurationMins)
	if err != nil {
		return nil, err
	}
	return &CreateOwnerResponse{
		Volume:    volumeName,
		Owner:     ownerName,
		ExpiresAt: expiry,
	}, nil
}

func DeleteVolumeOwners(ctx context.Context, os *store.OwnerStore, volumeName string) error {
	return os.DeleteForVolume(ctx, volumeName)
}
