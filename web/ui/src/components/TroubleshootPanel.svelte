<script lang="ts">
  import { Button } from 'bits-ui';

  export let checking = false;
  export let repairing = false;
  export let onCheck: () => void = () => {};
  export let onRepair: () => void = () => {};
  export let onDeleteVolume: () => void = () => {};
</script>

<section class="panel panel-layout">
  <h2 class="panel-title">Management</h2>
  <div class="troubleshoot-actions">
    <Button.Root class="button button-secondary button-block" title="Check repository integrity" onclick={onCheck} disabled={checking}>
      {checking ? 'Checking...' : 'Check'}
    </Button.Root>
    <Button.Root class="button button-secondary button-block" title="Unlock and rebuild repository index" onclick={onRepair} disabled={repairing}>
      {repairing ? 'Repairing...' : 'Repair'}
    </Button.Root>
    <Button.Root class="button button-block" style="background:var(--red);color:#fff;" onclick={onDeleteVolume}>
      Delete volume
    </Button.Root>
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
