<script lang="ts">
  import type { StatsResponse } from '$lib/types';
  import { formatBytes } from '$lib/util';

  let { stats = null as StatsResponse | null, loading = false }: {
    stats?: StatsResponse | null;
    loading?: boolean;
  } = $props();

  let barParts = $derived(
    stats?.repo?.total_size && stats?.repo?.total_uncompressed_size
      ? [
          { value: stats.repo.total_size, color: 'var(--accent)', label: 'On Disk', display: formatBytes(stats.repo.total_size) },
          { value: stats.repo.total_uncompressed_size - stats.repo.total_size, color: 'var(--accent-complement)', label: 'Space saved', display: formatBytes(stats.repo.total_uncompressed_size - stats.repo.total_size) },
        ]
      : []
  );

  let total = $derived(barParts.reduce((s, p) => s + p.value, 0) || 1);
</script>

<section class="panel panel-layout">
  {#if loading && !stats}
    <h2 class="panel-title">Storage</h2>
    <div style="padding: 4px 0;">
      <div class="skeleton" style="height:32px;width:160px;border-radius:8px;margin-bottom:8px;"></div>
      <div class="skeleton" style="height:18px;width:120px;border-radius:6px;margin-bottom:16px;"></div>
      <div class="skeleton" style="height:40px;border-radius:8px;"></div>
    </div>
  {:else if stats?.repo}
    <h2 class="panel-title">Storage</h2>
    <div class="panel-info">
      <div class="panel-info-primary">{stats.repo.total_uncompressed_size ? formatBytes(stats.repo.total_uncompressed_size) : '?'}</div>
      <div class="panel-info-secondary">(Uncompressed)</div>
    </div>
    {#if barParts.length > 0}
      <div style="margin-top:auto;">
        <div class="bar-wrapper">
          <div class="bar-legend">
            {#each barParts as p (p.label)}
              <div class="bar-legend-item">
                <span class="bar-legend-dot" style="background:{p.color}"></span>
                {p.label}
              </div>
            {/each}
          </div>
          <div class="bar-stacked">
            {#each barParts as p (p.label)}
              {@const pct = (p.value / total) * 100}
              <div class="bar-segment" style="flex:{pct} 1 0;background:{p.color};">
                {#if pct > 15 && p.value > 0}{p.display ?? p.value}{/if}
              </div>
            {/each}
          </div>
        </div>
      </div>
    {/if}
  {:else}
    <div style="text-align:center;padding:40px;color:var(--muted);">No stats available</div>
  {/if}
</section>

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
    color: rgb(0 0 0 / 70%);
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
