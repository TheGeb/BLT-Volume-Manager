<script lang="ts">
  import { onMount, onDestroy } from 'svelte';

  export let show = false;
  export let onClose: () => void = () => {};

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && show) onClose();
  }

  function handleOverlayClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onClose();
  }

  onMount(() => document.addEventListener('keydown', handleKeydown));
  onDestroy(() => document.removeEventListener('keydown', handleKeydown));
</script>

{#if show}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="modal-overlay" on:click={handleOverlayClick}>
    <div class="modal">
      <slot />
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed; inset: 0; z-index: 1000;
    background: rgba(0,0,0,0.6);
    display: flex; align-items: center; justify-content: center;
  }
  .modal {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 24px;
    padding: 28px;
    max-width: 440px;
    width: 90%;
    box-shadow: var(--shadow);
  }
</style>
