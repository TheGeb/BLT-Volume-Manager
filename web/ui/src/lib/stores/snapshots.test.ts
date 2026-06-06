import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { snapshots, query, sortNewestFirst, typeFilter, hostFilter, deleteSnapModal, snapDeleteInput, selectedForDeletion, filteredSnapshots, sortedSnapshots, hosts, timeFrom, timeTo, timeOfDayFrom, timeOfDayTo, onToggleSort, onSearch, onTypeFilter, onHostFilter, toggleForDeletion, openBulkDeleteModal, filterSnapshots, sortSnapshots, extractHosts } from './snapshots';
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
    expect(filterSnapshots(snaps, '')).toEqual(snaps);
  });

  it('filters by query matching short_id', () => {
    expect(filterSnapshots(snaps, 'b')).toEqual([snapB]);
  });

  it('filters by query matching hostname', () => {
    expect(filterSnapshots(snaps, 'h1')).toEqual([snapA, snapC]);
  });

  it('filters by query matching tags', () => {
    expect(filterSnapshots(snaps, 'hot')).toEqual([snapA]);
  });

  it('filters by query case-insensitively', () => {
    expect(filterSnapshots(snaps, 'HOT')).toEqual([snapA]);
  });

  it('filters by time range', () => {
    const from = new Date('2024-02-15T00:00:00Z').getTime();
    expect(filterSnapshots(snaps, '', from)).toEqual([snapA]);
  });

  it('returns empty when no match', () => {
    expect(filterSnapshots(snaps, 'nonexistent')).toEqual([]);
  });
});

describe('sortSnapshots', () => {
  it('sorts newest first', () => {
    const result = sortSnapshots([snapC, snapA, snapB], true);
    const r0 = result.at(0);
    const r1 = result.at(1);
    const r2 = result.at(2);
    if (!r0 || !r1 || !r2) throw new Error('expected 3 results');
    expect(r0.id).toBe('1');
    expect(r1.id).toBe('2');
    expect(r2.id).toBe('3');
  });

  it('sorts oldest first', () => {
    const result = sortSnapshots([snapC, snapA, snapB], false);
    const r0 = result.at(0);
    const r1 = result.at(1);
    const r2 = result.at(2);
    if (!r0 || !r1 || !r2) throw new Error('expected 3 results');
    expect(r0.id).toBe('3');
    expect(r1.id).toBe('2');
    expect(r2.id).toBe('1');
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
    timeFrom.set(undefined);
    timeTo.set(undefined);
    timeOfDayFrom.set(undefined);
    timeOfDayTo.set(undefined);
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

  it('filteredSnapshots filters by time range', () => {
    snapshots.set([snapA, snapB, snapC]);
    timeFrom.set(new Date('2024-02-15T00:00:00Z').getTime());
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

describe('toggleForDeletion', () => {
  beforeEach(() => {
    selectedForDeletion.set(new Set());
    deleteSnapModal.set(false);
    snapDeleteInput.set('dirty');
  });

  it('adds a snapshot to the selection set', () => {
    const snap = makeSnap({ id: 'del-id' });
    toggleForDeletion(snap);
    expect(get(selectedForDeletion).has('del-id')).toBe(true);
  });

  it('removes a snapshot when toggled again', () => {
    const snap = makeSnap({ id: 'del-id' });
    toggleForDeletion(snap);
    toggleForDeletion(snap);
    expect(get(selectedForDeletion).has('del-id')).toBe(false);
  });
});

describe('openBulkDeleteModal', () => {
  beforeEach(() => {
    snapDeleteInput.set('dirty');
    deleteSnapModal.set(false);
  });

  it('clears the delete input and opens modal', () => {
    openBulkDeleteModal();
    expect(get(snapDeleteInput)).toBe('');
    expect(get(deleteSnapModal)).toBe(true);
  });
});
