<script lang="ts">
  export let checking = false;
  export let repairing = false;
  export let onCheck: () => void = () => {};
  export let onRepair: () => void = () => {};
  export let onDeleteVolume: () => void = () => {};
</script>

<section class="panel panel-layout">
  <h2 class="panel-title">Management</h2>
  <div class="troubleshoot-actions">
    <button class="button button-secondary button-block" title="Check repository integrity" on:click={onCheck} disabled={checking}>
      {checking ? 'Checking...' : 'Check'}
    </button>
    <button class="button button-secondary button-block" title="Unlock and rebuild repository index" on:click={onRepair} disabled={repairing}>
      {repairing ? 'Repairing...' : 'Repair'}
    </button>
    <button class="button button-block" style="background:var(--red);color:#fff;" on:click={onDeleteVolume}>
      Delete volume
    </button>
  </div>
</section>

<style>
  .troubleshoot-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-top: auto;
  }
</style>
