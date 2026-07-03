<script lang="ts">
  import { onMount } from 'svelte';
  import StatsGrid from '../../components/StoragePanel.svelte';
  import OwnerPanel from './OwnerPanel.svelte';
  import TroubleshootPanel from './ActionsPanel.svelte';
  import { onSelectVolume, switchTab } from '$lib/stores/navigation';
  import { activeTab, ownerStatus, stats, statsLoading, checking, repairing, loadOwnerStatus, handleCheck, handleRepair } from '$lib/stores/repo';
  import { selectedVolume, openDeleteVolModal } from '$lib/stores/volumes';

  onMount(() => {
    loadOwnerStatus();
  });
</script>

<div id="volume-view">
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
      <OwnerPanel
        ownerStatus={$ownerStatus}
        volume={$selectedVolume}
        onOwnerDeleted={() => loadOwnerStatus()}
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
  .repo-layout {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
    align-items: stretch;
  }
</style>
