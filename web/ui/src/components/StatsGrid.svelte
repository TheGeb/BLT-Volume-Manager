<script lang="ts">
  import type { StatsResponse } from '../lib/types';
  import { formatBytes, renderStatBar } from '../lib/util';
  import { onMount } from 'svelte';

  export let stats: StatsResponse | null = null;
  export let loading = false;

  let gridEl: HTMLDivElement;

  $: if (stats && gridEl) {
    renderCharts(stats);
  }

  function renderCharts(s: StatsResponse) {
    if (!gridEl) return;
    gridEl.innerHTML = '';
    gridEl.style.display = 'flex';
    gridEl.style.flexDirection = 'column';
    gridEl.style.gap = '16px';

    // Repo size card
    if (s.repo && !s.repo.error) {
      const repoCard = document.createElement('div');
      repoCard.className = 'stat-card-graph';
      const repoTitle = document.createElement('div');
      repoTitle.className = 'chart-title';
      repoTitle.textContent = 'Repo Size';
      repoCard.appendChild(repoTitle);

      const barParts: { value: number; color: string; label: string; display?: string }[] = [];
      if (s.repo.compressed_size && s.repo.total_uncompressed_size) {
        const saved = s.repo.total_uncompressed_size - s.repo.compressed_size;
        barParts.push({ value: s.repo.compressed_size, color: 'var(--accent)', label: 'Compressed', display: formatBytes(s.repo.compressed_size) });
        barParts.push({ value: saved, color: 'var(--surface-strong)', label: 'Space saved', display: formatBytes(saved) });
      }

      if (barParts.length > 0) {
        const chart = renderStatBar(barParts);
        repoCard.appendChild(chart);
      }

      const sub = document.createElement('div');
      sub.className = 'stat-sub';
      sub.innerHTML = `
        <div>Blobs: ${s.repo.total_blob_count?.toLocaleString() ?? '?'}</div>
      `;
      repoCard.appendChild(sub);
      gridEl.appendChild(repoCard);
    }

  }
</script>

<div id="volumeStatsGrid" class="stats-grid" bind:this={gridEl}>
  {#if !stats && !loading}
    <div style="text-align:center;padding:40px;color:var(--muted);">No stats available</div>
  {/if}
</div>
