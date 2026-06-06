import { writable, derived, get } from 'svelte/store';
import type { Snapshot } from '../types';
import type { SnapshotListParams } from '../api';
import * as api from '../api';
import { showToast } from './toast';
import { selectedVolume } from './volumes';
import { formatBytes } from '../util';

export const snapshots = writable<Snapshot[]>([]);
export const query = writable('');
export const sortNewestFirst = writable(true);
export const typeFilter = writable('all');
export const hostFilter = writable('');
export const timeFrom = writable<number | undefined>(undefined);
export const timeTo = writable<number | undefined>(undefined);
export const timeOfDayFrom = writable<number | undefined>(undefined);
export const timeOfDayTo = writable<number | undefined>(undefined);
export const searchLatest = writable(25);
export const snapsLoading = writable(false);
export const currentSnapshot = writable<Snapshot | null>(null);
export const allSnapshots = derived(snapshots, $s => $s);
export const viewerOpen = writable(false);
export const deleteSnapModal = writable(false);
export const snapDeleteInput = writable('');
export const diffTargetId = writable('');
export const diffTargetFallbackHash = writable('');
export const sizes = writable<Record<string, string>>({});
export const restorePointLoading = writable<Record<string, boolean>>({});
export const restorePointID = writable('');
export const sizeLoading = writable<Record<string, boolean>>({});
export const displayLimit = writable(50);
export const allSnapshotsLoaded = writable(false);
export const allHosts = writable<string[]>([]);

export async function loadHosts(volume: string) {
	try {
		const hosts = await api.fetchSnapshotHosts(volume);
		allHosts.set(hosts);
	} catch {
		// leave current hosts unchanged on failure
	}
}

export function filterSnapshots(
  snapshots: Snapshot[],
  query: string,
  timeFrom?: number,
  timeTo?: number,
  timeOfDayFrom?: number,
  timeOfDayTo?: number,
): Snapshot[] {
  return snapshots.filter(sn => {
    const snTime = new Date(sn.time);
    if (timeFrom !== undefined && snTime.getTime() < timeFrom) return false;
    if (timeTo !== undefined && snTime.getTime() > timeTo) return false;
    if (timeOfDayFrom !== undefined) {
      const snSeconds = snTime.getHours() * 3600 + snTime.getMinutes() * 60 + snTime.getSeconds();
      const to = timeOfDayTo ?? 86400;
      if (snSeconds < timeOfDayFrom || snSeconds > to) return false;
    }
    if (!query) return true;
    const q = query.toLowerCase();
    return sn.short_id.toLowerCase().includes(q) ||
      sn.tags.some(t => t.toLowerCase().includes(q)) ||
      (typeof sn.hostname === 'string' && sn.hostname.toLowerCase().includes(q));
  });
}

export const filteredSnapshots = derived(
  [snapshots, query, timeFrom, timeTo, timeOfDayFrom, timeOfDayTo],
  ([$snapshots, $query, $timeFrom, $timeTo, $todFrom, $todTo]) =>
    filterSnapshots($snapshots, $query, $timeFrom, $timeTo, $todFrom, $todTo)
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

export const displayedSnapshots = derived(
  [sortedSnapshots, displayLimit],
  ([$sorted, $limit]) => $limit > 0 ? $sorted.slice(0, $limit) : $sorted
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

export async function loadSnapshots(volume: string, params?: SnapshotListParams) {
  snapsLoading.set(true);
  try {
    const p: SnapshotListParams = { latest: get(searchLatest), ...params };
    const result = await api.fetchSnapshots(volume, p);
    snapshots.set(result.snapshots);
    restorePointID.set(result.restorePointID ?? '');
    allSnapshotsLoaded.set(false);
    displayLimit.set(get(searchLatest));
	} catch {
		snapshots.set([]);
		restorePointID.set('');
		showToast('Failed to load snapshots', true);
	} finally {
		snapsLoading.set(false);
		reconcileViewerSnapshots();
	}
}

export async function loadAllSnapshots() {
	const vol = get(selectedVolume);
	if (!vol) return;
	snapsLoading.set(true);
  try {
    const params = { ...buildSnapshotParams(), latest: 0 };
    const result = await api.fetchSnapshots(vol, params);
		snapshots.set(result.snapshots);
		restorePointID.set(result.restorePointID ?? '');
		allSnapshotsLoaded.set(true);
		displayLimit.set(0);
	} catch {
		showToast('Failed to load all snapshots', true);
	} finally {
		snapsLoading.set(false);
		reconcileViewerSnapshots();
	}
}

function buildSnapshotParams(): SnapshotListParams {
  const hf = get(hostFilter);
  const tf = get(typeFilter);
  const latest = get(searchLatest);
  const params: SnapshotListParams = { latest };
  if (hf) params.hosts = [hf];
  if (tf !== 'all') params.tags = [tf];
  return params;
}

export async function reloadWithFilters(overrides?: Partial<SnapshotListParams>) {
  const vol = get(selectedVolume);
  if (!vol) return;
  const params = { ...buildSnapshotParams(), ...overrides };
  await loadSnapshots(vol, params);
}

export function onToggleSort() {
  sortNewestFirst.update(v => !v);
}

export function onSearch(q: string) { query.set(q); }
export function onTypeFilter(t: string) { typeFilter.set(t); }
export function onHostFilter(h: string) { hostFilter.set(h); }
export function onTimeFilter(from?: number, to?: number) {
  timeFrom.set(from);
  timeTo.set(to);
}

export function onTimeOfDayFilter(from?: number, to?: number) {
  timeOfDayFrom.set(from);
  timeOfDayTo.set(to);
}



export async function onAddTag(id: string, tag: string, vol: string) {
  restorePointLoading.update(r => ({ ...r, [id]: true }));
  try {
    const result = await api.addTag(id, tag, vol);
    snapshots.set(result.snapshots);
    restorePointID.set(result.restorePointID ?? '');
    reconcileViewerSnapshots();
  } catch (e) {
    showToast(`Failed to add tag: ${String(e)}`, true);
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
    showToast(`Failed to remove tag: ${String(e)}`, true);
    await loadSnapshots(vol);
  } finally {
    // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
    restorePointLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
  }
}

export const selectedForDeletion = writable<Set<string>>(new Set());

export const selectedDeletionCount = derived(selectedForDeletion, $s => $s.size);

export function toggleForDeletion(sn: Snapshot) {
  selectedForDeletion.update(s => {
    const next = new Set(s);
    if (next.has(sn.id)) {
      next.delete(sn.id);
    } else {
      next.add(sn.id);
    }
    return next;
  });
}

export function openBulkDeleteModal() {
  snapDeleteInput.set('');
  deleteSnapModal.set(true);
}

export async function confirmDeleteSnapshot() {
  const vol = get(selectedVolume);
  const ids = [...get(selectedForDeletion)];
  if (ids.length === 0) return;
  deleteSnapModal.set(false);
  try {
    const result = await api.deleteSnapshots(vol, ids);
    selectedForDeletion.set(new Set());
    if (result.failed > 0) {
      showToast(`Deleted ${String(result.deleted)}, failed ${String(result.failed)}`, true);
    } else {
      showToast(`Deleted ${String(result.deleted)} snapshot${result.deleted !== 1 ? 's' : ''}`);
    }
    await loadSnapshots(vol);
  } catch (e: unknown) {
    showToast((e as Error).message, true);
  }
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