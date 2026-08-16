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
  import MigrationModal from './components/MigrationModal.svelte';
  import SnapshotPicker from './components/SnapshotPicker.svelte';
  import { get } from 'svelte/store';
  import { showToast } from '$lib/stores/toast';
  import {
    volumes, selectedVolume, volumeOwnerInfo, volumesLoading,
    deleteVolModal, deleteConfirmText, deleteVolLoading, filteredVolumes,
    deleteRepoNotDeletable, deleteRepoPath, deleteAcknowledged,
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
  import { openMigrateModal } from '$lib/stores/migration';
  import { fetchVersion } from '$lib/api';
  import type { VersionInfo } from '$lib/api';
  import {
    creatingTest, testStatus,
    onSelectVolume, confirmDeleteVolume, handleCreateTestVolume,
    navigateTo, handleRefresh, syncUrl
  } from '$lib/stores/navigation';

  let initialSyncDone = false;
  let refreshing = false;
  $: busy = $loading;
  let showColorPicker = false;
  let colorPickerEl: HTMLDivElement;
  let showInfoModal = false;
  let versionInfo: VersionInfo | null = null;
  let versionLoading = false;

  async function loadVersionInfo() {
    if (versionInfo) return;
    versionLoading = true;
    try {
      versionInfo = await fetchVersion();
    } catch {
      versionInfo = {
        version: 'unknown',
        commit: 'unknown',
        date: 'unknown',
        metadata_backend: 'unknown',
        s3_endpoint: '',
        s3_bucket: '',
        etcd_endpoints: [],
      };
    } finally {
      versionLoading = false;
    }
  }

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
    requestAnimationFrame(() => {
      document.documentElement.classList.remove('no-theme-transition');
    });
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
      const navOpts: { tab?: string; tag?: string; diffTag?: string; snapshotId?: string; snapshotHash?: string; diffId?: string; diffHash?: string } = {};
      const tab = tabFromUrl ?? params.get('tab');
      if (tab) navOpts.tab = tab;
      const tag = params.get('tag');
      const diffTag = params.get('diffTag');
      const snapshotId = params.get('snapshot');
      const snapshotHash = params.get('hash');
      const diffId = params.get('diff');
      const diffHash = params.get('diffHash');
      if (tag) navOpts.tag = tag;
      else if (snapshotId) {
        navOpts.snapshotId = snapshotId;
        if (snapshotHash) navOpts.snapshotHash = snapshotHash;
      }
      if (diffTag) navOpts.diffTag = diffTag;
      else if (diffId) {
        navOpts.diffId = diffId;
        if (diffHash) navOpts.diffHash = diffHash;
      }
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

  @media (prefers-reduced-motion: reduce) {
    :global(*),
    :global(*::before),
    :global(*::after) {
      animation-duration: 0.01ms !important;
      transition-duration: 0.01ms !important;
    }
  }

  :global(.radio-item[data-state="checked"]::before) {
    border-color: var(--accent);
    background: var(--accent);
    box-shadow: inset 0 0 0 3px var(--surface);
  }

  .info-grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 8px 16px;
    font-size: 0.9rem;
  }

  .info-label {
    color: var(--muted);
    font-weight: 600;
  }

  .info-value {
    color: var(--text);
  }

  .info-value code {
    font-size: 0.8rem;
    background: var(--surface-strong);
    padding: 2px 6px;
    border-radius: 4px;
    word-break: break-all;
  }

</style>

<svelte:window on:click={handleWindowClick}/>
<div class="page-shell" aria-busy={busy}>
  <header class="topbar">
    <h1>BLT Volume Manager</h1>
    <div class="topbar-actions">
      {#if $devMode}
        <DevTools volume={$selectedVolume} onAction={doRefresh} />
      {/if}
      <button class="button-icon" title="Refresh" on:click={doRefresh} disabled={refreshing}>
        <span class="material-icon" class:spin={refreshing} style="mask: url('/material/sync.svg') no-repeat center / contain; width:28px;height:28px;"></span>
      </button>
      <button class="button-icon" title="Migrate metadata between S3 and etcd backends" on:click={openMigrateModal}>
        <span class="material-icon" style="mask: url('/material/swap_horiz.svg') no-repeat center / contain; width:28px;height:28px;"></span>
      </button>
      <div class="color-picker-wrapper">
        <button class="button-icon" title="Choose accent color" on:click|stopPropagation={() => showColorPicker = !showColorPicker}>
          <span class="material-icon" style="mask: url('/material/palette.svg') no-repeat center / contain;"></span>
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
        <span class="material-icon" style="mask: url('/material/routine.svg') no-repeat center / contain;"></span>
      </button>
      <button class="button-icon" title="About" on:click={() => { showInfoModal = true; loadVersionInfo(); }}>
        <span class="material-icon" style="mask: url('/material/info.svg') no-repeat center / contain;"></span>
      </button>
    </div>
  </header>

  <Toaster position="bottom-right" visibleToasts={3} toastOptions={{ style: 'padding:18px 28px;font-size:1rem;font-weight:500;border-radius:16px;background:var(--surface);color:var(--text);border:1px solid var(--border);box-shadow:0 6px 24px rgb(0 0 0 / 30%);', actionButtonStyle: 'background:var(--accent);color:#fff;border:none;border-radius:8px;padding:6px 12px;font-size:0.8rem;font-weight:600;cursor:pointer;font-family:inherit;margin-left:12px;flex-shrink:0;' }} />

  {#if !$selectedVolume}
    <VolumesPage
      volumes={$filteredVolumes}
      loading={$volumesLoading}
      onSelect={onSelectVolume}
      volumeOwnerInfo={$volumeOwnerInfo}
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
    This will permanently delete the volume, all its snapshots, backups, and owners from S3.
  </p>
  <p style="margin:0 0 16px;color:var(--yellow);font-size:0.9rem;">
    Make sure no other hosts are still using this volume before proceeding.
  </p>
  {#if $deleteRepoNotDeletable}
    <div style="border:1px solid color-mix(in srgb, var(--red), transparent 50%);border-radius:8px;padding:10px 12px;margin:0 0 12px;background:color-mix(in srgb, var(--red), transparent 92%);">
      <p style="margin:0 0 6px;color:var(--red);font-size:0.85rem;font-weight:600;">
        Backup data cannot be deleted automatically
      </p>
      <p style="margin:0 0 6px;color:var(--muted);font-size:0.85rem;">
        This volume's backup data lives on a remote backend (rest/sftp/rclone)
        that has no safe automatic delete. You must remove it manually from:
      </p>
      <code style="display:block;margin:0 0 8px;font-size:0.8rem;word-break:break-all;color:var(--text);">
        {$deleteRepoPath}
      </code>
      <label style="display:flex;align-items:flex-start;gap:8px;font-size:0.85rem;cursor:pointer;color:var(--text);">
        <input type="checkbox" style="margin-top:2px;" bind:checked={$deleteAcknowledged} />
        <span>I acknowledge that I will delete the backup data from the remote myself.</span>
      </label>
    </div>
  {/if}
  <p style="margin:0 0 8px;font-size:0.85rem;">
    Type <strong>{$selectedVolume}</strong> to confirm:
  </p>
  <input class="input modal-input" type="text" placeholder={$selectedVolume}
    style="margin-bottom:16px;"
    bind:value={$deleteConfirmText} />
  <div class="modal-footer">
    <Button.Root class="button button-secondary" onclick={() => $deleteVolModal = false}>Cancel</Button.Root>
    <Button.Root class="button button-destructive"
      disabled={$deleteConfirmText !== $selectedVolume || $deleteVolLoading || ($deleteRepoNotDeletable && !$deleteAcknowledged)}
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
    {#if $volumeOwnerInfo[$copyRenameSource]?.owner}
      <span style="color:var(--red);display:block;margin-top:6px;">
        This volume has an owner and cannot be renamed.
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
    <Button.Root class="button" disabled={!$copyRenameTarget || $copyRenameLoading || !!$volumeOwnerInfo[$copyRenameSource]?.owner}
      onclick={confirmRenameVolume}>
      {$copyRenameLoading ? 'Renaming...' : 'Rename'}
    </Button.Root>
  </div>
</Modal>

<Modal show={showInfoModal} onClose={() => showInfoModal = false}>
  <h3 style="margin:0 0 16px;">About BLT Volume Manager</h3>
  {#if versionLoading}
    <p style="color:var(--muted);font-size:0.9rem;">Loading...</p>
  {:else if versionInfo}
    <div class="info-grid">
      <div class="info-label">Version</div>
      <div class="info-value">{versionInfo.version}</div>
      <div class="info-label">Commit</div>
      <div class="info-value"><code>{versionInfo.commit}</code></div>
      <div class="info-label">Built</div>
      <div class="info-value">{versionInfo.date}</div>
      <div class="info-label">Metadata</div>
      <div class="info-value">{versionInfo.metadata_backend}</div>
      {#if versionInfo.metadata_backend === 's3'}
        {#if versionInfo.s3_endpoint}
          <div class="info-label">S3 Endpoint</div>
          <div class="info-value"><code>{versionInfo.s3_endpoint}</code></div>
        {/if}
        {#if versionInfo.s3_bucket}
          <div class="info-label">S3 Bucket</div>
          <div class="info-value"><code>{versionInfo.s3_bucket}</code></div>
        {/if}
      {/if}
      {#if versionInfo.metadata_backend === 'etcd' && versionInfo.etcd_endpoints?.length}
        <div class="info-label">etcd Endpoints</div>
        <div class="info-value">
          {#each versionInfo.etcd_endpoints as ep, i (ep)}
            <code>{ep}</code>{#if i < versionInfo.etcd_endpoints.length - 1}, {/if}
          {/each}
        </div>
      {/if}
    </div>
  {:else}
    <p style="color:var(--muted);font-size:0.9rem;">Failed to load version info.</p>
  {/if}
  <div class="modal-footer" style="margin-top:16px;">
    <Button.Root class="button button-secondary" onclick={() => showInfoModal = false}>Close</Button.Root>
  </div>
</Modal>

<MigrationModal />
