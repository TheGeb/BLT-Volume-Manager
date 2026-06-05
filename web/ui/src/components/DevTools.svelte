<script lang="ts">
  import { Button, Dialog } from 'bits-ui';
  import * as api from '$lib/api';

  export let volume = '';
  export let onAction: () => void = () => {};

  let open = false;
  let busy = false;
  let activeDialog: 'volume' | 'snapshot' | 'lock' | null = null;
  let inputName = '';
  let inputOwner = 'test-user';
  let devEl: HTMLElement;

  function toggle() { open = !open; }
  function closeDev() { open = false; }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') closeDev();
  }

  function handleDocClick(e: MouseEvent) {
    if (open && devEl && !devEl.contains(e.target as Node)) closeDev();
  }

  function defaultVolume(defaultVal: string) {
    return volume || defaultVal;
  }

  function openDialog(d: 'volume' | 'snapshot' | 'lock') {
    activeDialog = d;
    inputName = d === 'volume' ? '' : d === 'lock' ? volume : defaultVolume('test/example');
    inputOwner = 'test-user';
  }

  function closeDialog() { activeDialog = null; inputName = ''; inputOwner = ''; }

  async function submit(fn: () => Promise<void>) {
    busy = true;
    try { await fn(); onAction(); closeDialog(); closeDev(); }
    catch { alert('Failed'); }
    finally { busy = false; }
  }

  function submitVolume() {
    const name = inputName.trim();
    if (name) void submit(() => api.createTestVolume(name));
  }
  function submitSnapshot() {
    const vol = inputName.trim();
    if (vol) void submit(() => api.createTestSnapshot(vol));
  }
  function submitLock() {
    const vol = inputName.trim();
    const owner = inputOwner.trim();
    if (vol && owner) void submit(() => api.createLock(vol, owner));
  }
</script>

<svelte:window on:keydown={handleKeydown} on:click={handleDocClick} />

<div class="dev-tools" bind:this={devEl}>
  {#if open}
    <div class="dev-menu" role="menu">
      <button class="button dev-menu-btn" on:click={() => openDialog('volume')} disabled={busy}>Volume</button>
      <button class="button dev-menu-btn" on:click={() => openDialog('snapshot')} disabled={busy}>Snapshot</button>
      <button class="button dev-menu-btn" on:click={() => openDialog('lock')} disabled={busy}>Lock</button>
    </div>
  {/if}
  <button class="dev-fab" on:click={toggle} class:dev-fab-open={open} title="Dev tools">
    {open ? '×' : '+'}
  </button>
</div>

{#if activeDialog}
  <Dialog.Root open onOpenChange={(o) => { if (!o) closeDialog(); }}>
    <Dialog.Portal>
      <Dialog.Overlay class="modal-overlay" />
      <Dialog.Content class="modal" style="max-width:360px;">
        <h3 style="margin:0 0 12px;">
          {activeDialog === 'volume' ? 'Create Volume' : activeDialog === 'snapshot' ? 'Create Snapshot' : 'Create Lock'}
        </h3>
        <input class="input" type="text"
          placeholder={activeDialog === 'snapshot' ? 'Target volume' : 'Volume'}
          style="width:100%;box-sizing:border-box;margin-bottom:{activeDialog === 'lock' ? '8px' : '12px'};"
          bind:value={inputName}
          on:keydown={(e) => e.key === 'Enter' && (activeDialog === 'volume' ? submitVolume() : activeDialog === 'snapshot' ? submitSnapshot() : submitLock())} />
        {#if activeDialog === 'lock'}
          <input class="input" type="text" placeholder="Owner"
            style="width:100%;box-sizing:border-box;margin-bottom:12px;"
            bind:value={inputOwner}
            on:keydown={(e) => e.key === 'Enter' && submitLock()} />
        {/if}
        <div style="display:flex;gap:8px;justify-content:flex-end;">
          <Button.Root class="button button-secondary" onclick={closeDialog}>Cancel</Button.Root>
          <Button.Root class="button" onclick={activeDialog === 'volume' ? submitVolume : activeDialog === 'snapshot' ? submitSnapshot : submitLock}
            disabled={busy || !inputName.trim() || (activeDialog === 'lock' && !inputOwner.trim())}>
            {busy ? 'Creating...' : 'Create'}
          </Button.Root>
        </div>
      </Dialog.Content>
    </Dialog.Portal>
  </Dialog.Root>
{/if}

<style>
  .dev-tools {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .dev-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 8px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 8px;
    box-shadow: 0 4px 16px rgb(0 0 0 / 30%);
    min-width: 170px;
    z-index: 100;
  }

  .dev-menu-btn {
    background: var(--surface-strong);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 8px 14px;
    border-radius: 8px;
    font-size: 0.85rem;
    cursor: pointer;
    text-align: left;
    transition: background 0.15s;
  }
  .dev-menu-btn:hover { background: var(--hover-bg); }
  .dev-menu-btn:disabled { opacity: 0.5; cursor: default; }

  .dev-fab {
    width: 44px; height: 44px;
    border-radius: 14px;
    border: 1px solid var(--accent);
    background: var(--accent);
    color: #fff;
    font-size: 1.2rem;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: transform 0.2s ease, background 0.15s;
    line-height: 1;
  }
  .dev-fab:hover { background: var(--hover-bg); }
  .dev-fab-open { transform: rotate(45deg); }
</style>
