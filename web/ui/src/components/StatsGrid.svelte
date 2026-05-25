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
  {#if stats?.repo && !stats.repo.error}
    <h2 class="panel-title">Repo Size</h2>
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
  {:else if !loading}
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

