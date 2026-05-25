<script lang="ts">
  import type { StatsResponse } from '../lib/types';
  import { formatBytes } from '../lib/util';
  import StatBar from './StatBar.svelte';

  export let stats: StatsResponse | null = null;
  export let loading = false;

  $: barParts = stats?.repo?.compressed_size && stats?.repo?.total_uncompressed_size
    ? [
        { value: stats.repo.compressed_size, color: 'var(--accent)', label: 'On Disk', display: formatBytes(stats.repo.compressed_size) },
        { value: stats.repo.total_uncompressed_size - stats.repo.compressed_size, color: 'var(--blue)', label: 'Space saved', display: formatBytes(stats.repo.total_uncompressed_size - stats.repo.compressed_size) },
      ]
    : [];
</script>

<section class="panel panel-layout">
  {#if loading && !stats}
    <h2 class="panel-title">Storage</h2>
    <div style="padding: 4px 0;">
      <div class="skeleton" style="height:32px;width:160px;border-radius:8px;margin-bottom:8px;"></div>
      <div class="skeleton" style="height:18px;width:120px;border-radius:6px;margin-bottom:16px;"></div>
      <div class="skeleton" style="height:40px;border-radius:8px;margin-bottom:16px;"></div>
      <div class="skeleton" style="height:16px;width:100px;border-radius:6px;"></div>
    </div>
  {:else if stats?.repo && !stats.repo.error}
    <h2 class="panel-title">Storage</h2>
    <div class="panel-info">
      <div class="panel-info-primary">{stats.repo.total_uncompressed_size ? formatBytes(stats.repo.total_uncompressed_size) : '?'}</div>
      <div class="panel-info-secondary">(Uncompressed)</div>
    </div>
    {#if barParts.length > 0}
      <StatBar parts={barParts} />
    {/if}
    <div class="stat-sub">
      <div>Blobs: {stats.repo.total_blob_count?.toLocaleString() ?? '?'}</div>
    </div>
  {:else if stats?.repo?.error}
    <h2 class="panel-title">Storage</h2>
    <div style="color:var(--red);font-size:0.85rem;padding:12px 0;">
      {stats.repo.error}
    </div>
  {:else}
    <div style="text-align:center;padding:40px;color:var(--muted);">No stats available</div>
  {/if}
</section>

<style>
  .stat-sub {
    font-size: 0.85rem;
    color: var(--muted);
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }
</style>