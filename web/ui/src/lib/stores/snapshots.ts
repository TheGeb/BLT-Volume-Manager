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

export function filterSnapshots(snapshots: Snapshot[], typeFilter: string, hostFilter: string, query: string): Snapshot[] {
  return snapshots.filter(sn => {
    if (typeFilter === 'hot' && !sn.tags.includes('hot')) return false;
    if (typeFilter === 'cold' && !sn.tags.includes('cold')) return false;
    if (hostFilter && sn.hostname !== hostFilter) return false;
    if (!query) return true;
    const q = query.toLowerCase();
    return sn.short_id.toLowerCase().includes(q) ||
      sn.tags.some(t => t.toLowerCase().includes(q)) ||
      (typeof sn.hostname === 'string' && sn.hostname.toLowerCase().includes(q));
  });
}

export const filteredSnapshots = derived(
  [snapshots, typeFilter, hostFilter, query],
  ([$snapshots, $typeFilter, $hostFilter, $query]) =>
    filterSnapshots($snapshots, $typeFilter, $hostFilter, $query)
);

export function sortSnapshots(snapshots: Snapshot[], newestFirst: boolean): Snapshot[] {
  return [...snapshots].sort((a, b) => {
    const da = new Date(a.time).getTime();
    const db = new Date(b.time).getTime();
    return newestFirst ? db - da : da - db;
  });
}

export const sortedSnapshots = derived(
  [filteredSnapshots, sortNewestFirst],
  ([$filtered, $newestFirst]) => sortSnapshots($filtered, $newestFirst)
);

export function extractHosts(snapshots: Snapshot[]): string[] {
  return [...new Set(snapshots.map(sn => sn.hostname).filter(Boolean))].sort();
}

export const hosts = derived(snapshots, $s => extractHosts($s));

function reconcileViewerSnapshots() {
  if (!get(viewerOpen) || !get(currentSnapshot)) return;

  const $snapshots = get(snapshots);
  // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
  const $current = get(currentSnapshot)!;
  const $diffId = get(diffTargetId);

  let found = $snapshots.find(s => s.id === $current.id || s.short_id === $current.short_id);
  if (!found && $current.fallbackHash) {
    found = $snapshots.find(s => s.fallbackHash === $current.fallbackHash);
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
        dtSnap = $snapshots.find(s => s.fallbackHash === $hash);
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
    restorePointID.set(result.restorePointID ?? '');
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
    restorePointID.set(result.restorePointID ?? '');
    reconcileViewerSnapshots();
  } catch (e) {
    setBanner(`Failed to add tag: ${String(e)}`, true);
    await loadSnapshots(vol);
  } finally {
    // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
    restorePointLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
  }
}

export async function onRemoveTag(id: string, tag: string, vol: string) {
  restorePointLoading.update(r => ({ ...r, [id]: true }));
  try {
    const result = await api.removeTag(id, tag, vol);
    snapshots.set(result.snapshots);
    restorePointID.set(result.restorePointID ?? '');
    reconcileViewerSnapshots();
  } catch (e) {
    setBanner(`Failed to remove tag: ${String(e)}`, true);
    await loadSnapshots(vol);
  } finally {
    // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
    restorePointLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
  }
}

export function onDeleteSnapshot(sn: Snapshot) {
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
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
      sizes.update(s => ({ ...s, [id]: formatBytes(data[id]!) }));
    } else {
      sizes.update(s => ({ ...s, [id]: 'err' }));
    }
  } catch {
    sizes.update(s => ({ ...s, [id]: 'err' }));
  } finally {
    // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
    sizeLoading.update(s => { const n = { ...s }; delete n[id]; return n; });
  }
}