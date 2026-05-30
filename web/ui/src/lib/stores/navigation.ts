import { get } from 'svelte/store';
import { writable } from 'svelte/store';
import type { Snapshot } from '../types';
import * as api from '../api';
import { setBanner } from './banner';
import { selectedVolume, landingShown, loadVolumes, deleteConfirmText, deleteVolModal, deleteVolLoading } from './volumes';
import { activeTab, loadLockStatus, loadStats, doSwitchTab } from './repo';
import { snapshots, loadSnapshots, allSnapshots, currentSnapshot, viewerOpen, diffTargetId, diffTargetFallbackHash, snapshotHashInput, sha256Short, sizes, deleteSnapModal } from './snapshots';

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
  diffTargetFallbackHash.set('');
  landingShown.set(!volume);
  if (volume) {
    await Promise.all([
      loadSnapshots(volume),
      loadLockStatus(),
      loadStats(volume),
    ]);
  } else {
    await loadVolumes();
  }
  syncUrl();
}

export async function navigateTo(volume: string, opts?: { tab?: string; snapshotId?: string; diffId?: string; fallbackHash?: string; diffFallbackHash?: string }) {
  selectedVolume.set(volume);
  sizes.set({});
  deleteVolModal.set(false);
  deleteSnapModal.set(false);
  testStatus.set('');
  landingShown.set(false);

  const tab = (opts?.tab ?? 'snapshots') as 'snapshots' | 'repo';
  activeTab.set(tab);

  if (tab === 'snapshots' && opts?.snapshotId) {
    viewerOpen.set(true);
    diffTargetId.set(opts.diffId || '');
    if (opts.diffFallbackHash) {
      diffTargetFallbackHash.set(opts.diffFallbackHash);
    } else if (!opts.diffId) {
      diffTargetFallbackHash.set('');
    }
  } else {
    viewerOpen.set(false);
    currentSnapshot.set(null);
    diffTargetId.set('');
    diffTargetFallbackHash.set('');
  }

  if (tab === 'repo') {
    await Promise.all([loadLockStatus(), loadStats(volume)]);
  } else {
    await loadSnapshots(volume);
    await loadLockStatus();
    await loadStats(volume);

    if (opts?.snapshotId) {
      let snap = get(snapshots).find(s => s.id === opts.snapshotId || s.short_id === opts.snapshotId);
      if (!snap && opts?.fallbackHash) {
        for (const s of get(snapshots)) {
          const msg = snapshotHashInput(s);
          const hash = await sha256Short(msg, s.short_id.length);
          if (hash === opts.fallbackHash) {
            snap = s;
            break;
          }
        }
      }
      if (snap) {
        snap.fallbackHash = opts.fallbackHash;
        currentSnapshot.set(snap);
      }
      if (opts?.diffId && opts?.diffFallbackHash) {
        let diffSnap = get(snapshots).find(s => s.id === opts.diffId || s.short_id === opts.diffId);
        if (!diffSnap) {
          for (const s of get(snapshots)) {
            const msg = snapshotHashInput(s);
            const hash = await sha256Short(msg, s.short_id.length);
            if (hash === opts.diffFallbackHash) {
              diffSnap = s;
              break;
            }
          }
        }
        if (diffSnap) {
          diffSnap.fallbackHash = opts.diffFallbackHash;
          diffTargetFallbackHash.set(opts.diffFallbackHash);
          diffTargetId.set(diffSnap.id);
        }
      }
    }
  }

  syncUrl();
}

export async function handleRefresh() {
  setBanner('');
  try { await api.refreshStats(); } catch {}
  const vol = get(selectedVolume);
  if (vol) {
    await Promise.all([
      loadSnapshots(vol),
      loadStats(vol),
    ]);
  }
  await loadVolumes();
  if (vol) loadLockStatus();
}

export function onSelectVolume(vol: string) {
  if (vol === get(selectedVolume)) {
    loadAll('');
    return;
  }
  loadAll(vol);
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
    setBanner(`Volume ${vol} deleted`);
    selectedVolume.set('');
    landingShown.set(true);
    currentSnapshot.set(null);
    viewerOpen.set(false);
    sizes.set({});
    await Promise.all([
      api.refreshStats().catch(() => {}),
      loadVolumes(),
    ]);
  } catch (e: unknown) {
    deleteVolLoading.set(false);
    setBanner((e as Error).message, true);
  }
}

export async function handleCreateTestVolume(name: string) {
  creatingTest.set(true);
  testStatus.set('');
  try {
    await api.createTestVolume(name);
    testStatus.set('Updating volume list...');
    await api.refreshStats().catch(() => {});
    await loadVolumes();
    await loadAll(name);
  } catch (e: unknown) { testStatus.set((e as Error).message); }
  finally { creatingTest.set(false); }
}

export async function onOpenViewer(snapshot: Snapshot) {
  const msg = snapshotHashInput(snapshot);
  const hash = await sha256Short(msg, snapshot.short_id.length);
  snapshot.fallbackHash = hash;

  currentSnapshot.set(snapshot);
  viewerOpen.set(true);
  diffTargetId.set('');
  syncUrl();
}

export function onCloseViewer() {
  viewerOpen.set(false);
  currentSnapshot.set(null);
  diffTargetId.set('');
  diffTargetFallbackHash.set('');
  syncUrl();
}

export async function setDiffTarget(id: string) {
  const snap = get(allSnapshots).find(s => s.id === id || s.short_id === id);
  if (!snap) return;
  const msg = snapshotHashInput(snap);
  const hash = await sha256Short(msg, snap.short_id.length);
  snap.fallbackHash = hash;
  diffTargetFallbackHash.set(hash);
  diffTargetId.set(snap.id);
  syncUrl();
}

function buildUrl(): string {
  const vol = get(selectedVolume);
  if (!vol) return '/ui';
  const encodedVol = vol.split('/').map(encodeURIComponent).join('/');
  const p = new URLSearchParams();
  if (get(activeTab) === 'repo') p.set('tab', 'repo');
  if (get(viewerOpen) && get(currentSnapshot)) {
    const snap = get(currentSnapshot)!;
    p.set('snapshot', snap.short_id);
    
    if (snap.fallbackHash) {
      p.set('fallbackHash', snap.fallbackHash);
    }
    const dtId = get(diffTargetId);
    if (dtId) {
      const dtSnap = get(allSnapshots).find(s => s.id === dtId || s.short_id === dtId);
      if (dtSnap) {
        p.set('diff', dtSnap.short_id);
        const hash = dtSnap.fallbackHash || get(diffTargetFallbackHash);
        if (hash) {
          p.set('diffFallbackHash', hash);
        } else {
          const msg = snapshotHashInput(dtSnap);
          sha256Short(msg, dtSnap.short_id.length).then(h => {
            dtSnap.fallbackHash = h;
            syncUrl();
          });
        }
      }
    }
  }
  const qs = p.toString();
  return qs ? `/ui/volume/${encodedVol}?${qs}` : `/ui/volume/${encodedVol}`;
}

export function syncUrl() {
  const url = buildUrl();
  window.history.replaceState({}, '', url);
}
