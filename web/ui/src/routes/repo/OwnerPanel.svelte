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
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960" fill="currentColor" aria-hidden="true"><path d="M224.16-243q-98.66 0-167.91-69.25Q-13-381.51-13-480q0-98.91 69.25-168.45Q125.51-718 224-718q40.65 0 78.32 14Q340-690 370-663l57.5 49.5-90 81.5-47.5-41q-13.5-12-31-18t-35-6q-49 0-82.5 34T108-480q0 48.59 33.75 82.29Q175.49-364 224.14-364q16.86 0 33.61-6.25T288-388l304.94-274.93q29.56-26.57 66.7-40.82Q696.77-718 736.84-718q98.66 0 167.91 69.25Q974-579.49 974-481q0 98.91-69.25 168.45Q835.49-243 737-243q-40.65 0-78.32-14Q621-271 591-298l-57.5-49.5 90-81.5 47.5 41q13.5 12 31 18t35 6q49 0 82.5-34t33.5-83q0-48.59-33.75-82.29Q785.51-597 736.86-597q-16.86 0-33.61 6.25T673-573L368.06-298.07q-29.56 26.57-66.7 40.82Q264.23-243 224.16-243Z"/></svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960" fill="currentColor" aria-hidden="true"><path d="M479.92-56Q395-56 319.5-88.5 244-121 186.75-176.75t-92-130Q60-381 56-466h122q3 60 28.25 113t65.5 92.25Q312-221.5 365.49-198.5q53.48 23 114.33 23Q606.5-175.5 695-264t88.5-215q0-126.5-88.5-214.75T480-782q-74.43 0-137.96 32.5Q278.5-717 237.5-661H339v120H56v-283h120v49q58.5-60 136.25-94.5T480-904q87.83 0 164.91 33.25 77.09 33.25 134.83 90.9 57.73 57.65 91 134.82Q904-567.85 904-479.92 904-392 870.75-315q-33.25 77-90.9 134.74-57.65 57.73-134.82 91Q567.85-56 479.92-56ZM400-321.5q-16 0-27.25-11.25T361.5-360v-120q0-16 11.5-27.25t28.5-11.25V-560q0-32.38 23.08-55.44 23.09-23.06 55.5-23.06 32.42 0 55.42 23.06t23 55.44v41.5q17 0 28.5 11.25T598.5-480v120q0 16-11.25 27.25T560-321.5H400Zm38.5-197h83v-41.4q0-17.6-11.9-29.6-11.91-12-29.5-12-17.6 0-29.6 11.93t-12 29.57v41.5Z"/></svg>
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
  .lock-mode-badge svg { width: 20px; height: 20px; display: block; }

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
