import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { selectedVolume, landingShown, deleteVolModal, deleteConfirmText, deleteVolLoading } from './volumes';
import { activeTab, themeDark, lockStatus, stats, statsLoading, checking, repairing } from './repo';
import { currentSnapshot, viewerOpen, diffTargetId, diffTargetFallbackHash, sizes, deleteSnapModal, snapshots } from './snapshots';
import { creatingTest, testStatus, onSelectVolume, onCloseViewer, confirmDeleteVolume, syncUrl } from './navigation';

describe('onSelectVolume', () => {
  beforeEach(() => {
    selectedVolume.set('');
    landingShown.set(true);
    currentSnapshot.set(null);
    viewerOpen.set(false);
    sizes.set({});
    deleteVolModal.set(false);
    deleteSnapModal.set(false);
    diffTargetId.set('');
    diffTargetFallbackHash.set('');
  });

  it('clears selection when re-selecting the same volume', () => {
    selectedVolume.set('my-vol');
    onSelectVolume('my-vol');
    expect(get(selectedVolume)).toBe('');
    expect(get(landingShown)).toBe(true);
  });

  it('sets the selected volume', () => {
    onSelectVolume('new-vol');
    expect(get(selectedVolume)).toBe('new-vol');
  });
});

describe('onCloseViewer', () => {
  beforeEach(() => {
    currentSnapshot.set({ id: '1', short_id: 'a', time: '', tags: [], paths: [], hostname: '' });
    viewerOpen.set(true);
    diffTargetId.set('target');
    diffTargetFallbackHash.set('hash');
  });

  it('closes viewer and clears related state', () => {
    onCloseViewer();
    expect(get(viewerOpen)).toBe(false);
    expect(get(currentSnapshot)).toBe(null);
    expect(get(diffTargetId)).toBe('');
    expect(get(diffTargetFallbackHash)).toBe('');
  });
});

describe('confirmDeleteVolume', () => {
  beforeEach(() => {
    selectedVolume.set('my-vol');
    deleteConfirmText.set('my-vol');
    deleteVolLoading.set(false);
    deleteVolModal.set(true);
    landingShown.set(false);
    currentSnapshot.set({ id: '1', short_id: 'a', time: '', tags: [], paths: [], hostname: '' });
    viewerOpen.set(true);
    sizes.set({ test: '1 KiB' });
  });

  it('does nothing when confirm text does not match volume name', async () => {
    deleteConfirmText.set('wrong');
    await confirmDeleteVolume();
    expect(get(deleteVolModal)).toBe(true);
  });

  it('does nothing when no volume is selected', async () => {
    selectedVolume.set('');
    await confirmDeleteVolume();
    expect(get(deleteVolLoading)).toBe(false);
  });
});
