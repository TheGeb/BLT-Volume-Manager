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

  // Permanent (create mode) locks carry no expiry; TTL (mount mode) locks
  // always carry a positive expiry. Both backends (S3, etcd) report the same.
  $: lockMode = ownerStatus?.owner
    ? (ownerStatus.expiry != null && ownerStatus.expiry > 0)
      ? 'mount'
      : 'create'
    : '';
  $: isPermLock = ownerStatus?.owner ? !(ownerStatus.expiry != null && ownerStatus.expiry > 0) : false;

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
        <div class="panel-info-secondary lock-mode-row">
          <span>Lock mode:</span>
          <span class="lock-mode-badge" data-tooltip={isPermLock
            ? 'Permanent lock - held from volume creation until the volume is removed from the host.'
            : 'Temporary lock - re-acquired on each container mount; expires by lock TTL (S3) or is held via keepalive while the owner renews (etcd).'}>
            {#if isPermLock}
              <span class="material-icon" aria-hidden="true" style="mask: url('/all_inclusive.svg') no-repeat center / contain;"></span>
            {:else}
              <span class="material-icon" aria-hidden="true" style="mask: url('/lock.svg') no-repeat center / contain;"></span>
            {/if}
          </span>
          <span>({lockMode})</span>
        </div>
      {/if}
      {#if ownerStatus.expiry != null && ownerStatus.expiry > 0}
        <div class="owner-expiry">{formatExpiration(ownerStatus.expiry - Math.floor(Date.now() / 1000))}</div>
      {/if}
    </div>
    <div class="owner-actions">
      <Button.Root class="button button-block button-destructive" onclick={deleteOwner} disabled={deleting || !ownerStatus.owner}>
        {deleting ? 'Deleting...' : 'Delete lock'}
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

  .lock-mode-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .lock-mode-badge {
    position: relative;
    display: inline-flex;
    align-items: center;
    color: inherit;
    line-height: 0;
    transform: translateY(1px);
    cursor: default;
  }
  .lock-mode-badge .material-icon { width: 20px; height: 20px; display: block; }

  .lock-mode-badge::after {
    content: attr(data-tooltip);
    position: absolute; bottom: 100%; left: 50%; z-index: 60;
    width: max-content; max-width: 240px; white-space: normal;
    padding: 6px 8px; border-radius: 6px;
    background: var(--tooltip-bg, rgb(20 20 24 / 96%)); color: var(--tooltip-fg, #f0f0f0);
    font-size: 0.75rem; line-height: 1.35; font-weight: 400;
    box-shadow: 0 2px 10px rgb(0 0 0 / 35%);
    opacity: 0; pointer-events: auto; transform: translateX(-50%) translateY(2px);
    transition: opacity 0.15s ease, transform 0.15s ease;
  }
  .lock-mode-badge:hover::after { opacity: 1; transform: translateX(-50%) translateY(0); }

  .owner-actions {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: auto;
  }
</style>
