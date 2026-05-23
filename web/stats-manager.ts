/// <reference path="types.ts" />
/// <reference path="util.ts" />

class StatsManager {
  private grid: HTMLDivElement;
  private getPrev: () => StatsResponse | null;
  private setPrev: (s: StatsResponse | null) => void;

  constructor(grid: HTMLDivElement, getPrev: () => StatsResponse | null, setPrev: (s: StatsResponse | null) => void) {
    this.grid = grid;
    this.getPrev = getPrev;
    this.setPrev = setPrev;
  }

  showSkeleton(): void {
    this.grid.innerHTML = '';
    for (let i = 0; i < 2; i++) {
      const card = document.createElement('div');
      card.className = 'stat-card skeleton skeleton-card-graph';
      this.grid.appendChild(card);
    }
  }

  async load(vol: string, retries = 5, delay = 1000): Promise<void> {
    const hasData = this.grid.querySelector('.stat-card') !== null || this.grid.querySelector('.stat-card-graph') !== null;
    if (!hasData) this.showSkeleton();
    for (let attempt = 0; attempt < retries; attempt++) {
      try {
        const resp = await fetch(`/api/stats?volume=${encodeURIComponent(vol)}`);
        if (!resp.ok) {
          if (attempt < retries - 1) { await this.sleep(delay); continue; }
          if (!hasData) throw new Error('Stats not yet available');
          return;
        }
        const data = await resp.json() as StatsResponse;
        if (!data || !data.snapshots) {
          if (attempt < retries - 1) { await this.sleep(delay); continue; }
          if (!hasData) throw new Error('Stats not yet available');
          return;
        }
        this.render(data);
        return;
      } catch {
        if (!hasData && attempt >= retries - 1) throw new Error('Cannot reach server for stats');
        await this.sleep(delay);
      }
    }
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(r => setTimeout(r, ms));
  }

  render(data: StatsResponse): void {
    fadeIn(this.grid, () => {
    this.grid.innerHTML = '';

    const prev = this.getPrev();
    this.setPrev(data);

    if (data.repo.error) {
      const errCard = document.createElement('div');
      errCard.className = 'stat-card';
      errCard.style.cssText = 'grid-column:1/-1;color:var(--red);font-size:0.85rem;text-align:center;padding:12px;';
      errCard.textContent = `Stats unavailable: ${data.repo.error}`;
      this.grid.appendChild(errCard);
    }

    if (data.locks.total_volumes > 0) {
      const card = document.createElement('div');
      card.className = 'stat-card-graph';
      card.innerHTML = '<div class="chart-title">Lock status</div>';
      card.appendChild(renderStatBar([
        { value: data.locks.active, color: 'var(--green)', label: 'Active', names: data.locks.active_volumes },
        { value: data.locks.expired, color: 'var(--yellow)', label: 'Expired', names: data.locks.expired_volumes },
        { value: data.locks.unlocked, color: 'var(--muted)', label: 'Unlocked' },
      ]));
      this.grid.appendChild(card);
    }

    if (data.repo.compressed_size != null && data.repo.total_uncompressed_size != null) {
      const card = document.createElement('div');
      card.className = 'stat-card-graph';
      card.innerHTML = '<div class="chart-title">Storage</div>';
      const compressed = data.repo.compressed_size;
      const uncompressed = data.repo.total_uncompressed_size;
      const saved = uncompressed - compressed;
      card.appendChild(renderStatBar([
        { value: compressed, color: 'var(--accent)', label: `Compressed (${formatBytes(compressed)})`, display: formatBytes(compressed) },
        { value: Math.max(saved, 0), color: 'var(--green)', label: `Saved (${formatBytes(Math.max(saved, 0))})`, display: formatBytes(Math.max(saved, 0)) },
      ]));
      const ratio = uncompressed > 0 ? (compressed / uncompressed * 100).toFixed(1) : '-';
      const sub = document.createElement('div');
      sub.style.cssText = 'font-size:0.8rem;color:var(--muted);margin-top:8px;text-align:center;';
      if (compressed === 0 && uncompressed === 0) {
        sub.textContent = 'No data';
      } else {
        sub.textContent = `${formatBytes(uncompressed)} uncompressed · ${ratio}% compression ratio`;
      }
      card.appendChild(sub);
      this.grid.appendChild(card);
    }

    const parent = this.grid.parentElement!;
    parent.querySelectorAll('.stat-sub').forEach(el => el.remove());
    const parts: string[] = [];
    if (data.snapshots.newest || data.snapshots.oldest) {
      parts.push(`Newest: ${data.snapshots.newest ? new Date(data.snapshots.newest).toLocaleString() : '-'}  ·  Oldest: ${data.snapshots.oldest ? new Date(data.snapshots.oldest).toLocaleString() : '-'}`);
    }
    if (data.cached_at) {
      parts.push(`Cached: ${new Date(data.cached_at).toLocaleString()}`);
    }
    if (parts.length > 0) {
      const sub = document.createElement('div');
      sub.className = 'stat-sub';
      sub.textContent = parts.join('  ·  ');
      parent.appendChild(sub);
    }
    });
  }
}
