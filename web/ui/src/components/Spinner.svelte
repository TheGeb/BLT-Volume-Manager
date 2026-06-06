<script lang="ts">
  let { size = 24, strokeWidth = 3, style = '' }: {
    size?: number;
    strokeWidth?: number;
    style?: string;
  } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width={strokeWidth} stroke-linecap="round" class="spin" style="vertical-align:middle;{style}">
  <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
  <path d="M12 2a10 10 0 0 1 10 10" />
</svg>
