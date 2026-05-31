import type { Snapshot, LockStatus, RepoStatus, StatsResponse, SnapshotsResponse, BatchDeleteResponse, FileNode, DiffResult } from './types';

export async function fetchVolumes(): Promise<string[]> {
	const resp = await fetch('/api/volumes');
	const data = await resp.json() as { volumes?: string[] };
	return data.volumes ?? [];
}

export async function fetchSnapshots(volume: string): Promise<SnapshotsResponse> {
	const resp = await fetch(`/api/snapshots?volume=${encodeURIComponent(volume)}`);
	const data = await resp.json() as SnapshotsResponse;
	const snapshots = data.snapshots.map((sn: Snapshot) => ({ ...sn, tags: sn.tags }));
	return { snapshots, restorePointID: data.restorePointID ?? '' };
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
    const body = await resp.json() as { error?: string };
    throw new Error(body.error ?? 'Failed to delete locks');
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

export async function fetchFileTree(snapshotId: string, volume: string, path?: string, fallbackHash?: string): Promise<FileNode[]> {
  let url = `/api/snapshot-view/${encodeURIComponent(snapshotId)}/ls?volume=${encodeURIComponent(volume)}`;
  if (path) url += `&path=${encodeURIComponent(path)}`;
  if (fallbackHash) url += `&fallbackHash=${encodeURIComponent(fallbackHash)}`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error('Failed to list snapshot');
  return resp.json() as Promise<FileNode[]>;
}

export async function fetchFileContent(snapshotId: string, volume: string, path: string, fallbackHash?: string): Promise<string> {
  let url = `/api/snapshot-view/${encodeURIComponent(snapshotId)}/dump?volume=${encodeURIComponent(volume)}&path=${encodeURIComponent(path)}`;
  if (fallbackHash) url += `&fallbackHash=${encodeURIComponent(fallbackHash)}`;
  const controller = new AbortController();
  const timer = setTimeout(() => { controller.abort(); }, 30000);
  try {
    const resp = await fetch(url, { signal: controller.signal });
    if (!resp.ok) throw new Error('Failed to read file');
    return await resp.text();
  } finally {
    clearTimeout(timer);
  }
}

export async function fetchDiff(snapshotA: string, snapshotB: string, volume: string, hashA?: string, hashB?: string): Promise<DiffResult> {
  let url = `/api/snapshot-view/${encodeURIComponent(snapshotA)}/diff/${encodeURIComponent(snapshotB)}?volume=${encodeURIComponent(volume)}`;
  if (hashA) url += `&fallbackHash=${encodeURIComponent(hashA)}`;
  if (hashB) url += `&diffFallbackHash=${encodeURIComponent(hashB)}`;
  const resp = await fetch(url);
  if (!resp.ok) throw new Error('Failed to get diff');
  return resp.json() as Promise<DiffResult>;
}

export async function addTag(snapshotId: string, tag: string, volume: string): Promise<SnapshotsResponse> {
	const url = `/api/snapshot/${encodeURIComponent(snapshotId)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`;
	const resp = await fetch(url, { method: 'POST' });
	if (!resp.ok) throw new Error('Failed to add tag');
	const data = await resp.json() as SnapshotsResponse;
	const snapshots = data.snapshots.map((sn: Snapshot) => ({ ...sn, tags: sn.tags }));
	return { snapshots, restorePointID: data.restorePointID ?? '' };
}

export async function removeTag(snapshotId: string, tag: string, volume: string): Promise<SnapshotsResponse> {
	const url = `/api/snapshot/${encodeURIComponent(snapshotId)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`;
	const resp = await fetch(url, { method: 'DELETE' });
	if (!resp.ok) throw new Error('Failed to remove tag');
	const data = await resp.json() as SnapshotsResponse;
	const snapshots = data.snapshots.map((sn: Snapshot) => ({ ...sn, tags: sn.tags }));
	return { snapshots, restorePointID: data.restorePointID ?? '' };
}

export async function deleteSnapshot(snapshotId: string, volume: string): Promise<void> {
  const url = `/api/snapshot/${encodeURIComponent(snapshotId)}/delete?volume=${encodeURIComponent(volume)}`;
  const resp = await fetch(url, { method: 'DELETE' });
  if (!resp.ok) throw new Error('Failed to delete snapshot');
}

export async function deleteSnapshots(volume: string, ids: string[]): Promise<BatchDeleteResponse> {
  const resp = await fetch('/api/snapshots/delete-batch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ volume, ids }),
  });
  return parseResponse<BatchDeleteResponse>(resp);
}

export async function deleteVolume(volume: string): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}`, { method: 'DELETE' });
  if (!resp.ok) {
    const d = await resp.json() as { error?: string };
    throw new Error(d.error ?? 'delete failed');
  }
}

export async function checkRepo(volume: string): Promise<string> {
  const resp = await fetch(`/api/repo/check?volume=${encodeURIComponent(volume)}`, { method: 'POST' });
  const d = await resp.json() as { status?: string; error?: string };
  if (!resp.ok) throw new Error(d.error ?? 'check failed');
  return d.status ?? '';
}

export async function repairRepo(volume: string): Promise<string> {
  const resp = await fetch(`/api/repo/repair?volume=${encodeURIComponent(volume)}`, { method: 'POST' });
  const d = await resp.json() as { status?: string; error?: string };
  if (!resp.ok) throw new Error(d.error ?? 'repair failed');
  return d.status ?? '';
}

export async function copyVolume(source: string, target: string, preserveHistory?: boolean, snapshotIds?: string[]): Promise<{ status: string; source_locked?: boolean; source_owner?: string }> {
  const body: Record<string, unknown> = { target };
  if (snapshotIds && snapshotIds.length > 0) {
    body.snapshot_ids = snapshotIds;
  } else {
    body.preserve_history = preserveHistory;
  }
  const resp = await fetch(`/api/volume/${encodeURIComponent(source)}/copy`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  return parseResponse(resp);
}

export async function renameVolume(source: string, target: string): Promise<{ status: string }> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(source)}/rename`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ target }),
  });
  return parseResponse(resp);
}

async function parseResponse<T>(resp: Response): Promise<T> {
  let body: Record<string, unknown>;
  try {
    body = await resp.json() as Record<string, unknown>;
  } catch {
    const text = await resp.text();
    throw new Error(text || `HTTP ${String(resp.status)}`);
  }
  if (!resp.ok) {
    const err = body.error;
    if (typeof err === 'string') throw new Error(err);
    if (typeof err === 'number') throw new Error(String(err));
    throw new Error(`HTTP ${String(resp.status)}`);
  }
  return body as T;
}

export async function createTestVolume(name: string): Promise<void> {
  const resp = await fetch('/api/dummy-volume', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
  const d = await resp.json() as { error?: string };
  if (!resp.ok) throw new Error(d.error ?? `HTTP ${String(resp.status)}`);
}