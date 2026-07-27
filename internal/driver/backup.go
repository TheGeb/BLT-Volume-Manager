package driver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/TheGeb/BLT-Volume-Manager/internal/app/log"
	snapshot "github.com/TheGeb/BLT-Volume-Manager/internal/driver/fs_snapshot"
	"github.com/TheGeb/BLT-Volume-Manager/internal/restic"
)

const (
	hotBackupInterval   = 15 * time.Minute
	orphanCheckInterval = 30 * time.Minute
	orphanRetryMinAge   = 10 * time.Minute
)

func (d *Driver) startHotSchedule(ctx context.Context, name, volPath string) {
	rm := d.ResticManager(name)
	hotTicker := time.NewTicker(hotBackupInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				hotTicker.Stop()
				return
			case <-hotTicker.C:
				log.Infof("hot_backup_start", "volume=%s", name)
				vt := d.nextVersionTags(ctx, name, false)
				if err := rm.Backup(ctx, volPath, restic.WithTags(restic.BackupTagHot, vt...)...); err != nil {
					log.Errorf("hot_backup_failed", err, "volume=%s", name)
				}
			}
		}
	}()
}

func (d *Driver) coldBackup(ctx context.Context, name, volPath, fsType string, rm *restic.Manager) error {
	versionTags := d.nextVersionTags(ctx, name, false)

	if _, err := os.Stat(volPath); os.IsNotExist(err) {
		log.Warnf("cold_backup_skipped", "volume=%s path=%s does not exist", name, volPath)
		return nil
	}

	if fsType == "" {
		return rm.Backup(ctx, volPath, restic.WithTags(restic.BackupTagCold, versionTags...)...)
	}

	snapDir := filepath.Join(d.volumePath, SnapshotsDir)
	info, err := snapshot.Create(volPath, snapDir, name)
	if err != nil {
		log.Errorf("snapshot_create_failed", err, "volume=%s falling back to direct backup", name)
		return rm.Backup(ctx, volPath, restic.WithTags(restic.BackupTagCold, versionTags...)...)
	}

	snapTime := time.Now()
	if fi, err := os.Stat(info.AccessPath); err == nil {
		snapTime = fi.ModTime()
	}

	if err := rm.BackupAt(ctx, info.AccessPath, restic.WithTags(restic.BackupTagCold, versionTags...), snapTime); err != nil {
		return fmt.Errorf("cold backup: %w", err)
	}
	if err := snapshot.Remove(info); err != nil {
		log.Errorf("remove_snapshot_failed", err, "volume=%s fs=%s", name, fsType)
	}
	return nil
}

func (d *Driver) monitorOrphanedSnapshots(ctx context.Context) {
	ticker := time.NewTicker(orphanCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.retryOrphanedSnapshots()
		}
	}
}

func (d *Driver) retryOrphanedSnapshots() {
	snapDir := filepath.Join(d.volumePath, SnapshotsDir)
	snaps, err := snapshot.ListOrphaned(snapDir)
	if err != nil {
		log.Errorf("list_orphaned_snapshots_failed", err, "error=%v", err)
		return
	}
	for _, info := range snaps {
		fi, err := os.Stat(info.AccessPath)
		if err != nil {
			continue
		}
		if time.Since(fi.ModTime()) < orphanRetryMinAge {
			continue
		}
		if err := snapshot.ResolveType(info); err != nil {
			log.Errorf("resolve_snapshot_type_failed", err, "path=%s", info.AccessPath)
			continue
		}
		log.Infof("retry_orphaned_cold_backup", "volume=%s path=%s", info.VolName, info.AccessPath)
		rm := d.ResticManager(info.VolName)
		versionTags := d.nextVersionTags(context.Background(), info.VolName, false)
		if err := rm.BackupAt(context.Background(), info.AccessPath, restic.WithTags(restic.BackupTagCold, versionTags...), fi.ModTime()); err != nil {
			log.Errorf("orphaned_cold_backup_failed", err, "volume=%s", info.VolName)
			continue
		}
		if err := snapshot.Remove(info); err != nil {
			log.Errorf("cleanup_snapshot_failed", err, "path=%s", info.AccessPath)
		}
	}
}
