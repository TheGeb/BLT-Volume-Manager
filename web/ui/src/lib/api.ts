import type { Snapshot, LockStatus, RepoStatus, StatsResponse } from './types';

export async function fetchVolumes(): Promise<string[]> {
  const resp = await fetch('/api/pills');
  const data = await resp.json() as { volumes: string[] };
  return data.volumes || [];
}

export async function fetchSnapshots(volume: string): Promise<Snapshot[]> {
  const resp = await fetch(`/api/snapshots?volume=${encodeURIComponent(volume)}`);
  return resp.json() as Promise<Snapshot[]>;
}

export async function fetchRepoStatus(volume: string): Promise<RepoStatus> {
  const resp = await fetch(`/api/repo/status?volume=${encodeURIComponent(volume)}`);
  return resp.json() as Promise<RepoStatus>;
}

export async function initRepo(): Promise<void> {
  await fetch('/api/repo/init', { method: 'POST' });
}

export async function fetchLockStatus(volume: string): Promise<LockStatus> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/locks`);
  return resp.json() as Promise<LockStatus>;
}

export async function createLock(volume: string, owner?: string): Promise<void> {
  await fetch(`/api/volume/${encodeURIComponent(volume)}/locks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ owner }),
  });
}

export async function deleteLocks(volume: string): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/locks`, { method: 'DELETE' });
  if (!resp.ok) {
    const body = await resp.json();
    throw new Error(body.error || 'Failed to delete locks');
  }
}

export async function fetchStats(volume: string): Promise<StatsResponse> {
  const resp = await fetch(`/api/stats?volume=${encodeURIComponent(volume)}`);
  return resp.json() as Promise<StatsResponse>;
}

export async function refreshStats(): Promise<void> {
  await fetch('/api/stats/refresh', { method: 'POST' });
}

export async function fetchSnapshotSizes(volume: string, ids: string[]): Promise<Record<string, number>> {
  const resp = await fetch('/api/snapshot/sizes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ volume, ids }),
  });
  if (!resp.ok) throw new Error('Failed to fetch sizes');
  return resp.json() as Promise<Record<string, number>>;
}

export async function fetchFileTree(snapshotId: string, volume: string, path?: string): Promise<any[]> {
  let url = `/api/snapshot-view/${encodeURIComponent(snapshotId)}/ls?volume=${encodeURIComponent(volume)}`;
  if (path) url += `&path=${encodeURIComponent(path)}`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error('Failed to list snapshot');
  return resp.json() as Promise<any[]>;
}

export async function fetchFileContent(snapshotId: string, volume: string, path: string): Promise<string> {
  const url = `/api/snapshot-view/${encodeURIComponent(snapshotId)}/dump?volume=${encodeURIComponent(volume)}&path=${encodeURIComponent(path)}`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error('Failed to read file');
  return resp.text();
}

export async function fetchDiff(snapshotA: string, snapshotB: string, volume: string): Promise<any> {
  const url = `/api/snapshot-view/${encodeURIComponent(snapshotA)}/diff/${encodeURIComponent(snapshotB)}?volume=${encodeURIComponent(volume)}`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error('Failed to get diff');
  return resp.json() as Promise<any>;
}

export async function addTag(snapshotId: string, tag: string, volume: string): Promise<void> {
  const url = `/api/snapshot/${encodeURIComponent(snapshotId)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`;
  const resp = await fetch(url, { method: 'POST' });
  if (!resp.ok) throw new Error('Failed to add tag');
}

export async function removeTag(snapshotId: string, tag: string, volume: string): Promise<void> {
  const url = `/api/snapshot/${encodeURIComponent(snapshotId)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`;
  const resp = await fetch(url, { method: 'DELETE' });
  if (!resp.ok) throw new Error('Failed to remove tag');
}

export async function deleteSnapshot(snapshotId: string, volume: string): Promise<void> {
  const url = `/api/snapshot/${encodeURIComponent(snapshotId)}/delete?volume=${encodeURIComponent(volume)}`;
  const resp = await fetch(url, { method: 'DELETE' });
  if (!resp.ok) throw new Error('Failed to delete snapshot');
}

export async function deleteVolume(volume: string): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}`, { method: 'DELETE' });
  if (!resp.ok) {
    const d = await resp.json();
    throw new Error(d.error || 'delete failed');
  }
}

export async function checkRepo(volume: string): Promise<string> {
  const resp = await fetch(`/api/repo/check?volume=${encodeURIComponent(volume)}`, { method: 'POST' });
  const d = await resp.json();
  if (!resp.ok) throw new Error(d.error || 'check failed');
  return d.status;
}

export async function repairRepo(volume: string): Promise<string> {
  const resp = await fetch(`/api/repo/repair?volume=${encodeURIComponent(volume)}`, { method: 'POST' });
  const d = await resp.json();
  if (!resp.ok) throw new Error(d.error || 'repair failed');
  return d.status;
}

export async function createTestVolume(name: string): Promise<void> {
  const resp = await fetch('/api/test/create-volume', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  const d = await resp.json();
  if (!resp.ok) throw new Error(d.error || `HTTP ${resp.status}`);
}
