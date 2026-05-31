<script lang="ts">
  export let bannerText = '';
  export let bannerError = false;
  export let onClose: () => void = () => {};
</script>

<div class="banner-slot" class:visible={bannerText.length > 0}>
  <div class="banner" class:error={bannerError} class:info={bannerText.length > 0 && !bannerError}>
    <span style="flex:1;">{bannerText}</span>
    {#if bannerText.length > 0}
      <button class="banner-close" on:click={onClose} aria-label="Close banner">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
        </svg>
      </button>
    {/if}
  </div>
</div>

<style>
.banner-slot {
    min-height: 54px;
    margin-bottom: 20px;
    position: relative;
  }

  .banner {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    display: flex; align-items: center; gap: 16px;
    padding: 16px 24px; border-radius: 24px;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.4s ease;
    border: 1px solid transparent;
  }

  .banner-slot.visible .banner {
    opacity: 1;
    pointer-events: auto;
  }

  .banner.error {
    background: rgb(239 68 68 / 8%); border-color: rgb(239 68 68 / 30%);
    color: var(--red);
  }

  .banner.info {
    background: rgb(251 191 36 / 10%); border-color: rgb(251 191 36 / 30%);
    color: #fbbf24;
  }

  .banner-close {
    background: none; border: none; padding: 4px;
    cursor: pointer; color: inherit; opacity: 0.7;
    display: flex; align-items: center; flex-shrink: 0; border-radius: 6px;
  }
  .banner-close:hover { opacity: 1; background: rgb(0 0 0 / 10%); }
</style>