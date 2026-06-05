<script lang="ts">
  import StatsGrid from './StoragePanel.svelte';
  import LockPanel from './LockPanel.svelte';
  import TroubleshootPanel from './ActionsPanel.svelte';
  import { onSelectVolume, switchTab } from '$lib/stores/navigation';
  import { activeTab, lockStatus, stats, statsLoading, checking, repairing, loadLockStatus, handleCheck, handleRepair } from '$lib/stores/repo';
  import { selectedVolume, openDeleteVolModal } from '$lib/stores/volumes';
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
    <div class="repo-layout">
      <StatsGrid stats={$stats} loading={$statsLoading} />
      <LockPanel
        lockStatus={$lockStatus}
        volume={$selectedVolume}
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

  .repo-layout {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
    align-items: stretch;
  }
</style>
