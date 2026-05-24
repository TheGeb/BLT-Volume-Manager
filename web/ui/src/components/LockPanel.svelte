<script lang="ts">
  import type { LockStatus } from '../lib/types';
  import { formatDuration } from '../lib/util';
  import * as api from '../lib/api';

  export let lockStatus: LockStatus | null = null;
  export let volume = '';
  export let hostname = '';
  export let onLockCreated: () => void = () => {};
  export let onLocksDeleted: () => void = () => {};

  let creating = false;
  let deleting = false;

  async function createLock() {
    creating = true;
    const ownerName = prompt('Lock owner name:', hostname);
    if (!ownerName) { creating = false; return; }
    try {
      await api.createLock(volume, ownerName);
      onLockCreated();
    } catch {} finally { creating = false; }
  }

  async function deleteLocks() {
    deleting = true;
    try {
      await api.deleteLocks(volume);
      onLocksDeleted();
    } catch {} finally { deleting = false; }
  }
</script>

<section class="panel lock-panel" style="display:flex;flex-direction:column;">
  {#if lockStatus}
    <h2 class="lock-panel-title">Lock Status</h2>
    <div class="lock-info">
      <div class="lock-status-text">
        {lockStatus.locked ? 'Locked' : 'No lock'}
      </div>
      {#if lockStatus.owner}
        <div class="lock-owner">Owner: {lockStatus.owner}</div>
      {/if}
      {#if lockStatus.expires_in != null}
        <div class="lock-expiry">{formatDuration(lockStatus.expires_in)}</div>
      {/if}
    </div>
    <div class="lock-actions">
      <button class="button button-block" on:click={createLock} disabled={creating || lockStatus.locked}>
        {creating ? 'Creating...' : 'Create lock'}
      </button>
      <button class="button button-secondary button-block" on:click={deleteLocks} disabled={deleting || !lockStatus.locked}>
        {deleting ? 'Deleting...' : 'Delete locks'}
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
