<script lang="ts">
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { Button, RadioGroup } from 'bits-ui';
  import { Toaster } from 'svelte-sonner';
  import VolumeList from './routes/volumes/index.svelte';
  import SnapshotsPage from './routes/snapshots/index.svelte';
  import RepoPage from './routes/repo/index.svelte';
  import DevTools from './components/DevTools.svelte';
  import Modal from './components/Modal.svelte';
  import { get } from 'svelte/store';
  import { showToast } from '$lib/stores/toast';
  import {
    volumes, selectedVolume, volumeFilter, volumeLockInfo, volumesLoading,
    deleteVolModal, deleteConfirmText, deleteVolLoading, filteredVolumes,
    copyVolModal, renameVolModal, copyRenameSource, copyRenameTarget, copyRenameLoading, copyRenameError,
    copySnapshots, copySnapshotsLoading, copySnapshotMode, copySelectedSnapshotIds, copyRestorePointID,
    loadVolumes, onFilterChange,
    confirmCopyVolume, confirmRenameVolume
  } from '$lib/stores/volumes';
  import {
    deleteSnapModal, snapDeleteInput, selectedDeletionCount,
    confirmDeleteSnapshot
  } from '$lib/stores/snapshots';
  import { themeDark, loading, activeTab, devMode, toggleTheme, loadDevMode } from '$lib/stores/repo';
  import {
    creatingTest, testStatus,
    onSelectVolume, confirmDeleteVolume, handleCreateTestVolume,
    navigateTo, handleRefresh, syncUrl
  } from '$lib/stores/navigation';

  let initialSyncDone = false;
  let refreshing = false;

  async function doRefresh() {
    refreshing = true;
    try { await Promise.all([handleRefresh(), new Promise(r => setTimeout(r, 300))]); } finally { refreshing = false; }
  }

  onMount(async () => {
    loadDevMode();
    const saved = localStorage.getItem('themeDark');
    if (saved !== null) {
      themeDark.set(JSON.parse(saved));
    } else if (window.matchMedia('(prefers-color-scheme: light)').matches) {
      themeDark.set(false);
    }
    if (!$themeDark) document.body.classList.add('light');
    await loadVolumes();
    if ($volumes.length === 0) {
      showToast('No volumes found. Create one with: docker volume create --driver blt-volume-manager --name <name>');
    } else {
      showToast('');
    }

    const params = new URLSearchParams(window.location.search);

    let volFromUrl = '';
    let tabFromUrl: string | undefined;
    const path = window.location.pathname;

    if (path.startsWith('/ui/snapshots/')) {
      tabFromUrl = 'snapshots';
      volFromUrl = path.slice('/ui/snapshots/'.length).split('/').map(decodeURIComponent).join('/');
    } else if (path.startsWith('/ui/repo/')) {
      tabFromUrl = 'repo';
      volFromUrl = path.slice('/ui/repo/'.length).split('/').map(decodeURIComponent).join('/');
    }

    if (volFromUrl && $volumes.includes(volFromUrl)) {
      const navOpts: { tab?: string; snapshotId?: string; diffId?: string; fallbackHash?: string; diffFallbackHash?: string } = {};
      const tab = tabFromUrl ?? params.get('tab');
      const snapshotId = params.get('snapshot');
      const diffId = params.get('diff');
      const fallbackHash = params.get('fallbackHash');
      const diffFallbackHash = params.get('diffFallbackHash');
      if (tab) navOpts.tab = tab;
      if (snapshotId) navOpts.snapshotId = snapshotId;
      if (diffId) navOpts.diffId = diffId;
      if (fallbackHash) navOpts.fallbackHash = fallbackHash;
      if (diffFallbackHash) navOpts.diffFallbackHash = diffFallbackHash;
      await navigateTo(volFromUrl, navOpts);
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
  .button-icon:disabled { opacity: 0.5; cursor: default; }

  @media (width <= 900px) {
    .topbar {
      flex-direction: column;
      align-items: flex-start;
    }
  }

  .snap-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    font-size: 0.78rem;
    cursor: pointer;
    border-bottom: 1px solid var(--border);
    transition: background 0.1s;
    white-space: nowrap;
  }
  .snap-row:last-child { border-bottom: none; }
  .snap-row:hover { background: rgb(255 255 255 / 4%); }
  .snap-row.selected { background: color-mix(in srgb, var(--accent) 10%, transparent); }
  .snap-row input { margin: 0; flex-shrink: 0; }

  .snap-short-id {
    font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace;
    color: var(--accent);
    flex-shrink: 0;
  }
  .snap-date { color: var(--muted); overflow: hidden; text-overflow: ellipsis; }
  .snap-tags { color: var(--muted); overflow: hidden; text-overflow: ellipsis; }

  .snap-rp-badge {
    background: color-mix(in srgb, var(--accent) 20%, transparent);
    color: var(--accent);
    font-size: 0.65rem;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 4px;
    flex-shrink: 0;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }

  .snap-tag-badge {
    background: color-mix(in srgb, var(--muted) 20%, transparent);
    color: var(--muted);
    font-size: 0.65rem;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 4px;
    flex-shrink: 0;
    text-transform: capitalize;
  }
  .snap-row.restore-point { background: color-mix(in srgb, var(--accent) 4%, transparent); }
  .snap-row.restore-point:hover { background: color-mix(in srgb, var(--accent) 8%, transparent); }
  .snap-host { color: var(--muted); margin-left: auto; flex-shrink: 0; }

  :global(.radio-item) {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    font-size: 0.85rem;
    background: none;
    border: none;
    color: var(--text);
    cursor: pointer;
    font-family: inherit;
    outline: none;
    text-align: left;
  }

  :global(.radio-item::before) {
    content: '';
    display: inline-block;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    border: 2px solid var(--border);
    flex-shrink: 0;
    transition: border-color 0.15s, background 0.15s;
    box-sizing: border-box;
  }

  :global(.radio-item[data-state="checked"]::before) {
    border-color: var(--accent);
    background: var(--accent);
    box-shadow: inset 0 0 0 3px var(--surface);
  }

</style>

<div class="page-shell">
  <header class="topbar">
    <h1>BLT Volume Manager</h1>
    <div class="topbar-actions">
      {#if $devMode}
        <DevTools volume={$selectedVolume} onAction={doRefresh} />
      {/if}
      <button class="button-icon" title="Refresh" on:click={doRefresh} disabled={refreshing}>
        {#if refreshing}
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" class="spin" style="vertical-align:middle;">
            <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
            <path d="M12 2a10 10 0 0 1 10 10"/>
          </svg>
        {:else}
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="23 4 23 10 17 10"></polyline>
          <polyline points="1 20 1 14 7 14"></polyline>
          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
        </svg>
        {/if}
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

  <Toaster position="bottom-right" visibleToasts={3} toastOptions={{ style: 'padding:18px 28px;font-size:1rem;font-weight:500;border-radius:16px;background:var(--surface);color:var(--text);border:1px solid var(--border);box-shadow:0 6px 24px rgb(0 0 0 / 30%);' }} />

  {#if !$selectedVolume}
    <VolumeList
      volumes={$filteredVolumes}
      loading={$volumesLoading}
      onSelect={onSelectVolume}
      filter={$volumeFilter}
      onFilterChange={onFilterChange}
      volumeLockInfo={$volumeLockInfo}
      onCreateTestVolume={handleCreateTestVolume}
      creatingTest={$creatingTest}
      testStatus={$testStatus}
    />
  {/if}

  {#if $selectedVolume}
    {#if $activeTab === 'repo'}
      <RepoPage />
    {:else}
      <SnapshotsPage />
    {/if}
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
    <Button.Root class="button button-secondary" onclick={() => $deleteVolModal = false}>Cancel</Button.Root>
    <Button.Root class="button button-destructive"
      disabled={$deleteConfirmText !== $selectedVolume || $deleteVolLoading}
      onclick={confirmDeleteVolume}>
      {$deleteVolLoading ? 'Deleting...' : 'Delete'}
    </Button.Root>
  </div>
</Modal>

<Modal show={$deleteSnapModal} onClose={() => $deleteSnapModal = false}>
  <h3 style="margin:0 0 12px;color:var(--red);">Delete snapshot{$selectedDeletionCount !== 1 ? 's' : ''}</h3>
  <div style="margin-bottom:16px;font-size:0.85rem;color:var(--muted);">
    {$selectedDeletionCount} snapshot{$selectedDeletionCount !== 1 ? 's' : ''} selected for deletion.
  </div>
  <input class="input" type="text" placeholder='Type "delete" to confirm'
    style="width:100%;box-sizing:border-box;margin-bottom:16px;"
    bind:value={$snapDeleteInput} />
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <Button.Root class="button button-secondary" onclick={() => $deleteSnapModal = false}>Cancel</Button.Root>
    <Button.Root class="button button-destructive"
      disabled={$snapDeleteInput !== 'delete'}
      onclick={confirmDeleteSnapshot}>Delete</Button.Root>
  </div>
</Modal>

<Modal show={$copyVolModal} onClose={() => $copyVolModal = false} wide={true}>
  <h3 style="margin:0 0 12px;">Copy volume</h3>
  <p style="margin:0 0 8px;color:var(--muted);font-size:0.9rem;">
    Copy snapshots from <strong>{$copyRenameSource}</strong> to a new volume.
  </p>
  <p style="margin:0 0 8px;font-size:0.85rem;">
    New volume name:
  </p>
  <input class="input" type="text" placeholder="Enter new volume name"
    style="width:100%;box-sizing:border-box;margin-bottom:12px;"
    bind:value={$copyRenameTarget} />

  <fieldset style="border:none;padding:0;margin:0 0 8px;">
    <legend style="font-size:0.85rem;margin-bottom:6px;">Snapshots to copy:</legend>
    <RadioGroup.Root bind:value={$copySnapshotMode}>
      <RadioGroup.Item value="all" class="radio-item">
        All snapshots
      </RadioGroup.Item>
      <RadioGroup.Item value="specific" class="radio-item">
        Select snapshots...
      </RadioGroup.Item>
    </RadioGroup.Root>
  </fieldset>

  {#if $copySnapshotMode === 'specific'}
    <div style="margin-bottom:8px;" transition:slide>
      {#if $copySnapshotsLoading}
        <p style="color:var(--muted);font-size:0.8rem;text-align:center;padding:12px;">Loading snapshots…</p>
      {:else if $copySnapshots.length === 0}
        <p style="color:var(--muted);font-size:0.8rem;text-align:center;padding:12px;">No snapshots found</p>
      {:else}
        <div style="max-height:200px;overflow-y:auto;border:1px solid var(--border);border-radius:8px;">
          {#each $copySnapshots as sn (sn.id)}
            <label class="snap-row" class:selected={$copySelectedSnapshotIds.includes(sn.id)} class:restore-point={sn.id === $copyRestorePointID || sn.short_id === $copyRestorePointID}>
              <input type="checkbox" checked={$copySelectedSnapshotIds.includes(sn.id)}
                on:change={(e) => {
                  const checked = e.currentTarget.checked;
                  $copySelectedSnapshotIds = checked
                    ? [...$copySelectedSnapshotIds, sn.id]
                    : $copySelectedSnapshotIds.filter(id => id !== sn.id);
                }} />
              <span class="snap-short-id">{sn.short_id.slice(0, 8)}</span>
              <span class="snap-date">{new Date(sn.time).toLocaleDateString()} {new Date(sn.time).toLocaleTimeString()}</span>
              {#each sn.tags.filter(t => t !== 'restore-point') as tag (tag)}
                {#if tag === 'hot' || tag === 'cold'}
                  <span class="snap-tag-badge">{tag}</span>
                {:else}
                  <span class="snap-tags">{tag}</span>
                {/if}
              {/each}
              {#if sn.id === $copyRestorePointID || sn.short_id === $copyRestorePointID}
                <span class="snap-rp-badge" title="Restore point">Restore Point</span>
              {/if}
              <span class="snap-host">{sn.hostname}</span>
            </label>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  {#if $copyRenameError}
    <p style="margin:0 0 8px;color:var(--red);font-size:0.85rem;">{$copyRenameError}</p>
  {/if}
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <Button.Root class="button button-secondary" onclick={() => $copyVolModal = false}>Cancel</Button.Root>
    <Button.Root class="button" disabled={!$copyRenameTarget || $copyRenameLoading || ($copySnapshotMode === 'specific' && $copySelectedSnapshotIds.length === 0)}
      onclick={confirmCopyVolume}>
      {$copyRenameLoading ? 'Copying...' : 'Copy'}
    </Button.Root>
  </div>
</Modal>

<Modal show={$renameVolModal} onClose={() => $renameVolModal = false}>
  <h3 style="margin:0 0 12px;">Rename volume</h3>
  <p style="margin:0 0 8px;color:var(--muted);font-size:0.9rem;">
    Rename <strong>{$copyRenameSource}</strong> to a new name.
    {#if $volumeLockInfo[$copyRenameSource]?.locked}
      <span style="color:var(--red);display:block;margin-top:6px;">
        This volume is locked and cannot be renamed. Unlock it first.
      </span>
    {/if}
  </p>
  <p style="margin:0 0 8px;font-size:0.85rem;">
    New volume name:
  </p>
  <input class="input" type="text" placeholder="Enter new volume name"
    style="width:100%;box-sizing:border-box;margin-bottom:8px;"
    bind:value={$copyRenameTarget} />
  {#if $copyRenameError}
    <p style="margin:0 0 8px;color:var(--red);font-size:0.85rem;">{$copyRenameError}</p>
  {/if}
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <Button.Root class="button button-secondary" onclick={() => $renameVolModal = false}>Cancel</Button.Root>
    <Button.Root class="button" disabled={!$copyRenameTarget || $copyRenameLoading || $volumeLockInfo[$copyRenameSource]?.locked}
      onclick={confirmRenameVolume}>
      {$copyRenameLoading ? 'Renaming...' : 'Rename'}
    </Button.Root>
  </div>
</Modal>
