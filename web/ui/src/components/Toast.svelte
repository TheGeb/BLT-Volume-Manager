<script lang="ts">
  import { toasts, dismissToast } from '../lib/stores/toast';

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') toasts.set([]);
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $toasts.length > 0}
  <div class="toast-stack">
    {#each $toasts as toast (toast.id)}
      <div class="toast" class:error={toast.error} class:success={!toast.error}>
        <span class="toast-msg">{toast.message}</span>
        <button class="toast-close" on:click={() => dismissToast(toast.id)} aria-label="Dismiss">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
          </svg>
        </button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-stack {
    position: fixed;
    bottom: 28px;
    right: 28px;
    z-index: 2000;
    display: flex;
    flex-direction: column;
    gap: 12px;
    width: 480px;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 18px 24px;
    border-radius: 16px;
    border: 1px solid transparent;
    box-shadow: 0 6px 24px rgb(0 0 0 / 30%);
    pointer-events: auto;
    width: 100%;
    box-sizing: border-box;
    animation: toast-in 0.25s ease-out;
  }

  .toast.error {
    background: rgb(239 68 68 / 92%);
    border-color: rgb(239 68 68);
    color: #fff;
  }

  .toast.success {
    background: rgb(17 24 39 / 94%);
    border-color: var(--border);
    color: var(--text);
    backdrop-filter: blur(12px);
  }

  .toast-msg {
    flex: 1;
    font-size: 0.95rem;
    font-weight: 500;
  }

  .toast-close {
    background: none;
    border: none;
    padding: 5px;
    cursor: pointer;
    color: inherit;
    opacity: 0.7;
    display: flex;
    align-items: center;
    flex-shrink: 0;
    border-radius: 6px;
    transition: opacity 0.15s, background 0.15s;
  }

  .toast-close:hover {
    opacity: 1;
    background: rgb(255 255 255 / 12%);
  }

  @keyframes toast-in {
    from { opacity: 0; transform: translateY(12px); }
    to { opacity: 1; transform: translateY(0); }
  }
</style>
