import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { snapshots, query, sortNewestFirst, typeFilter, hostFilter, deletingSnap, deleteSnapModal, snapDeleteInput, filteredSnapshots, sortedSnapshots, hosts, snapshotHashInput, sha256Short, getSnapshotHash, onToggleSort, onSearch, onTypeFilter, onHostFilter, onDeleteSnapshot } from './snapshots';
import type { Snapshot } from '../types';

function makeSnap(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    id: 'abc123',
    short_id: 'abc',
    time: '2024-01-01T00:00:00Z',
    tags: [],
    paths: ['/volumes/vol1'],
    hostname: 'host1',
    ...overrides,
  };
}

const snapA = makeSnap({ id: '1', short_id: 'a', time: '2024-03-01T00:00:00Z', hostname: 'h1', tags: ['hot'] });
const snapB = makeSnap({ id: '2', short_id: 'b', time: '2024-02-01T00:00:00Z', hostname: 'h2', tags: ['cold'] });
const snapC = makeSnap({ id: '3', short_id: 'c', time: '2024-01-01T00:00:00Z', hostname: 'h1', tags: [] });

describe('snapshotHashInput', () => {
  it('concatenates hostname, time, tree, and sorted paths', () => {
    const snap = makeSnap({
      hostname: 'server1',
      time: '2024-06-15T10:30:00Z',
      tree: 'treehash123',
      paths: ['/volumes/test/data', '/volumes/test/config'],
    });
    const result = snapshotHashInput(snap);
    expect(result).toContain('server1');
    expect(result).toContain('2024-06-15T10:30:00Z');
    expect(result).toContain('treehash123');
    expect(result).toContain('/volumes/test/config');
    expect(result).toContain('/volumes/test/data');
  });

  it('sorts paths for consistency', () => {
    const snap1 = makeSnap({ paths: ['/b', '/a', '/c'] });
    const snap2 = makeSnap({ paths: ['/a', '/b', '/c'] });
    expect(snapshotHashInput(snap1)).toBe(snapshotHashInput(snap2));
  });

  it('handles missing tree', () => {
    const snap = makeSnap({ tree: undefined });
    expect(snapshotHashInput(snap)).toBeDefined();
  });

  it('handles empty paths', () => {
    const snap = makeSnap({ paths: [] });
    expect(snapshotHashInput(snap)).toBeDefined();
  });
});

describe('sha256Short', () => {
  it('returns a hex string of the requested length', async () => {
    const hash = await sha256Short('hello', 8);
    expect(hash).toMatch(/^[0-9a-f]{8}$/);
  });

  it('returns different hashes for different inputs', async () => {
    const [h1, h2] = await Promise.all([
      sha256Short('foo', 16),
      sha256Short('bar', 16),
    ]);
    expect(h1).not.toBe(h2);
  });

  it('returns consistent results for same input', async () => {
    const [h1, h2] = await Promise.all([
      sha256Short('consistent', 12),
      sha256Short('consistent', 12),
    ]);
    expect(h1).toBe(h2);
  });

  it('handles length 0', async () => {
    expect(await sha256Short('anything', 0)).toBe('');
  });
});

describe('getSnapshotHash', () => {
  it('returns fallbackHash if present', async () => {
    const snap = makeSnap({ fallbackHash: 'myhash' });
    expect(await getSnapshotHash(snap)).toBe('myhash');
  });

  it('computes and caches hash', async () => {
    const snap = makeSnap({ short_id: 'abc' });
    const hash = await getSnapshotHash(snap);
    expect(hash).toMatch(/^[0-9a-f]{3}$/);
    expect(snap.fallbackHash).toBe(hash);
  });
});

describe('filteredSnapshots', () => {
  beforeEach(() => {
    snapshots.set([snapA, snapB, snapC]);
    typeFilter.set('all');
    hostFilter.set('');
    query.set('');
  });

  it('returns all when no filters active', () => {
    expect(get(filteredSnapshots)).toEqual([snapA, snapB, snapC]);
  });

  it('filters by hot tag', () => {
    typeFilter.set('hot');
    expect(get(filteredSnapshots)).toEqual([snapA]);
  });

  it('filters by cold tag', () => {
    typeFilter.set('cold');
    expect(get(filteredSnapshots)).toEqual([snapB]);
  });

  it('filters by hostname', () => {
    hostFilter.set('h1');
    expect(get(filteredSnapshots)).toEqual([snapA, snapC]);
  });

  it('filters by query matching short_id', () => {
    query.set('b');
    expect(get(filteredSnapshots)).toEqual([snapB]);
  });

  it('filters by query matching hostname', () => {
    query.set('h1');
    expect(get(filteredSnapshots)).toEqual([snapA, snapC]);
  });

  it('filters by query matching tags', () => {
    query.set('hot');
    expect(get(filteredSnapshots)).toEqual([snapA]);
  });

  it('filters by query case-insensitively', () => {
    query.set('HOT');
    expect(get(filteredSnapshots)).toEqual([snapA]);
  });

  it('combines multiple filters', () => {
    typeFilter.set('hot');
    hostFilter.set('h1');
    expect(get(filteredSnapshots)).toEqual([snapA]);
  });

  it('returns empty when no match', () => {
    query.set('nonexistent');
    expect(get(filteredSnapshots)).toEqual([]);
  });
});

describe('sortedSnapshots', () => {
  beforeEach(() => {
    snapshots.set([snapC, snapA, snapB]);
    typeFilter.set('all');
    hostFilter.set('');
    query.set('');
    sortNewestFirst.set(true);
  });

  it('sorts newest first by default', () => {
    const result = get(sortedSnapshots);
    expect(result[0].id).toBe('1');
    expect(result[1].id).toBe('2');
    expect(result[2].id).toBe('3');
  });

  it('sorts oldest first when toggled', () => {
    sortNewestFirst.set(false);
    const result = get(sortedSnapshots);
    expect(result[0].id).toBe('3');
    expect(result[1].id).toBe('2');
    expect(result[2].id).toBe('1');
  });

  it('does not mutate the original array', () => {
    const original = get(snapshots);
    get(sortedSnapshots);
    expect(get(snapshots)).toBe(original);
    expect(get(snapshots)[0]).toBe(snapC);
  });
});

describe('hosts', () => {
  beforeEach(() => {
    snapshots.set([]);
  });

  it('extracts unique sorted hostnames', () => {
    snapshots.set([snapA, snapB, snapC]);
    expect(get(hosts)).toEqual(['h1', 'h2']);
  });

  it('returns empty when no snapshots', () => {
    expect(get(hosts)).toEqual([]);
  });

  it('deduplicates hostnames', () => {
    snapshots.set([
      makeSnap({ hostname: 'h1' }),
      makeSnap({ hostname: 'h1' }),
      makeSnap({ hostname: 'h2' }),
    ]);
    expect(get(hosts)).toEqual(['h1', 'h2']);
  });
});

describe('onToggleSort', () => {
  it('toggles sort direction', () => {
    sortNewestFirst.set(true);
    onToggleSort();
    expect(get(sortNewestFirst)).toBe(false);
    onToggleSort();
    expect(get(sortNewestFirst)).toBe(true);
  });
});

describe('onSearch', () => {
  it('sets the query', () => {
    query.set('');
    onSearch('test-query');
    expect(get(query)).toBe('test-query');
  });
});

describe('onTypeFilter', () => {
  it('sets the type filter', () => {
    typeFilter.set('all');
    onTypeFilter('hot');
    expect(get(typeFilter)).toBe('hot');
  });
});

describe('onHostFilter', () => {
  it('sets the host filter', () => {
    hostFilter.set('');
    onHostFilter('host1');
    expect(get(hostFilter)).toBe('host1');
  });
});

describe('onDeleteSnapshot', () => {
  beforeEach(() => {
    deletingSnap.set(null);
    deleteSnapModal.set(false);
    snapDeleteInput.set('dirty');
  });

  it('sets the snapshot to delete and opens modal', () => {
    const snap = makeSnap({ id: 'del-id' });
    onDeleteSnapshot(snap);
    expect(get(deletingSnap)?.id).toBe('del-id');
    expect(get(deleteSnapModal)).toBe(true);
  });

  it('clears the delete input', () => {
    onDeleteSnapshot(makeSnap());
    expect(get(snapDeleteInput)).toBe('');
  });
});
