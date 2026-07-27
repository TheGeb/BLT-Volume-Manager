package volume

import (
	"context"
	"fmt"
	"strings"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/owner"
	"github.com/TheGeb/BLT-Volume-Manager/internal/web/server"
)

func CleanupVolumeData(ctx context.Context, s *server.BLTService, volumeName string) {
	if volumeName == "" {
		return
	}
	if err := s.DeleteVolumeData(ctx, volumeName); err != nil {
		log.Error("cleanup_volume_data_failed", err)
	}
}

func validVolumeName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\") && !strings.Contains(name, "..")
}

func initTargetRepo(ctx context.Context, s *server.BLTService, target string) (*restic.Manager, error) {
	if !validVolumeName(target) {
		return nil, fmt.Errorf("invalid target volume name")
	}
	for _, v := range s.VolumeNames(ctx) {
		if v == target {
			return nil, fmt.Errorf("target volume %q already exists", target)
		}
	}
	tm := s.ResticManager(target)
	if err := tm.Init(ctx); err != nil {
		return nil, fmt.Errorf("init target repo: %w", err)
	}
	return tm, nil
}

func registerVolume(ctx context.Context, s *server.BLTService, target string) error {
	return s.RegisterVolume(ctx, target)
}

type CopyVolumeResult struct {
	SourceOwned     bool
	OwnerName       string
	PreserveHistory bool
}

func CopyVolumeData(ctx context.Context, s *server.BLTService, source, target string, preserveHistory *bool, snapshotIDs []string) (*CopyVolumeResult, error) {
	tm, err := initTargetRepo(ctx, s, target)
	if err != nil {
		return nil, err
	}

	owned, ownerName, err := owner.IsVolumeOwned(ctx, s.OwnerStore(), source)
	if err != nil {
		return nil, fmt.Errorf("check owner: %w", err)
	}

	sourceManager := s.ResticManager(source)
	preserve := true
	if preserveHistory != nil {
		preserve = *preserveHistory
	}
	switch {
	case len(snapshotIDs) > 0:
		if err := sourceManager.CopyTo(ctx, tm.Repo(), snapshotIDs...); err != nil {
			return nil, fmt.Errorf("copy snapshots: %w", err)
		}
	case preserve:
		if err := sourceManager.CopyTo(ctx, tm.Repo()); err != nil {
			return nil, fmt.Errorf("copy snapshots: %w", err)
		}
	default:
		return nil, fmt.Errorf("no snapshots to copy")
	}

	if err := registerVolume(ctx, s, target); err != nil {
		return nil, fmt.Errorf("register volume: %w", err)
	}

	return &CopyVolumeResult{
		SourceOwned:     owned,
		OwnerName:       ownerName,
		PreserveHistory: preserve,
	}, nil
}

func RenameVolumeData(ctx context.Context, s *server.BLTService, source, target string) error {
	tm, err := initTargetRepo(ctx, s, target)
	if err != nil {
		return err
	}

	owned, ownerName, err := owner.IsVolumeOwned(ctx, s.OwnerStore(), source)
	if err != nil {
		return fmt.Errorf("check owner: %w", err)
	}
	if owned {
		return fmt.Errorf("cannot rename owned volume %q (owned by %q)", source, ownerName)
	}

	sourceManager := s.ResticManager(source)
	if err := sourceManager.CopyTo(ctx, tm.Repo()); err != nil {
		return fmt.Errorf("copy snapshots: %w", err)
	}

	snapshots, err := sourceManager.ListSnapshots(ctx)
	if err == nil {
		ids := make([]string, len(snapshots))
		for i, snap := range snapshots {
			ids[i] = snap.ID
		}
		if err := sourceManager.ForgetSnapshots(ctx, ids...); err != nil {
			log.Error("forget_snapshots_failed", err)
		}
	}

	if err := registerVolume(ctx, s, target); err != nil {
		return fmt.Errorf("register volume: %w", err)
	}

	CleanupVolumeData(ctx, s, source)
	return nil
}
