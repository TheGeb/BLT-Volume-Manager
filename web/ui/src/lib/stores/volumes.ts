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

export const filteredVolumes = derived(
  [volumes, volumeFilter],
  ([$volumes, $volumeFilter]) =>
    $volumes.filter(v => v.toLowerCase().includes($volumeFilter.toLowerCase()))
);

export async function loadVolumes() {
  volumesLoading.set(true);
  try {
    volumes.set(await api.fetchVolumes());
    fetchAllVolumeLockInfo();
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
      info[vols[i]] = {
        locked: r.locked,
        owner: r.owner || '',
        expiresIn: r.expires_in || 0,
        status: r.locked ? 'locked' : 'unlocked',
      };
    } else {
      info[vols[i]] = { locked: false, owner: '', expiresIn: 0, status: 'unlocked' };
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
