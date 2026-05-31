import { writable, derived, get } from 'svelte/store';
import type { VolumeLockInfo } from '../types';
import * as api from '../api';
import { setBanner } from './banner';

export const volumes = writable<string[]>([]);
export const selectedVolume = writable('');
export const volumeFilter = writable('');
export const hostname = writable('');
export const landingShown = writable(true);
export const volumesLoading = writable(false);
export const volumeLockInfo = writable<Record<string, VolumeLockInfo>>({});
export const deleteVolModal = writable(false);
export const deleteConfirmText = writable('');
export const deleteVolLoading = writable(false);

export const copyVolModal = writable(false);
export const renameVolModal = writable(false);
export const copyRenameSource = writable('');
export const copyRenameTarget = writable('');
export const copyRenameLoading = writable(false);
export const copyRenameError = writable('');
export const copyPreserveHistory = writable(true);

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
    void fetchAllVolumeLockInfo();
  } catch {
    setBanner('Cannot reach server', true);
  } finally {
    volumesLoading.set(false);
  }
}

export async function fetchAllVolumeLockInfo() {
  const vols = get(volumes);
  if (vols.length === 0) return;
  const results = await Promise.all(
    vols.map(name => api.fetchLockStatus(name).catch(() => null))
  );
  const info: Record<string, VolumeLockInfo> = {};
  for (let i = 0; i < vols.length; i++) {
    const r = results[i];
    if (r) {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
      info[vols[i]!] = {
        locked: r.locked,
        owner: r.owner ?? '',
        expiresIn: r.expires_in ?? 0,
        status: r.locked ? 'locked' : 'unlocked',
      };
    } else {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
      info[vols[i]!] = { locked: false, owner: '', expiresIn: 0, status: 'unlocked' };
    }
  }
  volumeLockInfo.set(info);
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
  copyPreserveHistory.set(true);
  copyVolModal.set(true);
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
  const preserveHistory = get(copyPreserveHistory);
  if (!src || !target) return;

  copyRenameLoading.set(true);
  copyRenameError.set('');
  try {
    await api.copyVolume(src, target, preserveHistory);
    copyVolModal.set(false);
    await loadVolumes();
    setBanner(`Volume "${src}" copied to "${target}"`);
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
    setBanner(`Volume "${src}" renamed to "${target}"`);
  } catch (err: unknown) {
    copyRenameError.set(err instanceof Error ? err.message : 'Rename failed');
  } finally {
    copyRenameLoading.set(false);
  }
}