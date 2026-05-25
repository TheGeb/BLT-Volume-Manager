<script lang="ts">
  export let parts: { value: number; color: string; label: string; names?: string[]; display?: string }[] = [];

  $: total = parts.reduce((s, p) => s + p.value, 0) || 1;
</script>

<div class="bar-wrapper">
  <div class="bar-legend">
    {#each parts as p}
      <div class="bar-legend-item">
        <span class="bar-legend-dot" style="background:{p.color}"></span>
        {p.label}
      </div>
    {/each}
  </div>
  <div class="bar-stacked">
    {#each parts as p}
      {@const pct = (p.value / total) * 100}
      <div class="bar-segment" style="flex:{pct} 1 0;background:{p.color};"
        title={p.names?.join('\n') ?? ''}>
        {#if pct > 15 && p.value > 0}{p.display ?? p.value}{/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .bar-wrapper {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 10px;
    justify-content: center;
  }

  .bar-stacked {
    display: flex;
    height: 48px;
    border-radius: 8px;
    overflow: hidden;
  }

  .bar-segment {
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 600;
    color: rgba(0, 0, 0, 0.7);
    transition: flex 0.3s ease;
    min-width: 0;
  }

  .bar-segment:first-child {
    border-radius: 8px 0 0 8px;
  }

  .bar-segment:last-child {
    border-radius: 0 8px 8px 0;
  }

  .bar-segment:only-child {
    border-radius: 8px;
  }

  .bar-legend {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    font-size: 0.8rem;
  }

  .bar-legend-item {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
  }

  .bar-legend-dot {
    width: 10px;
    height: 10px;
    border-radius: 3px;
    flex-shrink: 0;
  }
</style>
