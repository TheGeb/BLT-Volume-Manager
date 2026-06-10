<script lang="ts">
  import { slide } from 'svelte/transition';
  import SnapshotTable from './SnapshotTable.svelte';
  import SnapshotViewer from './SnapshotViewer.svelte';
  import SnapshotSearch from './SnapshotSearch.svelte';
  import Modal from '../../components/Modal.svelte';
  import SnapshotPicker from '../../components/SnapshotPicker.svelte';
  import { onSelectVolume, onOpenViewer, onCloseViewer, switchTab, setDiffTarget, syncUrl } from '$lib/stores/navigation';
  import {
    sizes, currentSnapshot,
    viewerOpen, snapsLoading, restorePointLoading,
    sizeLoading, displayedSnapshots, diffTargetId, diffTargetFallbackHash, restorePointID,
    selectedForDeletion, onAddTag, onRemoveTag,
    toggleForDeletion, openBulkDeleteModal, handleSizeLoaded,
    pageSize, currentPage, hasMore, totalCount, goToPage, setPageSize,
    sortNewestFirst, onToggleSort
  } from '$lib/stores/snapshots';
  import { activeTab } from '$lib/stores/repo';
  import { selectedVolume } from '$lib/stores/volumes';

  let pickerDialogOpen = $state(false);

  function handleOpenViewer(sn: import('$lib/types').Snapshot) {
    onOpenViewer(sn);
  }

  function handleDiffTargetPicked(targetId: string) {
    if (!targetId) return;
    diffTargetId.set(targetId);
    setDiffTarget(targetId);
    pickerDialogOpen = false;
  }
</script>

<div id="volume-view">
  <div class="tab-bar">
    <button class="tab" onclick={() => onSelectVolume('')} title="Back to volumes">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:middle;">
        <line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/>
      </svg>
    </button>
    <button class="tab" class:tab-active={$activeTab === 'snapshots'} onclick={() => switchTab('snapshots')}>Volume</button>
    <button class="tab" class:tab-active={$activeTab === 'repo'} onclick={() => switchTab('repo')}>Repo</button>
  </div>

  <div class="tab-panel">
    {#if $viewerOpen}
      <div transition:slide>
        {#if $currentSnapshot}
          <SnapshotViewer
            snapshot={$currentSnapshot}
            onClose={onCloseViewer}
            initialDiffTarget={$diffTargetId}
            onDiffChange={setDiffTarget}
            onOpenDiffPicker={() => pickerDialogOpen = true}
            onSwapDiff={(snap, newDiffId, newSnapshotHash) => {
              if (newSnapshotHash) snap.fallbackHash = newSnapshotHash;
              currentSnapshot.set(snap);
              diffTargetId.set(newDiffId);
              syncUrl();
            }}
          />
        {:else}
          <div class="panel viewer-skeleton">
            <div class="row gap" style="margin-bottom:12px;">
              <div class="skeleton" style="height:22px;width:200px;border-radius:6px;"></div>
              <div class="skeleton" style="height:32px;width:60px;border-radius:8px;margin-left:auto;"></div>
            </div>
            <div class="skeleton" style="height:32px;width:300px;border-radius:8px;margin-bottom:12px;"></div>
            <div style="display:flex;gap:0;height:400px;">
              <div class="skeleton" style="flex:0 0 300px;border-radius:12px;"></div>
              <div style="width:12px;flex-shrink:0;"></div>
              <div class="skeleton" style="flex:1;border-radius:12px;"></div>
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <Modal show={pickerDialogOpen} onClose={() => pickerDialogOpen = false} wide>
      <SnapshotPicker
        mode="single"
        value=""
        onValueChange={(v: string | string[]) => handleDiffTargetPicked(v as string)}
        disabledId={$currentSnapshot?.id ?? ''}
        volume={$selectedVolume}
        restorePointID={$restorePointID}
      />
    </Modal>

    <SnapshotSearch />
    <SnapshotTable
      snapshots={$displayedSnapshots}
      sizes={$sizes}
      selectedVolume={$selectedVolume}
      loading={$snapsLoading}
      restorePointLoading={$restorePointLoading}
      sizeLoading={$sizeLoading}
      restorePointID={$restorePointID}
      selectedForDeletion={$selectedForDeletion}
      onOpenViewer={handleOpenViewer}
      onAddTag={onAddTag}
      onRemoveTag={onRemoveTag}
      onToggleDeletion={toggleForDeletion}
      onDeleteSelected={openBulkDeleteModal}
      onSizeLoaded={handleSizeLoaded}
      page={$currentPage}
      pageSize={$pageSize}
      hasMore={$hasMore}
      onGoToPage={goToPage}
      onSetPageSize={setPageSize}
      sortNewestFirst={$sortNewestFirst}
      onToggleSort={onToggleSort}
    />
  </div>
</div>
