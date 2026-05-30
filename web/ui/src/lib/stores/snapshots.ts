import { writable, derived, get } from 'svelte/store';
import type { Snapshot } from '../types';
import * as api from '../api';
import { setBanner } from './banner';
import { selectedVolume } from './volumes';
import { formatBytes } from '../util';

export const snapshots = writable<Snapshot[]>([]);
export const query = writable('');
export const sortNewestFirst = writable(true);
export const typeFilter = writable('all');
export const hostFilter = writable('');
export const snapsLoading = writable(false);
export const currentSnapshot = writable<Snapshot | null>(null);
export const allSnapshots = derived(snapshots, $s => $s);
export const viewerOpen = writable(false);
export const deleteSnapModal = writable(false);
export const deletingSnap = writable<Snapshot | null>(null);
export const snapDeleteInput = writable('');
export const diffTargetId = writable('');
export const diffTargetFallbackHash = writable('');
export const sizes = writable<Record<string, string>>({});
export const restorePointLoading = writable<Record<string, boolean>>({});
export const restorePointID = writable('');
export const sizeLoading = writable<Record<string, boolean>>({});

export const filteredSnapshots = derived(
  [snapshots, typeFilter, hostFilter, query],
  ([$snapshots, $typeFilter, $hostFilter, $query]) =>
    $snapshots.filter(sn => {
      if ($typeFilter === 'hot' && !sn.tags.includes('hot')) return false;
      if ($typeFilter === 'cold' && !sn.tags.includes('cold')) return false;
      if ($hostFilter && sn.hostname !== $hostFilter) return false;
      if (!$query) return true;
      const q = $query.toLowerCase();
      return sn.short_id.toLowerCase().includes(q) ||
        sn.tags.some(t => t.toLowerCase().includes(q)) ||
        sn.hostname?.toLowerCase().includes(q);
    })
);

export const sortedSnapshots = derived(
  [filteredSnapshots, sortNewestFirst],
  ([$filtered, $newestFirst]) =>
    [...$filtered].sort((a, b) => {
      const da = new Date(a.time).getTime();
      const db = new Date(b.time).getTime();
      return $newestFirst ? db - da : da - db;
    })
);

export const hosts = derived(snapshots, $s =>
  [...new Set($s.map(sn => sn.hostname).filter(Boolean))].sort()
);

export async function sha256Short(message: string, length: number): Promise<string> {
  const msgBuffer = new TextEncoder().encode(message);
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const fullHash = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
  return fullHash.substring(0, length);
}

export function snapshotHashInput(snap: Snapshot): string {
  const paths = snap.paths ? [...snap.paths].sort().join(',') : '';
  return snap.hostname + snap.time + (snap.tree || '') + paths;
}

export async function getSnapshotHash(snap: Snapshot): Promise<string> {
  if (snap.fallbackHash) return snap.fallbackHash;
  const msg = snapshotHashInput(snap);
  const hash = await sha256Short(msg, snap.short_id.length);
  snap.fallbackHash = hash;
  return hash;
}

async function reconcileViewerSnapshots() {
  if (!get(viewerOpen) || !get(currentSnapshot)) return;

  const $snapshots = get(snapshots);
  const $current = get(currentSnapshot)!;
  const $diffId = get(diffTargetId);

  let found = $snapshots.find(s => s.id === $current.id || s.short_id === $current.short_id);
  if (!found && $current.fallbackHash) {
    const hash = $current.fallbackHash;
    for (const s of $snapshots) {
      const msg = snapshotHashInput(s);
      const computed = await sha256Short(msg, s.short_id.length);
      if (computed === hash) {
        found = s;
        s.fallbackHash = hash;
        break;
      }
    }
  }

  if (found) {
    currentSnapshot.set(found);
  } else {
    currentSnapshot.set(null);
    viewerOpen.set(false);
    diffTargetId.set('');
    return;
  }

  if ($diffId) {
    let dtSnap = $snapshots.find(s => s.id === $diffId || s.short_id === $diffId);
    if (!dtSnap) {
      const $hash = get(diffTargetFallbackHash);
      if ($hash) {
        for (const s of $snapshots) {
          const msg = snapshotHashInput(s);
          const computed = await sha256Short(msg, s.short_id.length);
          if (computed === $hash) {
            dtSnap = s;
            break;
          }
        }
      }
    }
    if (dtSnap) {
      diffTargetId.set(dtSnap.id);
    } else {
      diffTargetId.set('');
      diffTargetFallbackHash.set('');
    }
  }
}

export async function loadSnapshots(volume: string) {
  snapsLoading.set(true);
  try {
    const result = await api.fetchSnapshots(volume);
    snapshots.set(result.snapshots);
    restorePointID.set(result.restorePointID || '');
  } catch {
    snapshots.set([]);
    restorePointID.set('');
    setBanner('Failed to load snapshots', true);
  } finally {
    snapsLoading.set(false);
    reconcileViewerSnapshots();
  }
}

export function onToggleSort() {
  sortNewestFirst.update(v => !v);
}

export function onSearch(q: string) { query.set(q); }
export function onTypeFilter(t: string) { typeFilter.set(t); }
export function onHostFilter(h: string) { hostFilter.set(h); }



export async function onAddTag(id: string, tag: string, vol: string) {
  restorePointLoading.update(r => ({ ...r, [id]: true }));
  try {
    const result = await api.addTag(id, tag, vol);
    snapshots.set(result.snapshots);
    restorePointID.set(result.restorePointID || '');
    reconcileViewerSnapshots();
  } catch (e) {
    setBanner(`Failed to add tag: ${e}`, true);
    await loadSnapshots(vol);
  } finally {
    restorePointLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
  }
}

export async function onRemoveTag(id: string, tag: string, vol: string) {
  restorePointLoading.update(r => ({ ...r, [id]: true }));
  try {
    const result = await api.removeTag(id, tag, vol);
    snapshots.set(result.snapshots);
    restorePointID.set(result.restorePointID || '');
    reconcileViewerSnapshots();
  } catch (e) {
    setBanner(`Failed to remove tag: ${e}`, true);
    await loadSnapshots(vol);
  } finally {
    restorePointLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
  }
}

export async function onDeleteSnapshot(sn: Snapshot) {
  deletingSnap.set(sn);
  snapDeleteInput.set('');
  deleteSnapModal.set(true);
}

export async function confirmDeleteSnapshot() {
  const sn = get(deletingSnap);
  const vol = get(selectedVolume);
  if (!sn) return;
  try {
    await api.deleteSnapshot(sn.id, vol);
    deleteSnapModal.set(false);
    deletingSnap.set(null);
    setBanner('Snapshot deleted');
    await loadSnapshots(vol);
  } catch (e: unknown) { setBanner((e as Error).message, true); }
}

export async function handleSizeLoaded(id: string) {
  sizeLoading.update(s => ({ ...s, [id]: true }));
  const vol = get(selectedVolume);
  try {
    const data = await api.fetchSnapshotSizes(vol, [id]);
    if (data[id] != null) {
      sizes.update(s => ({ ...s, [id]: formatBytes(data[id]) }));
    } else {
      sizes.update(s => ({ ...s, [id]: 'err' }));
    }
  } catch {
    sizes.update(s => ({ ...s, [id]: 'err' }));
  } finally {
    sizeLoading.update(s => { const n = { ...s }; delete n[id]; return n; });
  }
}
