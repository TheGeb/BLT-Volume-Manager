<script lang="ts">
  export let bannerText = '';
  export let bannerError = false;
</script>

<div class="banner" class:visible={bannerText.length > 0} class:error={bannerError} class:info={bannerText.length > 0 && !bannerError}>
  {bannerText}
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
</style>
