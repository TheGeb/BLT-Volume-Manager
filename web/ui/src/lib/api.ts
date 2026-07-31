import type { Snapshot, OwnerStatus, StatsResponse, SnapshotsResponse, BatchDeleteResponse, FileNode, DiffResult } from './types';

export function safeErrorMessage(err: unknown): string {
	if (err instanceof Error) return err.message;
	if (typeof err === 'string') return err;
	if (err && typeof err === 'object' && 'message' in err && typeof (err as Record<string, unknown>).message === 'string') return (err as Record<string, unknown>).message as string;
	return String(err);
}

function validateSnapshotsResponse(data: unknown): SnapshotsResponse {
	if (!data || typeof data !== 'object') throw new Error('invalid snapshots response');
	const d = data as Record<string, unknown>;
	if (!Array.isArray(d.snapshots)) throw new Error('invalid snapshots response');
	return {
		snapshots: d.snapshots as Snapshot[],
		restorePointID: typeof d.restorePointID === 'string' ? d.restorePointID : '',
		hasMore: typeof d.hasMore === 'boolean' ? d.hasMore : false,
	};
}

function validateOwnerStatus(data: unknown): OwnerStatus {
	if (!data || typeof data !== 'object') throw new Error('invalid owner status response');
	const d = data as Record<string, unknown>;
	if (typeof d.volume !== 'string') throw new Error('invalid owner status');
	if (typeof d.owner !== 'string') throw new Error('invalid owner status');
	const result: OwnerStatus = { volume: d.volume, owner: d.owner };
	if (typeof d.expiry === 'number') result.expiry = d.expiry;
	return result;
}

function validateFileTree(data: unknown): FileNode[] {
	if (!Array.isArray(data)) throw new Error('invalid file tree response');
	return data as FileNode[];
}

export interface SnapshotListParams {
  host?: string;
  hosts?: string[];
  latest?: number;
  tag?: string;
  tags?: string[];
  offset?: number;
  limit?: number;
  timeFrom?: number;
  timeTo?: number;
  timeOfDayFrom?: number;
  timeOfDayTo?: number;
  versionFrom?: string;
  versionTo?: string;
  query?: string;
}

export async function fetchVolumes(): Promise<string[]> {
	const resp = await fetch('/api/volumes');
	if (!resp.ok) throw new Error('failed to fetch volumes');
	const data = await resp.json() as { volumes?: string[] };
	return data.volumes ?? [];
}

export async function fetchSnapshots(volume: string, params?: SnapshotListParams, signal?: AbortSignal): Promise<SnapshotsResponse> {
	const p = new URLSearchParams();
	p.set('volume', volume);
	if (params) {
		const hosts = params.hosts ?? (params.host ? [params.host] : []);
		for (const h of hosts) p.append('host', h);
		const tags = params.tags ?? (params.tag ? [params.tag] : []);
		for (const t of tags) p.append('tag', t);
		if (params.latest !== undefined && params.latest > 0) {
			p.set('latest', String(params.latest));
		}
		if (params.offset !== undefined && params.offset >= 0) {
			p.set('offset', String(params.offset));
		}
		if (params.limit !== undefined && params.limit > 0) {
			p.set('limit', String(params.limit));
		}
		if (params.timeFrom !== undefined) {
			p.set('timestampFrom', String(params.timeFrom));
		}
		if (params.timeTo !== undefined) {
			p.set('timestampTo', String(params.timeTo));
		}
		if (params.timeOfDayFrom !== undefined) {
			p.set('timeOfDayFrom', String(params.timeOfDayFrom));
		}
		if (params.timeOfDayTo !== undefined) {
			p.set('timeOfDayTo', String(params.timeOfDayTo));
		}
		if (params.versionFrom) {
			p.set('versionFrom', params.versionFrom);
		}
		if (params.versionTo) {
			p.set('versionTo', params.versionTo);
		}
		if (params.query) {
			p.set('query', params.query);
		}
	}
	const resp = await fetch(`/api/snapshots?${p.toString()}`, { signal: signal ?? null });
	const data = await parseResponse<SnapshotsResponse>(resp);
	const validated = validateSnapshotsResponse(data);
	const snapshots = validated.snapshots.map((sn: Snapshot) => ({ ...sn, tags: sn.tags }));
	const result: SnapshotsResponse = { snapshots };
	if (validated.restorePointID) result.restorePointID = validated.restorePointID;
	if (validated.hasMore) result.hasMore = validated.hasMore;
	return result;
}

export async function fetchSnapshotHosts(volume: string, latest = 1): Promise<string[]> {
	const p = new URLSearchParams();
	p.set('volume', volume);
	p.set('latest', String(latest));
	const resp = await fetch(`/api/snapshots/hosts?${p.toString()}`);
	if (!resp.ok) throw new Error('Failed to fetch hosts');
	const data = await resp.json() as string[];
	return data;
}

export async function fetchOwnerStatus(volume: string, signal?: AbortSignal): Promise<OwnerStatus> {
  const controller = new AbortController();
  const timer = setTimeout(() => { controller.abort(); }, 10000);
  if (signal) {
    if (signal.aborted) { controller.abort(signal.reason); }
    else { signal.addEventListener('abort', () => { controller.abort(signal.reason); }, { once: true }); }
  }
  try {
    const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/owners`, { signal: controller.signal });
    const data = await parseResponse<OwnerStatus>(resp);
    return validateOwnerStatus(data);
  } finally {
    clearTimeout(timer);
  }
}

export async function fetchAllOwnerStatus(signal?: AbortSignal): Promise<Record<string, OwnerStatus>> {
  const controller = new AbortController();
  const timer = setTimeout(() => { controller.abort(); }, 10000);
  if (signal) {
    if (signal.aborted) { controller.abort(signal.reason); }
    else { signal.addEventListener('abort', () => { controller.abort(signal.reason); }, { once: true }); }
  }
  try {
    const resp = await fetch('/api/volumes/owners', { signal: controller.signal });
    const data = await parseResponse<{ owners?: Record<string, OwnerStatus> }>(resp);
    return data.owners ?? {};
  } finally {
    clearTimeout(timer);
  }
}

export async function createOwnerLock(volume: string, owner?: string, durationMinutes?: number): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/owners`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ owner, owner_duration_mins: durationMinutes ?? 0 }),
  });
  if (!resp.ok) {
    const d = await resp.json() as { error?: string };
    throw new Error(d.error ?? 'failed to create owner lock');
  }
}

export async function deleteOwnerLock(volume: string): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/owners`, { method: 'DELETE' });
  if (!resp.ok) {
    const body = await resp.json() as { error?: string };
    throw new Error(body.error ?? 'Failed to delete owners');
  }
}

export async function fetchStats(volume: string): Promise<StatsResponse> {
  const resp = await fetch(`/api/stats?volume=${encodeURIComponent(volume)}`);
  if (!resp.ok) {
    const d = await resp.json() as { error?: string };
    throw new Error(d.error ?? 'failed to fetch stats');
  }
  return resp.json() as Promise<StatsResponse>;
}

export async function refreshStats(): Promise<void> {
  const resp = await fetch('/api/stats/refresh', { method: 'POST' });
  if (!resp.ok) throw new Error('failed to refresh stats');
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

export async function fetchFileTree(snapshotId: string, volume: string, path?: string, fallbackHash?: string, signal?: AbortSignal): Promise<FileNode[]> {
  let url = `/api/snapshot-view/${encodeURIComponent(snapshotId)}/ls?volume=${encodeURIComponent(volume)}`;
  if (path) url += `&path=${encodeURIComponent(path)}`;
  if (fallbackHash) url += `&fallbackHash=${encodeURIComponent(fallbackHash)}`;
  const resp = await fetch(url, { signal: signal ?? null });
  const data = await parseResponse<FileNode[]>(resp);
  return validateFileTree(data);
}

export async function fetchFileContent(snapshotId: string, volume: string, path: string, fallbackHash?: string, signal?: AbortSignal): Promise<string> {
  let url = `/api/snapshot-view/${encodeURIComponent(snapshotId)}/dump?volume=${encodeURIComponent(volume)}&path=${encodeURIComponent(path)}`;
  if (fallbackHash) url += `&fallbackHash=${encodeURIComponent(fallbackHash)}`;
  const controller = new AbortController();
  const timer = setTimeout(() => { controller.abort(); }, 30000);
  if (signal) {
    if (signal.aborted) { controller.abort(signal.reason); }
    else { signal.addEventListener('abort', () => { controller.abort(signal.reason); }, { once: true }); }
  }
  try {
    const resp = await fetch(url, { signal: controller.signal });
    return await parseResponse<string>(resp);
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

export async function deleteSnapshots(volume: string, ids: string[]): Promise<BatchDeleteResponse> {
  const resp = await fetch('/api/snapshots/delete-batch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ volume, ids }),
  });
  return parseResponse<BatchDeleteResponse>(resp);
}

export async function setRestorePoint(volume: string, snapshotId: string): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/restore-point`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ snapshot_id: snapshotId }),
  });
  if (!resp.ok) {
    const d = await resp.json() as { error?: string };
    throw new Error(d.error ?? 'failed to set restore point');
  }
}

export async function deleteRestorePoint(volume: string): Promise<void> {
  const resp = await fetch(`/api/volume/${encodeURIComponent(volume)}/restore-point`, { method: 'DELETE' });
  if (!resp.ok) {
    const d = await resp.json() as { error?: string };
    throw new Error(d.error ?? 'failed to delete restore point');
  }
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

export async function copyVolume(source: string, target: string, preserveHistory?: boolean, snapshotIds?: string[]): Promise<{ status: string; source_owned?: boolean; source_owner?: string }> {
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
  const text = await resp.text();
  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    if (!resp.ok) throw new Error(text || `HTTP ${String(resp.status)}`);
    return text as T;
  }
  if (!resp.ok) {
    if (body && typeof body === 'object') {
      const errObj = body as Record<string, unknown>;
      if (typeof errObj.error === 'string') throw new Error(errObj.error);
      if (typeof errObj.error === 'number') throw new Error(String(errObj.error));
    }
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

export async function createTestSnapshot(volume: string): Promise<void> {
  const resp = await fetch('/api/dummy-snapshot', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ volume }),
  });
  const d = await resp.json() as { error?: string };
  if (!resp.ok) throw new Error(d.error ?? `HTTP ${String(resp.status)}`);
}

export async function fetchDevMode(): Promise<boolean> {
  const resp = await fetch('/api/dev-mode');
  if (!resp.ok) throw new Error('failed to fetch dev mode');
  const data = await resp.json() as { enabled?: boolean };
  return data.enabled ?? false;
}

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  metadata_backend: string;
  s3_endpoint: string;
  s3_bucket: string;
  etcd_endpoints: string[];
}

export async function fetchVersion(): Promise<VersionInfo> {
  const resp = await fetch('/api/version');
  if (!resp.ok) throw new Error('failed to fetch version');
  return resp.json() as Promise<VersionInfo>;
}
