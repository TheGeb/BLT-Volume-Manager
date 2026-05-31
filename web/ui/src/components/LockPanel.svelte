<script lang="ts">
  import type { LockStatus } from '../lib/types';
  import { formatExpiration } from '../lib/util';
  import * as api from '../lib/api';

  export let lockStatus: LockStatus | null = null;
  export let volume = '';
  export let onLocksDeleted: () => void = () => {};

  let deleting = false;

  async function deleteLocks() {
    deleting = true;
    try {
      await api.deleteLocks(volume);
      onLocksDeleted();
    } catch {} finally { deleting = false; }
  }
</script>

<section class="panel panel-layout">
  {#if lockStatus}
    <h2 class="panel-title">Status</h2>
    <div class="panel-info">
      <div class="panel-info-primary">
        {lockStatus.locked ? 'Locked' : 'No lock'}
      </div>
      {#if lockStatus.owner}
        <div class="panel-info-secondary">Owner: {lockStatus.owner}</div>
      {/if}
      {#if lockStatus.expires_in != null}
        <div class="lock-expiry">{formatExpiration(lockStatus.expires_in)}</div>
      {/if}
    </div>
    <div class="lock-actions">
      <button class="button button-block" style="background:var(--red);color:#fff;" on:click={deleteLocks} disabled={deleting || !lockStatus.locked}>
        {deleting ? 'Deleting...' : 'Delete lock'}
      </button>
    </div>
  {:else}
    <div style="padding:20px;text-align:center;color:var(--muted);font-size:0.9rem;">
      <div class="skeleton" style="height:28px;width:120px;border-radius:6px;margin:0 auto 12px;"></div>
      <div class="skeleton" style="height:18px;width:160px;border-radius:6px;margin:0 auto 8px;"></div>
      <div class="skeleton" style="height:18px;width:200px;border-radius:6px;margin:0 auto 20px;"></div>
      <div class="skeleton" style="height:40px;border-radius:999px;margin-bottom:10px;"></div>
      <div class="skeleton" style="height:40px;border-radius:999px;"></div>
    </div>
  {/if}
</section>

<style>
  .lock-expiry {
    font-size: 0.85rem;
    color: var(--muted);
  }

  .lock-actions {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: auto;
  }
</style>
