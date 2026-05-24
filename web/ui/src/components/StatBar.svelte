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
