import { writable, get } from 'svelte/store';
import type { StatsResponse, LockStatus } from '../types';
import * as api from '../api';
import { showToast } from './toast';
import { selectedVolume } from './volumes';
import { snapshots, loadSnapshots } from './snapshots';

export const loading = writable(true);
export const activeTab = writable<'snapshots' | 'repo'>('snapshots');
export const themeDark = writable(true);
export const lockStatus = writable<LockStatus | null>(null);
export const stats = writable<StatsResponse | null>(null);
export const statsLoading = writable(false);
export const checking = writable(false);
export const repairing = writable(false);
export const devMode = writable(false);

export async function loadDevMode() {
  try {
    devMode.set(await api.fetchDevMode());
  } catch {
    devMode.set(false);
  }
}

export function toggleTheme() {
  themeDark.update(v => !v);
  document.body.classList.toggle('light', !get(themeDark));
  localStorage.setItem('themeDark', JSON.stringify(get(themeDark)));
}

interface AccentColorDef {
  name: string;
  label: string;
  dark: { accent: string; soft: string; complement: string };
  light: { accent: string; soft: string; complement: string };
}

export const accentColors: AccentColorDef[] = [
  { name: 'red', dark: { accent: '#f43f5e', soft: '#7f1d2e', complement: '#2dd4bf' }, light: { accent: '#e11d48', soft: '#fb7185', complement: '#0d9488' }, label: 'Red' },
  { name: 'orange', dark: { accent: '#f97316', soft: '#7c2d12', complement: '#60a5fa' }, light: { accent: '#ea580c', soft: '#fb923c', complement: '#2563eb' }, label: 'Orange' },
  { name: 'yellow', dark: { accent: '#f59e0b', soft: '#713f12', complement: '#818cf8' }, light: { accent: '#d97706', soft: '#fbbf24', complement: '#6366f1' }, label: 'Yellow' },
  { name: 'lime', dark: { accent: '#84cc16', soft: '#2d4a0e', complement: '#a78bfa' }, light: { accent: '#4d7c0f', soft: '#84cc16', complement: '#7c3aed' }, label: 'Lime' },
  { name: 'green', dark: { accent: '#10b981', soft: '#064e3b', complement: '#f472b6' }, light: { accent: '#059669', soft: '#34d399', complement: '#db2777' }, label: 'Green' },
  { name: 'teal', dark: { accent: '#06b6d4', soft: '#0a4b4e', complement: '#fb7185' }, light: { accent: '#0891b2', soft: '#22d3ee', complement: '#e11d48' }, label: 'Teal' },
  { name: 'blue', dark: { accent: '#3b82f6', soft: '#1e3a8a', complement: '#f59e0b' }, light: { accent: '#2563eb', soft: '#60a5fa', complement: '#d97706' }, label: 'Blue' },
  { name: 'indigo', dark: { accent: '#6366f1', soft: '#312e81', complement: '#f59e0b' }, light: { accent: '#4f46e5', soft: '#818cf8', complement: '#d97706' }, label: 'Indigo' },
  { name: 'purple', dark: { accent: '#7c3aed', soft: '#3b0764', complement: '#60a5fa' }, light: { accent: '#6d28d9', soft: '#a78bfa', complement: '#3b82f6' }, label: 'Purple' },
  { name: 'pink', dark: { accent: '#ec4899', soft: '#6b142d', complement: '#34d399' }, light: { accent: '#db2777', soft: '#f472b6', complement: '#059669' }, label: 'Pink' },
];

export const currentAccent = writable('purple');

export function setAccentColor(name: string) {
  const color = accentColors.find(c => c.name === name);
  if (!color) return;
  currentAccent.set(name);
  document.documentElement.style.setProperty('--accent-dark', color.dark.accent);
  document.documentElement.style.setProperty('--accent-soft-dark', color.dark.soft);
  document.documentElement.style.setProperty('--accent-complement-dark', color.dark.complement);
  document.documentElement.style.setProperty('--accent-light', color.light.accent);
  document.documentElement.style.setProperty('--accent-soft-light', color.light.soft);
  document.documentElement.style.setProperty('--accent-complement-light', color.light.complement);
  localStorage.setItem('accentColor', name);
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
  } catch { /* stale stats ok */ } finally {
    statsLoading.set(false);
  }
}

async function withRepoOp(loading: { set: (v: boolean) => void }, apiFn: () => Promise<string>) {
  const vol = get(selectedVolume);
  if (!vol) return;
  loading.set(true);
  showToast('');
  try {
    const msg = await apiFn();
    showToast(msg);
  } catch (e: unknown) { showToast((e as Error).message, true); }
  finally { loading.set(false); }
}

export async function handleCheck() {
  await withRepoOp(checking, () => api.checkRepo(get(selectedVolume)));
}

export async function handleRepair() {
  await withRepoOp(repairing, () => api.repairRepo(get(selectedVolume)));
}

export function doSwitchTab(tab: 'snapshots' | 'repo') {
  activeTab.set(tab);
  const vol = get(selectedVolume);
  if (tab === 'repo') {
    if (vol) {
      if (!get(stats)) void loadStats(vol);
      void loadLockStatus();
    }
  } else if (vol && get(snapshots).length === 0) {
    void loadSnapshots(vol);
  }
}
