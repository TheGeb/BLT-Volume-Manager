import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { snapshots, snapsLoading, loadSnapshots, currentPage, pageSize } from './snapshots';
import { selectedVolume } from './volumes';
import type { SnapshotsResponse } from '../types';
import * as api from '../api';
import * as toast from './toast';

vi.mock('../api', () => ({
  fetchSnapshots: vi.fn(),
  fetchSnapshotHosts: vi.fn(),
  fetchSnapshotSizes: vi.fn(),
  deleteSnapshots: vi.fn(),
  setRestorePoint: vi.fn(),
  deleteRestorePoint: vi.fn(),
  safeErrorMessage: vi.fn((e) => (e instanceof Error ? e.message : String(e))),
}));

vi.mock('./toast', () => ({
  showToast: vi.fn(),
}));

function mockSnapshotsResponse(snaps: { id: string }[]): SnapshotsResponse {
  return {
    snapshots: snaps.map(s => ({
      id: s.id,
      short_id: s.id.slice(0, 3),
      time: '2024-01-01T00:00:00Z',
      tags: [],
      paths: [],
      hostname: 'h1',
    })),
  };
}

describe('snapshot request generation', () => {
  beforeEach(() => {
    selectedVolume.set('test-vol');
    currentPage.set(1);
    pageSize.set(25);
    snapshots.set([]);
    snapsLoading.set(false);
    vi.clearAllMocks();
  });

  it('older response cannot overwrite newer response', async () => {
    let resolve1: ((v: SnapshotsResponse) => void) | undefined;
    let resolve2: ((v: SnapshotsResponse) => void) | undefined;

    vi.mocked(api.fetchSnapshots)
      .mockImplementationOnce(() => new Promise<SnapshotsResponse>(r => { resolve1 = r; }))
      .mockImplementationOnce(() => new Promise<SnapshotsResponse>(r => { resolve2 = r; }));

    const p1 = loadSnapshots('test-vol');
    const p2 = loadSnapshots('test-vol');

    resolve1?.(mockSnapshotsResponse([{ id: 'old' }]));
    await p1;
    expect(get(snapshots)).toEqual([]);

    resolve2?.(mockSnapshotsResponse([{ id: 'new' }]));
    await p2;
    expect(get(snapshots)).toHaveLength(1);
    expect(get(snapshots)[0]?.id).toBe('new');
  });

  it('aborts do not create error toasts', async () => {
    vi.mocked(api.fetchSnapshots)
      .mockImplementationOnce((_v: string, _p?: unknown, signal?: AbortSignal) => new Promise<SnapshotsResponse>((_resolve, reject) => {
        if (signal) {
          signal.addEventListener('abort', () => {
            reject(new DOMException('aborted', 'AbortError'));
          }, { once: true });
        }
      }))
      .mockImplementationOnce(() => Promise.resolve(mockSnapshotsResponse([{ id: 'second' }])));

    const p1 = loadSnapshots('test-vol');
    const p2 = loadSnapshots('test-vol');

    await expect(p1).resolves.toBeUndefined();
    await expect(p2).resolves.toBeUndefined();

    expect(vi.mocked(toast.showToast)).not.toHaveBeenCalled();
    expect(get(snapshots)).toHaveLength(1);
    expect(get(snapshots)[0]?.id).toBe('second');
  });
});
