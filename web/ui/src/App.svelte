<script lang="ts">
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import Banner from './components/Banner.svelte';
  import LandingPanel from './components/LandingPanel.svelte';
  import VolumeList from './components/VolumeList.svelte';
  import SnapshotTable from './components/SnapshotTable.svelte';
  import SnapshotViewer from './components/SnapshotViewer.svelte';
  import StatsGrid from './components/StatsGrid.svelte';
  import LockPanel from './components/LockPanel.svelte';
  import TroubleshootPanel from './components/TroubleshootPanel.svelte';
  import Modal from './components/Modal.svelte';
  import type { Snapshot } from './lib/types';
  import { get } from 'svelte/store';
  import { bannerText, bannerError, setBanner } from './lib/stores/banner';
  import {
    volumes, selectedVolume, volumeFilter, hostname, volumeLockInfo, volumesLoading, landingShown,
    deleteVolModal, deleteConfirmText, deleteVolLoading, filteredVolumes,
    loadVolumes, onFilterChange, openDeleteVolModal
  } from './lib/stores/volumes';
  import {
    snapshots, query, sortNewestFirst, typeFilter, hostFilter, sizes, currentSnapshot, allSnapshots,
    viewerOpen, deleteSnapModal, deletingSnap, snapDeleteInput, snapsLoading, restorePointLoading,
    sizeLoading, filteredSnapshots, sortedSnapshots, hosts, diffTargetId, diffTargetFallbackHash, restorePointID,
    onToggleSort, onSearch, onTypeFilter, onHostFilter, onAddTag, onRemoveTag, onDeleteSnapshot,
    confirmDeleteSnapshot, handleSizeLoaded
  } from './lib/stores/snapshots';
  import {
    prevStats, themeDark, loading, activeTab, lockStatus, stats, statsLoading, checking, repairing,
    toggleTheme, loadLockStatus, loadStats, handleCheck, handleRepair
  } from './lib/stores/repo';
  import {
    creatingTest, testStatus,
    onSelectVolume, onOpenViewer, onCloseViewer, confirmDeleteVolume, handleCreateTestVolume,
    switchTab, loadAll, navigateTo, syncUrl, setDiffTarget, handleRefresh
  } from './lib/stores/navigation';

  let initialSyncDone = false;

  onMount(async () => {
    const saved = localStorage.getItem('themeDark');
    if (saved !== null) {
      themeDark.set(JSON.parse(saved));
    } else if (window.matchMedia('(prefers-color-scheme: light)').matches) {
      themeDark.set(false);
    }
    if (!$themeDark) document.body.classList.add('light');
    await loadVolumes();
    if ($volumes.length === 0) {
      setBanner('No volumes found. Create one with: docker volume create --driver blt-volume-manager --name <name>');
    } else {
      setBanner('');
    }

    const params = new URLSearchParams(window.location.search);

    // Volume from path: /ui/volume/1/2/test
    let volFromUrl = '';
    const path = window.location.pathname;
    const volumePrefix = '/ui/volume/';
    if (path.startsWith(volumePrefix)) {
      volFromUrl = path.slice(volumePrefix.length);
      // Decode any percent-encoded segments
      volFromUrl = volFromUrl.split('/').map(decodeURIComponent).join('/');
    }

    if (volFromUrl && $volumes.includes(volFromUrl)) {
      await navigateTo(volFromUrl, {
        tab: params.get('tab') || undefined,
        snapshotId: params.get('snapshot') || undefined,
        diffId: params.get('diff') || undefined,
        fallbackHash: params.get('fallbackHash') || undefined,
        diffFallbackHash: params.get('diffFallbackHash') || undefined,
      });
    }
    loading.set(false);
    initialSyncDone = true;
  });

  $: if (initialSyncDone && !$loading) {
    syncUrl();
  }
</script>

<style>
  .page-shell {
    max-width: 1200px;
    margin: 0 auto;
    padding: 28px 20px 40px;
  }

  .topbar {
    display: flex;
    justify-content: space-between;
    gap: 20px;
    align-items: center;
    margin-bottom: 24px;
  }

  .topbar-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
  }

  .button-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border: 1px solid var(--border);
    border-radius: 14px;
    background: rgb(255 255 255 / 4%);
    color: var(--muted);
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }

  .button-icon:hover {
    background: rgb(255 255 255 / 8%);
    color: var(--text);
  }

  .repo-layout {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
    align-items: stretch;
  }

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

  .tab:hover {
    color: var(--text);
  }

  .tab.tab-active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }

  .tab-panel {
    display: block;
  }

  .viewer-skeleton :global(.skeleton) {
    display: block;
  }

  @media (width <= 900px) {
    .topbar {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>

<div class="page-shell">
  <header class="topbar">
    <h1>BLT Volume Manager</h1>
    <div class="topbar-actions">
      <button class="button-icon" title="Refresh" on:click={handleRefresh}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="23 4 23 10 17 10"></polyline>
          <polyline points="1 20 1 14 7 14"></polyline>
          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
        </svg>
      </button>
      <button class="button-icon" title="Toggle light/dark mode" on:click={toggleTheme}>
        {#if $themeDark}
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="5"></circle>
          <line x1="12" y1="1" x2="12" y2="3"></line>
          <line x1="12" y1="21" x2="12" y2="23"></line>
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
          <line x1="1" y1="12" x2="3" y2="12"></line>
          <line x1="21" y1="12" x2="23" y2="12"></line>
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
        </svg>
        {:else}
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
        </svg>
        {/if}
      </button>
    </div>
  </header>

  <Banner bannerText={$bannerText} bannerError={$bannerError} onClose={() => setBanner('')} />

  {#if $landingShown}
    {#if $volumes.length === 0 && !$volumesLoading}
      <LandingPanel onCreateTestVolume={handleCreateTestVolume} creatingTest={$creatingTest} testStatus={$testStatus} />
    {:else}
      <VolumeList
        volumes={$filteredVolumes}
        loading={$volumesLoading}
        onSelect={onSelectVolume}
        filter={$volumeFilter}
        onFilterChange={onFilterChange}
        volumeLockInfo={$volumeLockInfo}
      />
    {/if}
  {/if}

  {#if $selectedVolume}
    <div id="volumeView">
      <div class="tab-bar">
        <button class="tab" on:click={() => onSelectVolume('')} title="Back to volumes">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="vertical-align:middle;">
            <line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/>
          </svg>
        </button>
        <button class="tab" class:tab-active={$activeTab === 'snapshots'} on:click={() => switchTab('snapshots')}>Snapshots</button>
        <button class="tab" class:tab-active={$activeTab === 'repo'} on:click={() => switchTab('repo')}>Repo</button>
      </div>

      {#if $activeTab === 'snapshots'}
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

          <SnapshotTable
              snapshots={$sortedSnapshots}
              sizes={$sizes}
              selectedVolume={$selectedVolume}
              sortNewestFirst={$sortNewestFirst}
              query={$query}
              typeFilter={$typeFilter}
              hostFilter={$hostFilter}
              hosts={$hosts}
              loading={$snapsLoading}
              restorePointLoading={$restorePointLoading}
              sizeLoading={$sizeLoading}
              restorePointID={$restorePointID}
              onSearch={onSearch}
              onToggleSort={onToggleSort}
              onTypeFilter={onTypeFilter}
              onHostFilter={onHostFilter}
              onOpenViewer={onOpenViewer}
              onAddTag={onAddTag}
              onRemoveTag={onRemoveTag}
              onDeleteSnapshot={onDeleteSnapshot}
              onSizeLoaded={handleSizeLoaded}
            />
        </div>
      {:else}
        <div class="tab-panel">
          <div class="repo-layout">
            <StatsGrid stats={$stats} loading={$statsLoading} />
            <LockPanel
              lockStatus={$lockStatus}
              volume={$selectedVolume}
              hostname={$hostname}
              onLockCreated={() => loadLockStatus()}
              onLocksDeleted={() => loadLockStatus()}
            />
            <TroubleshootPanel
              checking={$checking}
              repairing={$repairing}
              onCheck={handleCheck}
              onRepair={handleRepair}
              onDeleteVolume={openDeleteVolModal}
            />
          </div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<Modal show={$deleteVolModal} onClose={() => $deleteVolModal = false}>
  <h3 style="margin:0 0 12px;color:var(--red);">Delete volume</h3>
  <p style="margin:0 0 8px;color:var(--muted);font-size:0.9rem;">
    This will permanently delete the volume, all its snapshots, backups, and locks from S3.
  </p>
  <p style="margin:0 0 16px;color:var(--yellow);font-size:0.9rem;">
    Make sure no other hosts are still using this volume before proceeding.
  </p>
  <p style="margin:0 0 8px;font-size:0.85rem;">
    Type <strong>{$selectedVolume}</strong> to confirm:
  </p>
  <input class="input" type="text" placeholder={$selectedVolume}
    style="width:100%;box-sizing:border-box;margin-bottom:16px;"
    bind:value={$deleteConfirmText} />
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <button class="button button-secondary" on:click={() => $deleteVolModal = false}>Cancel</button>
    <button class="button" style="background:var(--red);color:#fff;"
      disabled={$deleteConfirmText !== $selectedVolume || $deleteVolLoading}
      on:click={confirmDeleteVolume}>
      {$deleteVolLoading ? 'Deleting...' : 'Delete'}
    </button>
  </div>
</Modal>

<Modal show={$deleteSnapModal} onClose={() => $deleteSnapModal = false}>
  <h3 style="margin:0 0 12px;color:var(--red);">Delete snapshot</h3>
  {#if $deletingSnap}
  <div style="margin-bottom:16px;font-size:0.85rem;">
    <div><strong>Hostname:</strong> {$deletingSnap.hostname}</div>
    <div><strong>Date:</strong> {new Date($deletingSnap.time).toLocaleString()}</div>
    <div><strong>Tags:</strong> {$deletingSnap.tags.join(', ') || '—'}</div>
  </div>
  {/if}
  <p style="margin:0 0 8px;font-size:0.85rem;">Type <strong>delete</strong> to confirm:</p>
  <input class="input" type="text" placeholder='Type "delete" to confirm'
    style="width:100%;box-sizing:border-box;margin-bottom:16px;"
    bind:value={$snapDeleteInput} />
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <button class="button button-secondary" on:click={() => $deleteSnapModal = false}>Cancel</button>
    <button class="button" style="background:var(--red);color:#fff;"
      disabled={$snapDeleteInput !== 'delete'}
      on:click={confirmDeleteSnapshot}>Delete</button>
  </div>
</Modal>
