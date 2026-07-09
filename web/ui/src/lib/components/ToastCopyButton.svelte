<script lang="ts">
  import { toast } from 'svelte-sonner';
  import { toastCopyMsg } from '../stores/toast';

  let showCopied = $state(false);

  function handleCopy() {
    navigator.clipboard.writeText($toastCopyMsg);
    showCopied = true;
    setTimeout(() => { showCopied = false; }, 1000);
  }
</script>

<div class="copy-wrapper">
  <button
    onclick={handleCopy}
    class="copy-btn"
    title="Copy text"
  >
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
    </svg>
  </button>
  {#if showCopied}
    <span class="copied-badge">Copied!</span>
  {/if}
</div>

<style>
  .copy-wrapper {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .copy-btn {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--muted);
    padding: 4px;
    border-radius: 6px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: color 0.15s, background 0.15s;
  }

  .copy-btn:hover,
  .copy-btn:focus-visible {
    background: var(--hover-bg);
    color: var(--text);
    outline: none;
  }

  .copied-badge {
    position: absolute;
    bottom: calc(100% + 4px);
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface);
    color: var(--accent);
    border: 1px solid var(--accent);
    font-size: 0.65rem;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 4px;
    white-space: nowrap;
    pointer-events: none;
    opacity: 0;
    animation: fade-in-out 1s ease;
  }

  @keyframes fade-in-out {
    0% { opacity: 0; transform: translateX(-50%) scale(0.9); }
    15% { opacity: 1; transform: translateX(-50%) scale(1); }
    70% { opacity: 1; transform: translateX(-50%) scale(1); }
    100% { opacity: 0; transform: translateX(-50%) scale(0.9); }
  }
</style>
