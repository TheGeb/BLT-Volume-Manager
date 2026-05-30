import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { themeDark, activeTab, doSwitchTab } from './repo';
import { selectedVolume } from './volumes';
import { snapshots } from './snapshots';

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
