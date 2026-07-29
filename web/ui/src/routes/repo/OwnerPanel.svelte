<script lang="ts">
  import { Button } from 'bits-ui';
  import type { OwnerStatus } from '$lib/types';
  import { formatExpiration } from '$lib/util';
  import * as api from '$lib/api';
  import { showToast } from '$lib/stores/toast';

  export let ownerStatus: OwnerStatus | null = null;
  export let volume = '';
  export let onOwnerDeleted: () => void = () => {};

  let deleting = false;

  async function deleteOwner() {
    deleting = true;
    try {
      await api.deleteOwnerLock(volume);
      onOwnerDeleted();
    } catch (e: unknown) {
      showToast((e as Error).message, true);
    } finally { deleting = false; }
  }
</script>

<section class="panel panel-layout">
  {#if ownerStatus}
    <h2 class="panel-title">Status</h2>
    <div class="panel-info">
      <div class="panel-info-primary">
        {ownerStatus.owner ? 'Locked' : 'Unclaimed'}
      </div>
      {#if ownerStatus.owner}
        <div class="panel-info-secondary">Owner: {ownerStatus.owner}</div>
      {/if}
      {#if ownerStatus.expiry != null && ownerStatus.expiry > 0}
        <div class="owner-expiry">{formatExpiration(ownerStatus.expiry - Math.floor(Date.now() / 1000))}</div>
      {/if}
    </div>
    <div class="owner-actions">
      <Button.Root class="button button-block button-destructive" onclick={deleteOwner} disabled={deleting || !ownerStatus.owner}>
        {deleting ? 'Deleting...' : 'Delete owner'}
      </Button.Root>
    </div>
  {:else}
    <div style="padding:20px;color:var(--muted);font-size:0.9rem;">
      <div class="skeleton" style="height:28px;width:120px;border-radius:6px;margin-bottom:12px;"></div>
      <div class="skeleton" style="height:18px;width:160px;border-radius:6px;margin-bottom:8px;"></div>
      <div class="skeleton" style="height:18px;width:200px;border-radius:6px;margin-bottom:20px;"></div>
      <div class="skeleton" style="height:40px;border-radius:999px;margin-bottom:10px;"></div>
      <div class="skeleton" style="height:40px;border-radius:999px;"></div>
    </div>
  {/if}
</section>

<style>
  .owner-expiry {
    font-size: 0.85rem;
    color: var(--muted);
  }

  .owner-actions {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: auto;
  }
</style>
