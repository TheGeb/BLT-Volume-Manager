import { writable, derived, get } from 'svelte/store';
import type { Snapshot, StatsResponse, LockStatus, VolumeLockInfo } from './types';
import { formatBytes } from './util';
import * as api from './api';

export const snapshots = writable<Snapshot[]>([]);
export const volumes = writable<string[]>([]);
export const selectedVolume = writable('');
export const volumeFilter = writable('');
export const query = writable('');
export const sortNewestFirst = writable(true);
export const hostname = writable('');
export const prevStats = writable<StatsResponse | null>(null);
export const typeFilter = writable('all');
export const hostFilter = writable('');

export const loading = writable(true);
export const activeTab = writable<'snapshots' | 'repo'>('snapshots');
export const bannerText = writable('');
export const bannerError = writable(false);
export const lockStatus = writable<LockStatus | null>(null);
export const stats = writable<StatsResponse | null>(null);
export const statsLoading = writable(false);
export const sizes = writable<Record<string, string>>({});
export const currentSnapshot = writable<Snapshot | null>(null);
export const allSnapshots = writable<Snapshot[]>([]);
export const viewerOpen = writable(false);
export const checking = writable(false);
export const repairing = writable(false);
export const deleteVolModal = writable(false);
export const deleteSnapModal = writable(false);
export const deletingSnap = writable<Snapshot | null>(null);
export const deleteConfirmText = writable('');
export const snapDeleteInput = writable('');
export const deleteVolLoading = writable(false);
export const creatingTest = writable(false);
export const testStatus = writable('');
export const themeDark = writable(true);
export const pillsCachedAt = writable('');
export const pillsLoading = writable(false);
export const snapsLoading = writable(false);
export const landingShown = writable(true);
export const rpLoading = writable<Record<string, boolean>>({});
export const sizeLoading = writable<Record<string, boolean>>({});
export const diffTargetId = writable('');
export const volumeLockInfo = writable<Record<string, VolumeLockInfo>>({});

export const filteredVolumes = derived(
  [volumes, volumeFilter],
  ([$volumes, $volumeFilter]) =>
    $volumes.filter(v => v.toLowerCase().includes($volumeFilter.toLowerCase()))
);

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

export function setBanner(msg: string, isError = false) {
  bannerText.set(msg);
  bannerError.set(isError);
}

export async function loadVolumes() {
  pillsLoading.set(true);
  try {
    volumes.set(await api.fetchVolumes());
    fetchAllVolumeLockInfo();
  } catch {
    setBanner('Cannot reach server', true);
  } finally {
    pillsLoading.set(false);
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

export async function loadSnapshots(volume: string) {
  snapsLoading.set(true);
  try {
    snapshots.set(await api.fetchSnapshots(volume));
  } catch {
    snapshots.set([]);
    setBanner('Failed to load snapshots', true);
  } finally {
    snapsLoading.set(false);
  }
}

export async function loadLockStatus() {
  const vol = get(selectedVolume);
  if (!vol) { lockStatus.set(null); return; }
  try {
    lockStatus.set(await api.fetchLockStatus(vol));
  } catch { lockStatus.set(null); }
}

export async function loadStats(volume: string) {
  statsLoading.set(true);
  try {
    const s = await api.fetchStats(volume);
    stats.set(s);
    prevStats.set(s);
  } catch { /* stale stats ok */ } finally {
    statsLoading.set(false);
  }
}

export async function loadAll(volume: string) {
  selectedVolume.set(volume);
  allSnapshots.set([]);
  viewerOpen.set(false);
  currentSnapshot.set(null);
  sizes.set({});
  deleteVolModal.set(false);
  deleteSnapModal.set(false);
  testStatus.set('');
  diffTargetId.set('');
  landingShown.set(!volume);
  if (volume) {
    await Promise.all([
      loadSnapshots(volume),
      loadLockStatus(),
      loadStats(volume),
    ]);
  }
  syncUrl();
}

export async function navigateTo(volume: string, opts?: { tab?: string; snapshotId?: string; diffId?: string; fallbackHash?: string; diffFallbackHash?: string }) {
  selectedVolume.set(volume);
  allSnapshots.set([]);
  sizes.set({});
  deleteVolModal.set(false);
  deleteSnapModal.set(false);
  testStatus.set('');
  landingShown.set(false);

  const tab = opts?.tab || 'snapshots';
  activeTab.set(tab);

  if (tab === 'snapshots' && opts?.snapshotId) {
    viewerOpen.set(true);
    diffTargetId.set(opts.diffId || '');
  } else {
    viewerOpen.set(false);
    currentSnapshot.set(null);
    diffTargetId.set('');
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
          diffTargetId.set(diffSnap.id);
        }
      }
    }
    allSnapshots.set(get(snapshots));
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

export function onToggleSort() {
  sortNewestFirst.update(v => !v);
}

export function onSearch(q: string) { query.set(q); }
export function onFilterChange(f: string) { volumeFilter.set(f); }
export function onTypeFilter(t: string) { typeFilter.set(t); }
export function onHostFilter(h: string) { hostFilter.set(h); }

async function sha256Short(message: string, length: number): Promise<string> {
  const msgBuffer = new TextEncoder().encode(message);
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
  const hashArray = Array.from(new Uint8Array(hashBuffer));
  const fullHash = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
  return fullHash.substring(0, length);
}

function snapshotHashInput(snap: Snapshot): string {
  const paths = snap.paths ? [...snap.paths].sort().join(',') : '';
  return snap.hostname + snap.time + (snap.tree || '') + paths;
}

export async function onOpenViewer(snapshot: Snapshot) {
  const msg = snapshotHashInput(snapshot);
  const hash = await sha256Short(msg, snapshot.short_id.length);
  snapshot.fallbackHash = hash;

  currentSnapshot.set(snapshot);
  allSnapshots.set(get(snapshots));
  viewerOpen.set(true);
  diffTargetId.set('');
  syncUrl();
}

export function onCloseViewer() {
  viewerOpen.set(false);
  currentSnapshot.set(null);
  allSnapshots.set([]);
  diffTargetId.set('');
  syncUrl();
}

export async function setDiffTarget(id: string) {
  const snap = get(allSnapshots).find(s => s.id === id || s.short_id === id);
  if (snap) {
    const msg = snapshotHashInput(snap);
    const hash = await sha256Short(msg, snap.short_id.length);
    snap.fallbackHash = hash;
  }
  diffTargetId.set(id);
  syncUrl();
}

export async function onAddTag(id: string, tag: string, vol: string) {
  rpLoading.update(r => ({ ...r, [id]: true }));
  try {
    const snaps = await api.addTag(id, tag, vol);
    snapshots.set(snaps);
  } catch (e) {
    setBanner(`Failed to add tag: ${e}`, true);
    await loadSnapshots(vol);
  } finally {
    rpLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
  }
}

export async function onRemoveTag(id: string, tag: string, vol: string) {
  rpLoading.update(r => ({ ...r, [id]: true }));
  try {
    const snaps = await api.removeTag(id, tag, vol);
    snapshots.set(snaps);
  } catch (e) {
    setBanner(`Failed to remove tag: ${e}`, true);
    await loadSnapshots(vol);
  } finally {
    rpLoading.update(r => { const n = { ...r }; delete n[id]; return n; });
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
  } catch (e: any) { setBanner(e.message, true); }
}

export function openDeleteVolModal() {
  const vol = get(selectedVolume);
  if (!vol) return;
  deleteConfirmText.set('');
  deleteVolModal.set(true);
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
    allSnapshots.set([]);
    currentSnapshot.set(null);
    viewerOpen.set(false);
    sizes.set({});
    await Promise.all([
      api.refreshStats().catch(() => {}),
      loadVolumes(),
    ]);
  } catch (e: any) {
    deleteVolLoading.set(false);
    setBanner(e.message, true);
  }
}

export async function handleCheck() {
  const vol = get(selectedVolume);
  if (!vol) return;
  checking.set(true);
  setBanner('');
  try {
    const msg = await api.checkRepo(vol);
    setBanner(msg);
  } catch (e: any) { setBanner(e.message, true); }
  finally { checking.set(false); }
}

export async function handleRepair() {
  const vol = get(selectedVolume);
  if (!vol) return;
  repairing.set(true);
  setBanner('');
  try {
    const msg = await api.repairRepo(vol);
    setBanner(msg);
  } catch (e: any) { setBanner(e.message, true); }
  finally { repairing.set(false); }
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
  } catch (e: any) { testStatus.set(e.message); }
  finally { creatingTest.set(false); }
}

export function switchTab(tab: 'snapshots' | 'repo') {
  activeTab.set(tab);
  const vol = get(selectedVolume);
  if (tab === 'repo') {
    if (vol) loadStats(vol);
  } else if (vol && get(snapshots).length === 0) {
    loadSnapshots(vol);
  }
  syncUrl();
}

export function toggleTheme() {
  themeDark.update(v => !v);
  document.body.classList.toggle('light', !get(themeDark));
  localStorage.setItem('themeDark', JSON.stringify(get(themeDark)));
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
      p.set('diff', dtSnap ? dtSnap.short_id : dtId);
      if (dtSnap && dtSnap.fallbackHash) {
        p.set('diffFallbackHash', dtSnap.fallbackHash);
      } else if (dtSnap) {
        const msg = snapshotHashInput(dtSnap);
        sha256Short(msg, dtSnap.short_id.length).then(h => {
          dtSnap.fallbackHash = h;
          syncUrl();
        });
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
