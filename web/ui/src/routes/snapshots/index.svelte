<script lang="ts">
  import { slide } from 'svelte/transition';
  import SnapshotTable from './SnapshotTable.svelte';
  import SnapshotViewer from './SnapshotViewer.svelte';
  import SnapshotSearch from './SnapshotSearch.svelte';
  import { onSelectVolume, onOpenViewer, onCloseViewer, switchTab, setDiffTarget, syncUrl } from '$lib/stores/navigation';
  import {
    sortNewestFirst, sizes, currentSnapshot, allSnapshots,
    viewerOpen, snapsLoading, restorePointLoading,
    sizeLoading, displayedSnapshots, diffTargetId, diffTargetFallbackHash, restorePointID,
    selectedForDeletion, onToggleSort, onAddTag, onRemoveTag,
    toggleForDeletion, openBulkDeleteModal, handleSizeLoaded, timeFrom, timeTo, onTimeFilter,
    timeOfDayFrom, timeOfDayTo, onTimeOfDayFilter,
    typeFilter, hostFilter, hosts, onTypeFilter, onHostFilter,
    pageSize, currentPage, hasMore, totalCount, goToPage, setPageSize
  } from '$lib/stores/snapshots';
  import { activeTab } from '$lib/stores/repo';
  import { selectedVolume } from '$lib/stores/volumes';
</script>

<div id="volumeView">
  <div class="tab-bar">
    <button class="tab" on:click={() => onSelectVolume('')} title="Back to volumes">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:middle;">
        <line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/>
      </svg>
    </button>
    <button class="tab" class:tab-active={$activeTab === 'snapshots'} on:click={() => switchTab('snapshots')}>Volume</button>
    <button class="tab" class:tab-active={$activeTab === 'repo'} on:click={() => switchTab('repo')}>Repo</button>
  </div>

  <div class="tab-panel">
    {#if $viewerOpen}
      <div transition:slide>
        {#if $currentSnapshot}
          <SnapshotViewer
            snapshot={$currentSnapshot}
            allSnapshots={$allSnapshots}
            onClose={onCloseViewer}
            initialDiffTarget={$diffTargetId}
            onDiffChange={setDiffTarget}
            onSwapDiff={(newSnapshotId, newDiffId, newSnapshotHash, newDiffHash) => {
              const newSnap = $allSnapshots.find(s => s.id === newSnapshotId);
              if (!newSnap) return;
              if (newSnapshotHash) newSnap.fallbackHash = newSnapshotHash;
              currentSnapshot.set(newSnap);
              diffTargetId.set(newDiffId);
              if (newDiffHash) diffTargetFallbackHash.set(newDiffHash);
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

    <SnapshotSearch />
    <SnapshotTable
      snapshots={$displayedSnapshots}
      sizes={$sizes}
      selectedVolume={$selectedVolume}
      sortNewestFirst={$sortNewestFirst}
      typeFilter={$typeFilter}
      hostFilter={$hostFilter}
      hosts={$hosts}
      loading={$snapsLoading}
      restorePointLoading={$restorePointLoading}
      sizeLoading={$sizeLoading}
      restorePointID={$restorePointID}
      selectedForDeletion={$selectedForDeletion}
      onToggleSort={onToggleSort}
      onTypeFilter={onTypeFilter}
      onHostFilter={onHostFilter}
      onOpenViewer={onOpenViewer}
      onAddTag={onAddTag}
      onRemoveTag={onRemoveTag}
      onToggleDeletion={toggleForDeletion}
      onDeleteSelected={openBulkDeleteModal}
      onSizeLoaded={handleSizeLoaded}
      timeFrom={$timeFrom}
      timeTo={$timeTo}
      onTimeFilter={onTimeFilter}
      timeOfDayFrom={$timeOfDayFrom}
      timeOfDayTo={$timeOfDayTo}
      onTimeOfDayFilter={onTimeOfDayFilter}
      page={$currentPage}
      pageSize={$pageSize}
      hasMore={$hasMore}
      onGoToPage={goToPage}
      onSetPageSize={setPageSize}
    />
  </div>
</div>

<style>
  .tab-bar {
    display: flex;
    gap: 0;
    margin-bottom: 20px;
    border-bottom: 2px solid var(--border);
  }

  .tab {
    background: none;
    border: none;
    border-radius: 0;
    padding: 12px 28px;
    font-size: 1rem;
    font-weight: 500;
    font-family: inherit;
    color: var(--muted);
    cursor: pointer;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
    transition: color 0.15s, border-color 0.15s;
    letter-spacing: 0.02em;
    appearance: none;
    outline: none;
  }
  .tab:hover { color: var(--text); }

  .tab.tab-active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .tab-panel { display: block; }
  .viewer-skeleton :global(.skeleton) { display: block; }
</style>
