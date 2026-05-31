import { writable, get } from 'svelte/store';
import type { StatsResponse, LockStatus } from '../types';
import * as api from '../api';
import { setBanner } from './banner';
import { selectedVolume } from './volumes';
import { snapshots, loadSnapshots } from './snapshots';

export const loading = writable(true);
export const activeTab = writable<'snapshots' | 'repo'>('snapshots');
export const themeDark = writable(true);
export const prevStats = writable<StatsResponse | null>(null);
export const lockStatus = writable<LockStatus | null>(null);
export const stats = writable<StatsResponse | null>(null);
export const statsLoading = writable(false);
export const checking = writable(false);
export const repairing = writable(false);

export function toggleTheme() {
  themeDark.update(v => !v);
  document.body.classList.toggle('light', !get(themeDark));
  localStorage.setItem('themeDark', JSON.stringify(get(themeDark)));
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

export async function handleCheck() {
  const vol = get(selectedVolume);
  if (!vol) return;
  checking.set(true);
  setBanner('');
  try {
    const msg = await api.checkRepo(vol);
    setBanner(msg);
  } catch (e: unknown) { setBanner((e as Error).message, true); }
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
  } catch (e: unknown) { setBanner((e as Error).message, true); }
  finally { repairing.set(false); }
}

export function doSwitchTab(tab: 'snapshots' | 'repo') {
  activeTab.set(tab);
  const vol = get(selectedVolume);
  if (tab === 'repo') {
    if (vol) void loadStats(vol);
  } else if (vol && get(snapshots).length === 0) {
    void loadSnapshots(vol);
  }
}
