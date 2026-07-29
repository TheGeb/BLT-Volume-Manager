import { writable, derived, get } from 'svelte/store';
import type { Snapshot, VolumeOwnerInfo } from '../types';
import * as api from '../api';
import { showToast } from './toast';

export const volumes = writable<string[]>([]);
export const selectedVolume = writable('');
export const volumeFilter = writable('');
export const landingShown = writable(true);
export const volumesLoading = writable(false);
export const volumeOwnerInfo = writable<Record<string, VolumeOwnerInfo>>({});
export const deleteVolModal = writable(false);
export const deleteConfirmText = writable('');
export const deleteVolLoading = writable(false);

export const copyVolModal = writable(false);
export const renameVolModal = writable(false);
export const copyRenameSource = writable('');
export const copyRenameTarget = writable('');
export const copyRenameLoading = writable(false);
export const copyRenameError = writable('');
export const copySnapshots = writable<Snapshot[]>([]);
export const copySnapshotsLoading = writable(false);
export const copySnapshotMode = writable<'all' | 'specific'>('all');
export const copySelectedSnapshotIds = writable<string[]>([]);
export const copyRestorePointID = writable('');

export function filterVolumes(all: string[], filter: string): string[] {
  return all.filter(v => v.toLowerCase().includes(filter.toLowerCase()));
}

export const filteredVolumes = derived(
  [volumes, volumeFilter],
  ([$volumes, $volumeFilter]) => filterVolumes($volumes, $volumeFilter)
);

export async function loadVolumes() {
  volumesLoading.set(true);
  try {
    volumes.set(await api.fetchVolumes());
    void fetchAllVolumeOwnerInfo();
  } catch {
    showToast('Cannot reach server', true);
  } finally {
    volumesLoading.set(false);
  }
}

async function fetchAllVolumeOwnerInfo() {
  const vols = get(volumes);
  if (vols.length === 0) return;
  try {
    const allOwners = await api.fetchAllOwnerStatus();
    const info: Record<string, VolumeOwnerInfo> = {};
    for (const vol of vols) {
      const r = allOwners[vol];
      if (r) {
        info[vol] = {
          owner: r.owner,
          expiry: r.expiry ?? 0,
          status: r.owner ? 'owned' : 'unclaimed',
        };
      } else {
        info[vol] = { owner: '', expiry: 0, status: 'unclaimed' };
      }
    }
    volumeOwnerInfo.set(info);
  } catch {
    volumeOwnerInfo.set({});
  }
}

export function onFilterChange(f: string) { volumeFilter.set(f); }

export function openDeleteVolModal() {
  const vol = get(selectedVolume);
  if (!vol) return;
  deleteConfirmText.set('');
  deleteVolModal.set(true);
}

export function openCopyVolModal(vol: string) {
  copyRenameSource.set(vol);
  copyRenameTarget.set('');
  copyRenameError.set('');
  copySnapshotMode.set('all');
  copySelectedSnapshotIds.set([]);
  copyRestorePointID.set('');
  copySnapshots.set([]);
  copyVolModal.set(true);
  void loadCopySnapshots(vol);
}

async function loadCopySnapshots(vol: string) {
  copySnapshotsLoading.set(true);
  try {
    const result = await api.fetchSnapshots(vol);
    copySnapshots.set(result.snapshots);
    copyRestorePointID.set(result.restorePointID ?? '');
  } catch (e: unknown) {
    copySnapshots.set([]);
    copyRestorePointID.set('');
    showToast((e as Error).message, true);
  } finally {
    copySnapshotsLoading.set(false);
  }
}

export function openRenameVolModal(vol: string) {
  copyRenameSource.set(vol);
  copyRenameTarget.set('');
  copyRenameError.set('');
  renameVolModal.set(true);
}

export async function confirmCopyVolume() {
  const src = get(copyRenameSource);
  const target = get(copyRenameTarget);
  const mode = get(copySnapshotMode);
  const snapshotIds = get(copySelectedSnapshotIds);
  if (!src || !target) return;

  copyRenameLoading.set(true);
  copyRenameError.set('');
  try {
    if (mode === 'specific' && snapshotIds.length > 0) {
      await api.copyVolume(src, target, undefined, snapshotIds);
    } else {
      await api.copyVolume(src, target, true);
    }
    copyVolModal.set(false);
    await loadVolumes();
    showToast(`Volume "${src}" copied to "${target}"`);
  } catch (err: unknown) {
    copyRenameError.set(err instanceof Error ? err.message : 'Copy failed');
  } finally {
    copyRenameLoading.set(false);
  }
}

export async function confirmRenameVolume() {
  const src = get(copyRenameSource);
  const target = get(copyRenameTarget);
  if (!src || !target) return;

  copyRenameLoading.set(true);
  copyRenameError.set('');
  try {
    await api.renameVolume(src, target);
    renameVolModal.set(false);
    await loadVolumes();
    showToast(`Volume "${src}" renamed to "${target}"`);
  } catch (err: unknown) {
    copyRenameError.set(err instanceof Error ? err.message : 'Rename failed');
  } finally {
    copyRenameLoading.set(false);
  }
}