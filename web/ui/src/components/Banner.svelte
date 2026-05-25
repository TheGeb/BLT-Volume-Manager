<script lang="ts">
  export let bannerText = '';
  export let bannerError = false;
  export let onClose: () => void = () => {};
</script>

<div class="banner" class:visible={bannerText.length > 0} class:error={bannerError} class:info={bannerText.length > 0 && !bannerError}>
  <span style="flex:1;">{bannerText}</span>
  {#if bannerText.length > 0}
    <button class="banner-close" on:click={onClose} aria-label="Close banner">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
      </svg>
    </button>
  {/if}
</div>

<style>
  .banner {
    visibility: hidden; opacity: 0; transition: opacity 0.2s ease;
    display: flex; align-items: center; gap: 16px;
    padding: 16px 24px; margin-bottom: 20px; border-radius: 24px;
  }
  .banner.visible { visibility: visible; opacity: 1; }
  .banner.error {
    background: rgba(239, 68, 68, 0.08); border: 1px solid rgba(239, 68, 68, 0.3);
    color: var(--red);
  }
  .banner.info {
    background: rgba(251, 191, 36, 0.1); border: 1px solid rgba(251, 191, 36, 0.3);
    color: #fbbf24;
  }
  .banner-close {
    background: none; border: none; padding: 4px;
    cursor: pointer; color: inherit; opacity: 0.7;
    display: flex; align-items: center; flex-shrink: 0; border-radius: 6px;
  }
  .banner-close:hover { opacity: 1; background: rgba(0,0,0,0.1); }
</style>
