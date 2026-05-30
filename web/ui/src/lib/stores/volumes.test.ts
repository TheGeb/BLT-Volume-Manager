import { describe, it, expect, beforeEach } from 'vitest';
import { get, derived } from 'svelte/store';
import { volumes, volumeFilter, selectedVolume, deleteVolModal, deleteConfirmText, filteredVolumes, onFilterChange, openDeleteVolModal } from './volumes';

describe('filteredVolumes', () => {
  beforeEach(() => {
    volumes.set([]);
    volumeFilter.set('');
  });

  it('returns all volumes when filter is empty', () => {
    volumes.set(['vol-a', 'vol-b', 'vol-c']);
    expect(get(filteredVolumes)).toEqual(['vol-a', 'vol-b', 'vol-c']);
  });

  it('filters case-insensitively', () => {
    volumes.set(['MyVol', 'OtherVol', 'something']);
    volumeFilter.set('myvol');
    expect(get(filteredVolumes)).toEqual(['MyVol']);
  });

  it('returns empty when no match', () => {
    volumes.set(['vol-a', 'vol-b']);
    volumeFilter.set('zzz');
    expect(get(filteredVolumes)).toEqual([]);
  });

  it('subscribing reactively updates when volumeFilter changes', () => {
    volumes.set(['alpha', 'beta', 'gamma']);
    const updates: string[][] = [];
    const unsub = filteredVolumes.subscribe(v => { updates.push(v); });
    expect(updates).toHaveLength(1);
    expect(updates[0]).toEqual(['alpha', 'beta', 'gamma']);
    volumeFilter.set('a');
    expect(get(volumeFilter)).toBe('a');
    expect(updates).toHaveLength(2);
    expect(updates[1]).toEqual(['alpha', 'gamma']);
    volumeFilter.set('be');
    expect(updates).toHaveLength(3);
    expect(updates[2]).toEqual(['beta']);
    unsub();
  });

  it('inline derived store works correctly', () => {
    const testFilter = derived([volumes, volumeFilter], ([$v, $f]) =>
      $v.filter(x => x.includes($f))
    );
    volumes.set(['a', 'b', 'ab']);
    const updates: string[][] = [];
    const unsub = testFilter.subscribe(v => { updates.push(v); });
    expect(updates).toHaveLength(1);
    expect(updates[0]).toEqual(['a', 'ab']);
    volumeFilter.set('b');
    expect(updates).toHaveLength(2);
    expect(updates[1]).toEqual(['b', 'ab']);
    unsub();
  });

  it('updates reactively when volumes change', () => {
    volumeFilter.set('vol');
    let value: string[] = [];
    const unsub = filteredVolumes.subscribe(v => { value = v; });
    volumes.set(['volume-1', 'other', 'volume-2']);
    expect(value).toEqual(['volume-1', 'volume-2']);
    volumes.set(['volume-1']);
    expect(value).toEqual(['volume-1']);
    unsub();
  });
});

describe('onFilterChange', () => {
  it('sets the volume filter', () => {
    volumeFilter.set('');
    onFilterChange('test-vol');
    expect(get(volumeFilter)).toBe('test-vol');
  });
});

describe('openDeleteVolModal', () => {
  beforeEach(() => {
    selectedVolume.set('');
    deleteVolModal.set(false);
    deleteConfirmText.set('preserved');
  });

  it('opens modal and clears confirm text when volume is selected', () => {
    selectedVolume.set('my-vol');
    openDeleteVolModal();
    expect(get(deleteVolModal)).toBe(true);
    expect(get(deleteConfirmText)).toBe('');
  });

  it('does nothing when no volume is selected', () => {
    openDeleteVolModal();
    expect(get(deleteVolModal)).toBe(false);
  });
});
