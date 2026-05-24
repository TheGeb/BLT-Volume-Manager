<script lang="ts">
  import type { StatsResponse } from '../lib/types';
  import { formatBytes, renderStatBar } from '../lib/util';

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
    gridEl.style.flex = '1';

    if (s.repo && !s.repo.error) {
      const repoCard = document.createElement('div');
      repoCard.className = 'stat-card-graph';

      const repoTitle = document.createElement('h2');
      repoTitle.className = 'lock-panel-title';
      repoTitle.textContent = 'Repo Size';
      repoCard.appendChild(repoTitle);

      const info = document.createElement('div');
      info.className = 'lock-info';

      const totalSize = document.createElement('div');
      totalSize.className = 'lock-status-text';
      totalSize.textContent = s.repo.total_uncompressed_size ? formatBytes(s.repo.total_uncompressed_size) : '?';
      info.appendChild(totalSize);

      const subLabel = document.createElement('div');
      subLabel.className = 'lock-owner';
      subLabel.textContent = '(Uncompressed)';
      info.appendChild(subLabel);

      repoCard.appendChild(info);

      const barParts: { value: number; color: string; label: string; display?: string }[] = [];
      if (s.repo.compressed_size && s.repo.total_uncompressed_size) {
        const saved = s.repo.total_uncompressed_size - s.repo.compressed_size;
        barParts.push({ value: s.repo.compressed_size, color: 'var(--accent)', label: 'On Disk', display: formatBytes(s.repo.compressed_size) });
        barParts.push({ value: saved, color: 'var(--blue)', label: 'Space saved', display: formatBytes(saved) });
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

<section class="panel" style="display:flex;flex-direction:column;">
  <div id="volumeStatsGrid" bind:this={gridEl} style="display:flex;flex-direction:column;flex:1;">
    {#if !stats && !loading}
      <div style="text-align:center;padding:40px;color:var(--muted);">No stats available</div>
    {/if}
  </div>
</section>
