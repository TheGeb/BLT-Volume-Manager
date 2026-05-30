import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { volumes, volumeFilter, selectedVolume, deleteVolModal, deleteConfirmText, filteredVolumes, filterVolumes, onFilterChange, openDeleteVolModal } from './volumes';

describe('filterVolumes', () => {
  it('returns all when filter is empty', () => {
    expect(filterVolumes(['vol-a', 'vol-b'], '')).toEqual(['vol-a', 'vol-b']);
  });

  it('filters case-insensitively', () => {
    expect(filterVolumes(['MyVol', 'OtherVol'], 'myvol')).toEqual(['MyVol']);
  });

  it('returns empty when no match', () => {
    expect(filterVolumes(['vol-a', 'vol-b'], 'zzz')).toEqual([]);
  });

  it('matches partial strings', () => {
    expect(filterVolumes(['alpha', 'beta', 'gamma'], 'ph')).toEqual(['alpha']);
  });
});

describe('filteredVolumes derived store', () => {
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
