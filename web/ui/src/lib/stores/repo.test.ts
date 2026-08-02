import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { themeDark, activeTab, doSwitchTab, ownerStatus, loadOwnerStatus } from './repo';
import { selectedVolume } from './volumes';
import { snapshots } from './snapshots';
import * as api from '../api';

vi.mock('../api', () => ({
  fetchOwnerStatus: vi.fn(),
  fetchStats: vi.fn(),
  checkRepo: vi.fn(),
  repairRepo: vi.fn(),
  fetchDevMode: vi.fn(),
}));

describe('themeDark', () => {
  beforeEach(() => {
    themeDark.set(true);
  });

  it('starts as true', () => {
    expect(get(themeDark)).toBe(true);
  });
});

describe('activeTab', () => {
  beforeEach(() => {
    activeTab.set('snapshots');
  });

  it('starts as snapshots', () => {
    expect(get(activeTab)).toBe('snapshots');
  });
});

describe('doSwitchTab', () => {
  beforeEach(() => {
    activeTab.set('snapshots');
    selectedVolume.set('');
    snapshots.set([]);
  });

  it('updates the active tab', () => {
    doSwitchTab('repo');
    expect(get(activeTab)).toBe('repo');
    doSwitchTab('snapshots');
    expect(get(activeTab)).toBe('snapshots');
  });

  it('does not load snapshots when switching away from snapshots tab', () => {
    snapshots.set([{ id: '1', short_id: 'a', time: '', tags: [], paths: [], hostname: 'h' }]);
    doSwitchTab('repo');
    expect(get(activeTab)).toBe('repo');
  });
});

describe('loadOwnerStatus', () => {
  beforeEach(() => {
    selectedVolume.set('test-vol');
    ownerStatus.set({ volume: 'ignored', owner: 'prev' });
    vi.clearAllMocks();
  });

  it('sets null on fetch failure, not empty owner', async () => {
    vi.mocked(api.fetchOwnerStatus).mockRejectedValue(new Error('network error'));
    await loadOwnerStatus();
    expect(get(ownerStatus)).toBeNull();
  });

  it('sets the owner status on success', async () => {
    vi.mocked(api.fetchOwnerStatus).mockResolvedValue({ volume: 'test-vol', owner: 'alice' });
    await loadOwnerStatus();
    expect(get(ownerStatus)).toEqual({ volume: 'test-vol', owner: 'alice' });
  });
});
