<script lang="ts">
  import { onMount } from 'svelte';
  import Banner from './components/Banner.svelte';
  import Toolbar from './components/Toolbar.svelte';
  import LandingPanel from './components/LandingPanel.svelte';
  import SnapshotTable from './components/SnapshotTable.svelte';
  import SnapshotViewer from './components/SnapshotViewer.svelte';
  import StatsGrid from './components/StatsGrid.svelte';
  import LockPanel from './components/LockPanel.svelte';
  import TroubleshootPanel from './components/TroubleshootPanel.svelte';
  import Modal from './components/Modal.svelte';
  import type { Snapshot, AppState, StatsResponse, LockStatus } from './lib/types';
  import { formatBytes } from './lib/util';
  import * as api from './lib/api';

  let state: AppState = {
    snapshots: [], volumes: [], selectedVolume: '', volumeFilter: '',
    query: '', sortNewestFirst: true, hostname: '', prevStats: null, showHot: true,
  };

  let loading = true;
  let activeTab: 'snapshots' | 'repo' = 'snapshots';
  let bannerText = '';
  let bannerError = false;
  let lockStatus: LockStatus | null = null;
  let stats: StatsResponse | null = null;
  let statsLoading = false;
  let sizes: Record<string, string> = {};
  let currentSnapshot: Snapshot | null = null;
  let allSnapshots: Snapshot[] = [];
  let viewerOpen = false;
  let checking = false;
  let repairing = false;
  let deleteVolModal = false;
  let deleteSnapModal = false;
  let deletingSnap: Snapshot | null = null;
  let deleteConfirmText = '';
  let snapDeleteInput = '';
  let deleteVolLoading = false;
  let creatingTest = false;
  let testStatus = '';
  let themeDark = true;
  let pillsCachedAt = '';

  let pillsLoading = false;
  let snapsLoading = false;

  $: filteredVolumes = state.volumes.filter(v =>
    v.toLowerCase().includes(state.volumeFilter.toLowerCase())
  );

  $: filteredSnapshots = state.snapshots.filter(sn => {
    if (!state.showHot && sn.tags.includes('hot')) return false;
    if (!state.query) return true;
    const q = state.query;
    return sn.short_id.toLowerCase().includes(q) ||
      sn.tags.some(t => t.toLowerCase().includes(q)) ||
      sn.hostname?.toLowerCase().includes(q);
  });

  $: sortedSnapshots = [...filteredSnapshots].sort((a, b) => {
    const da = new Date(a.time).getTime();
    const db = new Date(b.time).getTime();
    return state.sortNewestFirst ? db - da : da - db;
  });

  function setBanner(msg: string, isError = false) {
    bannerText = msg;
    bannerError = isError;
  }

  async function loadVolumes() {
    pillsLoading = true;
    try {
      state.volumes = await api.fetchVolumes();
      pillsLoading = false;
    } catch (e) {
      pillsLoading = false;
      setBanner('Cannot reach server', true);
    }
  }

  async function loadSnapshots(volume: string) {
    snapsLoading = true;
    try {
      state.snapshots = await api.fetchSnapshots(volume);
      snapsLoading = false;
    } catch (e) {
      state.snapshots = [];
      snapsLoading = false;
      setBanner('Failed to load snapshots', true);
    }
  }

  async function loadAll(volume: string) {
    state.selectedVolume = volume;
    allSnapshots = [];
    viewerOpen = false;
    currentSnapshot = null;
    sizes = {};
    deleteVolModal = false;
    deleteSnapModal = false;
    landingShown = !volume;
    if (volume) {
      await Promise.all([
        loadSnapshots(volume),
        loadLockStatus(volume),
        loadStats(volume),
      ]);
    }
  }

  async function loadLockStatus(volume: string) {
    try {
      lockStatus = await api.fetchLockStatus(volume);
    } catch { lockStatus = null; }
  }

  async function loadStats(volume: string) {
    statsLoading = true;
    try {
      stats = await api.fetchStats(volume);
      state.prevStats = stats;
      statsLoading = false;
    } catch {
      statsLoading = false;
    }
  }

  async function handleRefresh() {
    setBanner('');
    const vol = state.selectedVolume;
    try { await api.refreshStats(); } catch {}
    if (vol) {
      await Promise.all([
        loadSnapshots(vol),
        loadStats(vol),
      ]);
    }
    await loadVolumes();
    if (vol) loadLockStatus(vol);
  }

  function onSelectVolume(vol: string) {
    if (vol === state.selectedVolume) {
      state.selectedVolume = '';
      loadAll('');
      return;
    }
    loadAll(vol);
  }

  function onToggleSort() {
    state.sortNewestFirst = !state.sortNewestFirst;
  }

  function onSearch(q: string) {
    state.query = q;
  }

  function onFilterChange(f: string) {
    state.volumeFilter = f;
  }

  function onToggleHot() {
    state.showHot = !state.showHot;
  }

  function onOpenViewer(snapshot: Snapshot) {
    currentSnapshot = snapshot;
    allSnapshots = state.snapshots;
    viewerOpen = true;
  }

  function onCloseViewer() {
    viewerOpen = false;
    currentSnapshot = null;
    allSnapshots = [];
  }

  let rpLoading: Record<string, boolean> = {};

  async function onAddTag(id: string, tag: string, vol: string) {
    rpLoading = { ...rpLoading, [id]: true };
    try {
      await api.addTag(id, tag, vol);
      await loadSnapshots(vol);
    } catch { setBanner('Failed to add tag', true); }
    finally {
      const next = { ...rpLoading };
      delete next[id];
      rpLoading = next;
    }
  }

  async function onRemoveTag(id: string, tag: string, vol: string) {
    rpLoading = { ...rpLoading, [id]: true };
    try {
      await api.removeTag(id, tag, vol);
      await loadSnapshots(vol);
    } catch { setBanner('Failed to remove tag', true); }
    finally {
      const next = { ...rpLoading };
      delete next[id];
      rpLoading = next;
    }
  }

  async function onDeleteSnapshot(sn: Snapshot) {
    deletingSnap = sn;
    snapDeleteInput = '';
    deleteSnapModal = true;
  }

  async function confirmDeleteSnapshot() {
    if (!deletingSnap) return;
    try {
      await api.deleteSnapshot(deletingSnap.id, state.selectedVolume);
      deleteSnapModal = false;
      deletingSnap = null;
      setBanner('Snapshot deleted');
      await loadSnapshots(state.selectedVolume);
    } catch (e: any) {
      setBanner(e.message, true);
    }
  }

  function openDeleteVolModal() {
    const vol = state.selectedVolume;
    if (!vol) return;
    deleteConfirmText = '';
    deleteVolModal = true;
  }

  async function confirmDeleteVolume() {
    const vol = state.selectedVolume;
    if (!vol || deleteConfirmText !== vol) return;
    deleteVolLoading = true;
    try {
      await api.deleteVolume(vol);
      deleteVolModal = false;
      deleteVolLoading = false;
      setBanner(`Volume ${vol} deleted`);
      state.selectedVolume = '';
      loadAll('');
      await Promise.all([
        api.refreshStats().catch(() => {}),
        loadVolumes(),
      ]);
    } catch (e: any) {
      deleteVolLoading = false;
      setBanner(e.message, true);
    }
  }

  async function handleCheck() {
    const vol = state.selectedVolume;
    if (!vol) return;
    checking = true;
    setBanner('');
    try {
      const msg = await api.checkRepo(vol);
      setBanner(msg);
    } catch (e: any) { setBanner(e.message, true); }
    finally { checking = false; }
  }

  async function handleRepair() {
    const vol = state.selectedVolume;
    if (!vol) return;
    repairing = true;
    setBanner('');
    try {
      const msg = await api.repairRepo(vol);
      setBanner(msg);
    } catch (e: any) { setBanner(e.message, true); }
    finally { repairing = false; }
  }

  async function handleCreateTestVolume(name: string) {
    creatingTest = true;
    testStatus = '';
    try {
      await api.createTestVolume(name);
      testStatus = 'Updating volume list...';
      await api.refreshStats().catch(() => {});
      await loadVolumes();
      state.selectedVolume = name;
      loadAll(name);
    } catch (e: any) { testStatus = e.message; }
    finally { creatingTest = false; }
  }

  function switchTab(tab: 'snapshots' | 'repo') {
    activeTab = tab;
    if (tab === 'repo' && state.selectedVolume) {
      loadStats(state.selectedVolume);
    }
  }

  function toggleTheme() {
    themeDark = !themeDark;
    document.body.classList.toggle('light', !themeDark);
  }

  let landingShown = true;

  let sizeLoading: Record<string, boolean> = {};

  async function handleSizeLoaded(id: string, vol: string) {
    sizeLoading = { ...sizeLoading, [id]: true };
    try {
      const data = await api.fetchSnapshotSizes(vol, [id]);
      if (data[id] != null) {
        sizes = { ...sizes, [id]: formatBytes(data[id]) };
      } else {
        sizes = { ...sizes, [id]: 'err' };
      }
    } catch {
      sizes = { ...sizes, [id]: 'err' };
    } finally {
      const next = { ...sizeLoading };
      delete next[id];
      sizeLoading = next;
    }
  }

  onMount(async () => {
    if (window.matchMedia('(prefers-color-scheme: light)').matches) {
      themeDark = false;
      document.body.classList.add('light');
    }
    await loadVolumes();
    if (state.volumes.length === 0) {
      setBanner('No volumes found. Create one with: docker volume create --driver s3vol --name <name>');
    } else {
      setBanner('');
    }
    loading = false;
  });
</script>

<div class="page-shell">
  <header class="topbar">
    <div>
      <h1>BLT Volume Manager</h1>
    </div>
    <div class="topbar-actions">
      <button class="button-icon" title="Refresh" on:click={handleRefresh}>
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="23 4 23 10 17 10"></polyline>
          <polyline points="1 20 1 14 7 14"></polyline>
          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
        </svg>
      </button>
      <button class="button-icon" title="Toggle light/dark mode" on:click={toggleTheme}>
        {#if themeDark}
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

  <Banner {bannerText} {bannerError} />

  <Toolbar
    volumes={filteredVolumes}
    selectedVolume={state.selectedVolume}
    volumeFilter={state.volumeFilter}
    loading={pillsLoading}
    pillsCachedAt={pillsCachedAt}
    onSelect={onSelectVolume}
    onFilterChange={onFilterChange}
  />

  {#if landingShown}
    <LandingPanel onCreateTestVolume={handleCreateTestVolume} {creatingTest} {testStatus} />
  {/if}

  {#if state.selectedVolume}
    <div id="volumeView">
      <div class="tab-bar">
        <button class="tab" class:tab-active={activeTab === 'snapshots'} on:click={() => switchTab('snapshots')}>Snapshots</button>
        <button class="tab" class:tab-active={activeTab === 'repo'} on:click={() => switchTab('repo')}>Repo</button>
      </div>

      {#if activeTab === 'snapshots'}
        <div class="tab-panel">
          {#if viewerOpen && currentSnapshot}
            <SnapshotViewer
              snapshot={currentSnapshot}
              allSnapshots={allSnapshots}
              onClose={onCloseViewer}
            />
          {/if}

          <SnapshotTable
              snapshots={sortedSnapshots}
              sizes={sizes}
              selectedVolume={state.selectedVolume}
              sortNewestFirst={state.sortNewestFirst}
              query={state.query}
              showHot={state.showHot}
              loading={snapsLoading}
              {rpLoading}
              {sizeLoading}
              onSearch={onSearch}
              onToggleSort={onToggleSort}
              onToggleHot={onToggleHot}
              onOpenViewer={onOpenViewer}
              onAddTag={onAddTag}
              onRemoveTag={onRemoveTag}
              onDeleteSnapshot={onDeleteSnapshot}
              onSizeLoaded={(id) => handleSizeLoaded(id, state.selectedVolume)}
            />
        </div>
      {:else}
        <div class="tab-panel">
          <div class="repo-layout">
            <StatsGrid {stats} loading={statsLoading} />
            <LockPanel
              {lockStatus}
              volume={state.selectedVolume}
              hostname={state.hostname}
              onLockCreated={() => loadLockStatus(state.selectedVolume)}
              onLocksDeleted={() => loadLockStatus(state.selectedVolume)}
            />
            <TroubleshootPanel
              {checking}
              {repairing}
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

<Modal show={deleteVolModal} onClose={() => deleteVolModal = false}>
  <h3 style="margin:0 0 12px;color:var(--red);">Delete volume</h3>
  <p style="margin:0 0 8px;color:var(--muted);font-size:0.9rem;">
    This will permanently delete the volume, all its snapshots, backups, and locks from S3.
  </p>
  <p style="margin:0 0 16px;color:var(--yellow);font-size:0.9rem;">
    Make sure no other hosts are still using this volume before proceeding.
  </p>
  <p style="margin:0 0 8px;font-size:0.85rem;">
    Type <strong>{state.selectedVolume}</strong> to confirm:
  </p>
  <input class="input" type="text" placeholder={state.selectedVolume}
    style="width:100%;box-sizing:border-box;margin-bottom:16px;"
    bind:value={deleteConfirmText} />
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <button class="button button-secondary" on:click={() => deleteVolModal = false}>Cancel</button>
    <button class="button" style="background:var(--red);color:#fff;"
      disabled={deleteConfirmText !== state.selectedVolume || deleteVolLoading}
      on:click={confirmDeleteVolume}>
      {deleteVolLoading ? 'Deleting...' : 'Delete'}
    </button>
  </div>
</Modal>

<Modal show={deleteSnapModal} onClose={() => deleteSnapModal = false}>
  <h3 style="margin:0 0 12px;color:var(--red);">Delete snapshot</h3>
  {#if deletingSnap}
  <div style="margin-bottom:16px;font-size:0.85rem;">
    <div><strong>Hostname:</strong> {deletingSnap.hostname}</div>
    <div><strong>Date:</strong> {new Date(deletingSnap.time).toLocaleString()}</div>
    <div><strong>Tags:</strong> {deletingSnap.tags.join(', ') || '—'}</div>
  </div>
  {/if}
  <p style="margin:0 0 8px;font-size:0.85rem;">Type <strong>delete</strong> to confirm:</p>
  <input class="input" type="text" placeholder='Type "delete" to confirm'
    style="width:100%;box-sizing:border-box;margin-bottom:16px;"
    bind:value={snapDeleteInput} />
  <div style="display:flex;gap:8px;justify-content:flex-end;">
    <button class="button button-secondary" on:click={() => deleteSnapModal = false}>Cancel</button>
    <button class="button" style="background:var(--red);color:#fff;"
      disabled={snapDeleteInput !== 'delete'}
      on:click={confirmDeleteSnapshot}>Delete</button>
  </div>
</Modal>
