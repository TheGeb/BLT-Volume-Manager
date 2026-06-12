import { get } from 'svelte/store';
import { writable } from 'svelte/store';
import type { Snapshot } from '../types';
import { versionTag } from '../util';
import * as api from '../api';
import { showToast } from './toast';
import { selectedVolume, landingShown, loadVolumes, deleteConfirmText, deleteVolModal, deleteVolLoading } from './volumes';
import { activeTab, loadLockStatus, loadStats, stats as repoStats, doSwitchTab } from './repo';
import { snapshots, loadSnapshots, allSnapshots, currentSnapshot, viewerOpen, diffTargetId, sizes, deleteSnapModal, findSnapshot, findSnapshotByVersion } from './snapshots';

export const creatingTest = writable(false);
export const testStatus = writable('');

export async function loadAll(volume: string) {
  selectedVolume.set(volume);
  viewerOpen.set(false);
  currentSnapshot.set(null);
  sizes.set({});
  deleteVolModal.set(false);
  deleteSnapModal.set(false);
  testStatus.set('');
  diffTargetId.set('');
  landingShown.set(!volume);
  syncUrl();
  if (volume) {
    await loadSnapshots(volume);
  } else {
    await loadVolumes();
  }
}

export async function navigateTo(volume: string, params?: { tab?: string; tag?: string; diffTag?: string; snapshotId?: string; diffId?: string }) {
  selectedVolume.set(volume);
  sizes.set({});
  deleteVolModal.set(false);
  deleteSnapModal.set(false);
  testStatus.set('');
  landingShown.set(false);

  const tab = (params?.tab ?? 'snapshots') as 'snapshots' | 'repo';
  activeTab.set(tab);

  const hasSnapshot = !!(params?.tag ?? params?.snapshotId);

  if (tab === 'snapshots' && hasSnapshot) {
    viewerOpen.set(true);
    diffTargetId.set('');
  } else if (tab === 'snapshots') {
    viewerOpen.set(false);
    currentSnapshot.set(null);
    diffTargetId.set('');
  }

  if (tab === 'repo') {
    await Promise.all([loadLockStatus(), get(repoStats) ? Promise.resolve() : loadStats(volume)]);
  } else {
    await loadSnapshots(volume);

    if (params?.tag) {
      const snap = findSnapshotByVersion(get(snapshots), params.tag);
      if (snap) currentSnapshot.set(snap);
    } else if (params?.snapshotId) {
      const snap = findSnapshot(get(snapshots), params.snapshotId);
      if (snap) currentSnapshot.set(snap);
    }

    if (params?.diffTag) {
      const diffSnap = findSnapshotByVersion(get(snapshots), params.diffTag);
      if (diffSnap) diffTargetId.set(diffSnap.id);
    } else if (params?.diffId) {
      const diffSnap = findSnapshot(get(snapshots), params.diffId);
      if (diffSnap) diffTargetId.set(diffSnap.id);
    }
  }
}

export async function handleRefresh() {
  showToast('');
  try { await api.refreshStats(); } catch {}
  const vol = get(selectedVolume);
  if (vol) {
    const promises: Promise<unknown>[] = [loadSnapshots(vol)];
    if (get(activeTab) === 'repo') {
      promises.push(loadStats(vol));
    }
    await Promise.all(promises);
  }
  await loadVolumes();
  if (vol && get(activeTab) === 'repo') void loadLockStatus();
}

export function onSelectVolume(vol: string) {
  if (vol === get(selectedVolume)) {
    void loadAll('');
    return;
  }
  void loadAll(vol);
}

export function switchTab(tab: 'snapshots' | 'repo') {
  doSwitchTab(tab);
  syncUrl();
}

export async function confirmDeleteVolume() {
  const vol = get(selectedVolume);
  if (!vol || get(deleteConfirmText) !== vol) return;
  deleteVolLoading.set(true);
  try {
    await api.deleteVolume(vol);
    deleteVolModal.set(false);
    deleteVolLoading.set(false);
    showToast(`Volume ${vol} deleted`);
    selectedVolume.set('');
    landingShown.set(true);
    currentSnapshot.set(null);
    viewerOpen.set(false);
    sizes.set({});
    await Promise.all([
      api.refreshStats().catch(() => { /* intentionally ignored */ }),
      loadVolumes(),
    ]);
  } catch (e: unknown) {
    deleteVolLoading.set(false);
    showToast((e as Error).message, true);
  }
}

export async function handleCreateTestVolume(name: string) {
  creatingTest.set(true);
  testStatus.set('');
  try {
    await api.createTestVolume(name);
    testStatus.set('Updating volume list...');
    await api.refreshStats().catch(() => { /* intentionally ignored */ });
    await loadVolumes();
    await loadAll(name);
  } catch (e: unknown) { testStatus.set((e as Error).message); }
  finally { creatingTest.set(false); }
}

export function onOpenViewer(snapshot: Snapshot) {
  currentSnapshot.set(snapshot);
  viewerOpen.set(true);
  diffTargetId.set('');
  syncUrl();
}

export function onCloseViewer() {
  viewerOpen.set(false);
  currentSnapshot.set(null);
  diffTargetId.set('');
  syncUrl();
}

export function setDiffTarget(id: string) {
  if (!id) {
    diffTargetId.set('');
    syncUrl();
    return;
  }
  const snap = findSnapshot(get(allSnapshots), id);
  if (!snap) return;
  diffTargetId.set(snap.id);
  syncUrl();
}

function buildUrl(): string {
  const vol = get(selectedVolume);
  if (!vol) return '/ui/volumes';
  const encodedVol = vol.split('/').map(encodeURIComponent).join('/');
  const tab = get(activeTab) === 'repo' ? 'repo' : 'snapshots';
  let url = `/ui/${tab}/${encodedVol}`;
  if (get(viewerOpen) && get(currentSnapshot)) {
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    const snap = get(currentSnapshot)!;
    const p = new URLSearchParams();
    const vtag = versionTag(snap.tags);
    if (vtag) {
      p.set('tag', vtag);
    } else {
      p.set('snapshot', snap.short_id);
    }
    const dtId = get(diffTargetId);
    if (dtId) {
      const dtSnap = findSnapshot(get(allSnapshots), dtId);
      if (dtSnap) {
        const dtVtag = versionTag(dtSnap.tags);
        if (dtVtag) {
          p.set('diffTag', dtVtag);
        } else {
          p.set('diff', dtSnap.short_id);
        }
      }
    }
    if (p.size > 0) url += `?${p.toString()}`;
  }
  return url;
}

export function syncUrl() {
  const url = buildUrl();
  window.history.replaceState({}, '', url);
}