import { describe, it, expect } from 'vitest';
import { snapshotHashInput } from './stores/snapshots';
import type { Snapshot } from './types';

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
