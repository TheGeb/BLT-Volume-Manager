<script lang="ts">
  import { onMount } from 'svelte';
  import { slide } from 'svelte/transition';
  import { Button, RadioGroup } from 'bits-ui';
  import { Toaster } from 'svelte-sonner';
  import VolumesPage from './routes/volumes/VolumesPage.svelte';
  import SnapshotsPage from './routes/snapshots/SnapshotsPage.svelte';
  import RepoPage from './routes/repo/RepoPage.svelte';
  import DevTools from './components/DevTools.svelte';
  import Modal from './components/Modal.svelte';
  import SnapshotPicker from './components/SnapshotPicker.svelte';
  import { get } from 'svelte/store';
  import { showToast } from '$lib/stores/toast';
  import {
    volumes, selectedVolume, volumeLockInfo, volumesLoading,
    deleteVolModal, deleteConfirmText, deleteVolLoading, filteredVolumes,
    copyVolModal, renameVolModal, copyRenameSource, copyRenameTarget, copyRenameLoading, copyRenameError,
    copySnapshots, copySnapshotsLoading, copySnapshotMode, copySelectedSnapshotIds, copyRestorePointID,
    loadVolumes,
    confirmCopyVolume, confirmRenameVolume
  } from '$lib/stores/volumes';
  import {
    deleteSnapModal, snapDeleteInput, selectedDeletionCount,
    confirmDeleteSnapshot
  } from '$lib/stores/snapshots';
  import { themeDark, loading, activeTab, devMode, toggleTheme, loadDevMode, currentAccent, setAccentColor, accentColors } from '$lib/stores/repo';
  import {
    creatingTest, testStatus,
    onSelectVolume, confirmDeleteVolume, handleCreateTestVolume,
    navigateTo, handleRefresh, syncUrl
  } from '$lib/stores/navigation';

  let initialSyncDone = false;
  let refreshing = false;
  let showColorPicker = false;
  let colorPickerEl: HTMLDivElement;

  function handleWindowClick(e: MouseEvent) {
    if (showColorPicker && colorPickerEl && !colorPickerEl.contains(e.target as Node)) {
      showColorPicker = false;
    }
  }

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
    const savedAccent = localStorage.getItem('accentColor');
    if (savedAccent) {
      setAccentColor(savedAccent);
    } else {
      setAccentColor('purple');
    }
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
      const navOpts: { tab?: string; version?: string; diffVersion?: string; snapshotId?: string; diffId?: string } = {};
      const tab = tabFromUrl ?? params.get('tab');
      if (tab) navOpts.tab = tab;
      const version = params.get('version');
      const diffVersion = params.get('diffVersion');
      const snapshotId = params.get('snapshot');
      const diffId = params.get('diff');
      if (version) navOpts.version = version;
      else if (snapshotId) navOpts.snapshotId = snapshotId;
      if (diffVersion) navOpts.diffVersion = diffVersion;
      else if (diffId) navOpts.diffId = diffId;
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

  .color-picker-wrapper {
    position: relative;
  }

  .color-picker-popover {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 16px;
    box-shadow: var(--shadow);
    padding: 14px;
    z-index: 100;
  }

  .color-grid {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 8px;
  }

  .color-swatch {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border-radius: 10px;
    border: 2px solid transparent;
    cursor: pointer;
    transition: transform 0.15s, border-color 0.15s;
    padding: 0;
    outline: none;
    box-sizing: border-box;
  }

  .color-swatch:hover {
    transform: scale(1.2);
  }

  .color-swatch.active {
    border-color: var(--text);
    box-shadow: 0 0 0 1px var(--surface);
  }

  .color-label {
    font-size: 14px;
    font-weight: 700;
    line-height: 1;
    color: #fff;
    text-shadow: 0 1px 3px rgb(0 0 0 / 50%);
    pointer-events: none;
  }

  @media (width <= 900px) {
    .topbar {
      flex-direction: column;
      align-items: flex-start;
    }
  }

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

<svelte:window on:click={handleWindowClick}/>
<div class="page-shell">
  <header class="topbar">
    <h1>BLT Volume Manager</h1>
    <div class="topbar-actions">
      {#if $devMode}
        <DevTools volume={$selectedVolume} onAction={doRefresh} />
      {/if}
      <button class="button-icon" title="Refresh" on:click={doRefresh} disabled={refreshing}>
        <svg xmlns="http://www.w3.org/2000/svg" height="24" viewBox="0 -960 960 960" width="24" fill="currentColor" class={refreshing ? 'spin' : ''} style="vertical-align:middle;">
          <path d="M160-160v-80h110l-16-14q-52-46-73-105t-21-119q0-111 66.5-197.5T400-790v84q-72 26-116 88.5T240-478q0 45 17 87.5t53 78.5l10 10v-98h80v240H160Zm400-10v-84q72-26 116-88.5T720-482q0-45-17-87.5T650-648l-10-10v98h-80v-240h240v80H690l16 14q49 49 71.5 106.5T800-482q0 111-66.5 197.5T560-170Z"/>
        </svg>
      </button>
      <div class="color-picker-wrapper">
        <button class="button-icon" title="Choose accent color" on:click|stopPropagation={() => showColorPicker = !showColorPicker}>
          <svg xmlns="http://www.w3.org/2000/svg" height="24" viewBox="0 -960 960 960" width="24" fill="currentColor">
            <path d="M480-80q-82 0-155-31.5t-127.5-86Q143-252 111.5-325T80-480q0-83 32.5-156t88-127Q256-817 330-848.5T488-880q80 0 151 27.5t124.5 76q53.5 48.5 85 115T880-518q0 115-70 176.5T640-280h-74q-9 0-12.5 5t-3.5 11q0 12 15 34.5t15 51.5q0 50-27.5 74T480-80Zm0-400Zm-177 23q17-17 17-43t-17-43q-17-17-43-17t-43 17q-17 17-17 43t17 43q17 17 43 17t43-17Zm120-160q17-17 17-43t-17-43q-17-17-43-17t-43 17q-17 17-17 43t17 43q17 17 43 17t43-17Zm200 0q17-17 17-43t-17-43q-17-17-43-17t-43 17q-17 17-17 43t17 43q17 17 43 17t43-17Zm120 160q17-17 17-43t-17-43q-17-17-43-17t-43 17q-17 17-17 43t17 43q17 17 43 17t43-17ZM480-160q9 0 14.5-5t5.5-13q0-14-15-33t-15-57q0-42 29-67t71-25h70q66 0 113-38.5T800-518q0-121-92.5-201.5T488-800q-136 0-232 93t-96 227q0 133 93.5 226.5T480-160Z"/>
          </svg>
        </button>
        {#if showColorPicker}
          <div class="color-picker-popover" bind:this={colorPickerEl}>
            <div class="color-grid">
              {#each accentColors as color (color.name)}
                <button
                  class="color-swatch"
                  class:active={$currentAccent === color.name}
                  style="background: {color.dark.accent}"
                  title={color.label}
                  on:click={() => { setAccentColor(color.name); showColorPicker = false; }}
                >
                  {#if $currentAccent === color.name}
                    <span class="color-label">✓</span>
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        {/if}
      </div>
      <button class="button-icon" title="Toggle light/dark mode" on:click={toggleTheme}>
        <svg xmlns="http://www.w3.org/2000/svg" height="24" viewBox="0 -960 960 960" width="24" fill="currentColor">
          <path d="M337.5-463Q311-498 289-537q-5 14-6.5 28.5T281-480q0 83 58 141t141 58q14 0 28.5-2t28.5-6q-39-22-74-48.5T396-396q-32-32-58.5-67ZM567-364.5Q630-328 702-308q-40 51-98 79.5T481-200q-117 0-198.5-81.5T201-480q0-65 28.5-123t79.5-98q20 72 56.5 135T453-452q51 51 114 87.5ZM743-380q-20-5-39.5-11T665-405q8-18 11.5-36.5T680-480q0-83-58.5-141.5T480-680q-20 0-38.5 3.5T405-665q-8-19-13.5-38T381-742q24-9 49-13.5t51-4.5q117 0 198.5 81.5T761-480q0 26-4.5 51T743-380ZM440-840v-120h80v120h-80Zm0 840v-120h80V0h-80Zm323-706-57-57 85-84 57 56-85 85ZM169-113l-57-56 85-85 57 57-85 84Zm671-327v-80h120v80H840ZM0-440v-80h120v80H0Zm791 328-85-85 57-57 84 85-56 57ZM197-706l-84-85 56-57 85 85-57 57Zm199 310Z"/>
        </svg>
      </button>
    </div>
  </header>

  <Toaster position="bottom-right" visibleToasts={3} toastOptions={{ style: 'padding:18px 28px;font-size:1rem;font-weight:500;border-radius:16px;background:var(--surface);color:var(--text);border:1px solid var(--border);box-shadow:0 6px 24px rgb(0 0 0 / 30%);' }} />

  {#if !$selectedVolume}
    <VolumesPage
      volumes={$filteredVolumes}
      loading={$volumesLoading}
      onSelect={onSelectVolume}
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
  <input class="input modal-input" type="text" placeholder={$selectedVolume}
    style="margin-bottom:16px;"
    bind:value={$deleteConfirmText} />
  <div class="modal-footer">
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
  <input class="input modal-input" type="text" placeholder='Type "delete" to confirm'
    style="margin-bottom:16px;"
    bind:value={$snapDeleteInput} />
  <div class="modal-footer">
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
  <input class="input modal-input" type="text" placeholder="Enter new volume name"
    style="margin-bottom:12px;"
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
      {:else}
        <SnapshotPicker
          mode="multi"
          value={$copySelectedSnapshotIds}
          onValueChange={(v: string | string[]) => $copySelectedSnapshotIds = v as string[]}
          volume={$copyRenameSource}
          restorePointID={$copyRestorePointID}
        />
      {/if}
    </div>
  {/if}

  {#if $copyRenameError}
    <p class="error-text">{$copyRenameError}</p>
  {/if}
  <div class="modal-footer">
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
  <input class="input modal-input" type="text" placeholder="Enter new volume name"
    style="margin-bottom:8px;"
    bind:value={$copyRenameTarget} />
  {#if $copyRenameError}
    <p class="error-text">{$copyRenameError}</p>
  {/if}
  <div class="modal-footer">
    <Button.Root class="button button-secondary" onclick={() => $renameVolModal = false}>Cancel</Button.Root>
    <Button.Root class="button" disabled={!$copyRenameTarget || $copyRenameLoading || $volumeLockInfo[$copyRenameSource]?.locked}
      onclick={confirmRenameVolume}>
      {$copyRenameLoading ? 'Renaming...' : 'Rename'}
    </Button.Root>
  </div>
</Modal>
