<script lang="ts">
  import * as api from '../lib/api';

  export let volume = '';
  export let onAction: () => void = () => {};

  let open = false;
  let busy = false;

  function toggle() { open = !open; }
  function close() { open = false; }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  function defaultVolume(defaultVal: string) {
    return volume || defaultVal;
  }

  async function createVolume() {
    const name = prompt('Volume name:', '');
    if (!name) return;
    busy = true;
    try {
      await api.createTestVolume(name);
      onAction();
      close();
    } catch { alert('Failed'); } finally { busy = false; }
  }

  async function createSnapshot() {
    const vol = prompt('Target volume:', defaultVolume('test/example'));
    if (!vol) return;
    busy = true;
    try {
      await api.createTestSnapshot(vol);
      onAction();
      close();
    } catch { alert('Failed'); } finally { busy = false; }
  }

  async function createLock() {
    const vol = prompt('Volume:', defaultVolume('test/example'));
    if (!vol) return;
    const owner = prompt('Owner:', 'test-user');
    if (!owner) return;
    busy = true;
    try {
      await api.createLock(vol, owner);
      onAction();
      close();
    } catch { alert('Failed'); } finally { busy = false; }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="dev-tools">
  {#if open}
    <div class="dev-menu" role="menu">
      <button class="button dev-menu-btn" on:click={createVolume} disabled={busy}>Volume</button>
      <button class="button dev-menu-btn" on:click={createSnapshot} disabled={busy}>Snapshot</button>
      <button class="button dev-menu-btn" on:click={createLock} disabled={busy}>Lock</button>
    </div>
  {/if}
  <button class="dev-fab" on:click={toggle} class:dev-fab-open={open} title="Dev tools">
    {open ? '×' : '+'}
  </button>
</div>

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

  .dev-menu-btn:hover {
    background: var(--hover-bg);
  }

  .dev-menu-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .dev-fab {
    width: 44px;
    height: 44px;
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

  .dev-fab:hover {
    background: var(--hover-bg);
  }

  .dev-fab-open {
    transform: rotate(45deg);
  }
</style>
