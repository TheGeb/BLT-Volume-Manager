import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { snapshots, query, sortNewestFirst, typeFilter, hostFilter, deletingSnap, deleteSnapModal, snapDeleteInput, filteredSnapshots, sortedSnapshots, hosts, onToggleSort, onSearch, onTypeFilter, onHostFilter, onDeleteSnapshot, filterSnapshots, sortSnapshots, extractHosts } from './snapshots';
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

describe('filterSnapshots', () => {
  const snaps = [snapA, snapB, snapC];

  it('returns all when no filters active', () => {
    expect(filterSnapshots(snaps, 'all', '', '')).toEqual(snaps);
  });

  it('filters by hot tag', () => {
    expect(filterSnapshots(snaps, 'hot', '', '')).toEqual([snapA]);
  });

  it('filters by cold tag', () => {
    expect(filterSnapshots(snaps, 'cold', '', '')).toEqual([snapB]);
  });

  it('filters by hostname', () => {
    expect(filterSnapshots(snaps, 'all', 'h1', '')).toEqual([snapA, snapC]);
  });

  it('filters by query matching short_id', () => {
    expect(filterSnapshots(snaps, 'all', '', 'b')).toEqual([snapB]);
  });

  it('filters by query matching hostname', () => {
    expect(filterSnapshots(snaps, 'all', '', 'h1')).toEqual([snapA, snapC]);
  });

  it('filters by query matching tags', () => {
    expect(filterSnapshots(snaps, 'all', '', 'hot')).toEqual([snapA]);
  });

  it('filters by query case-insensitively', () => {
    expect(filterSnapshots(snaps, 'all', '', 'HOT')).toEqual([snapA]);
  });

  it('combines multiple filters', () => {
    expect(filterSnapshots(snaps, 'hot', 'h1', '')).toEqual([snapA]);
  });

  it('returns empty when no match', () => {
    expect(filterSnapshots(snaps, 'all', '', 'nonexistent')).toEqual([]);
  });
});

describe('sortSnapshots', () => {
  it('sorts newest first', () => {
    const result = sortSnapshots([snapC, snapA, snapB], true);
    expect(result[0]!.id).toBe('1');
    expect(result[1]!.id).toBe('2');
    expect(result[2]!.id).toBe('3');
  });

  it('sorts oldest first', () => {
    const result = sortSnapshots([snapC, snapA, snapB], false);
    expect(result[0]!.id).toBe('3');
    expect(result[1]!.id).toBe('2');
    expect(result[2]!.id).toBe('1');
  });

  it('does not mutate the original array', () => {
    const original = [snapC, snapA, snapB];
    sortSnapshots(original, true);
    expect(original[0]).toBe(snapC);
  });

  it('handles empty array', () => {
    expect(sortSnapshots([], true)).toEqual([]);
  });
});

describe('extractHosts', () => {
  it('extracts unique sorted hostnames', () => {
    expect(extractHosts([snapA, snapB, snapC])).toEqual(['h1', 'h2']);
  });

  it('returns empty when no snapshots', () => {
    expect(extractHosts([])).toEqual([]);
  });

  it('deduplicates hostnames', () => {
    const snaps = [
      makeSnap({ hostname: 'h1' }),
      makeSnap({ hostname: 'h1' }),
      makeSnap({ hostname: 'h2' }),
    ];
    expect(extractHosts(snaps)).toEqual(['h1', 'h2']);
  });
});

describe('derived stores initial state', () => {
  beforeEach(() => {
    snapshots.set([]);
    query.set('');
    sortNewestFirst.set(true);
    typeFilter.set('all');
    hostFilter.set('');
  });

  it('filteredSnapshots starts empty', () => {
    expect(get(filteredSnapshots)).toEqual([]);
  });

  it('sortedSnapshots starts empty', () => {
    expect(get(sortedSnapshots)).toEqual([]);
  });

  it('hosts starts empty', () => {
    expect(get(hosts)).toEqual([]);
  });

  it('filteredSnapshots computes initial value', () => {
    snapshots.set([snapA, snapB, snapC]);
    typeFilter.set('hot');
    expect(get(filteredSnapshots)).toEqual([snapA]);
  });

  it('sortedSnapshots computes initial value', () => {
    snapshots.set([snapC, snapA, snapB]);
    expect(get(sortedSnapshots).map(s => s.id)).toEqual(['1', '2', '3']);
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
