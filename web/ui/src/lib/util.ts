export function formatBytes(b: number): string {
  if (b <= 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), units.length - 1);
  const val = b / Math.pow(1024, i);
  const formatted = val.toFixed(2).replace(/\.?0+$/, '');
  return `${formatted} ${units[i]}`;
}

export function formatDuration(totalSeconds: number): string {
  if (totalSeconds <= 0) return 'expired';
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const secs = totalSeconds % 60;
  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0 || days > 0) parts.push(`${hours}h`);
  if (minutes > 0 || hours > 0 || days > 0) parts.push(`${minutes}m`);
  parts.push(`${secs}s`);
  return parts.join(' ') + ' remaining';
}

export function extractVolumeName(path: string): string {
  const marker = '/volumes/';
  const idx = path.indexOf(marker);
  if (idx >= 0) {
    const subpath = path.slice(idx + marker.length).replace(/^\//, '');
    const parts = subpath.split('/');
    return parts[0] || '';
  }
  const parts = path.split('/').filter(Boolean);
  return parts.length ? parts[parts.length - 1] : '';
}

export function escapeHtml(s: string): string {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

export function renderStatBar(
  parts: { value: number; color: string; label: string; names?: string[]; display?: string }[]
): HTMLDivElement {
  const total = parts.reduce((s, p) => s + p.value, 0) || 1;
  const bar = document.createElement('div');
  bar.className = 'bar-stacked';
  for (const p of parts) {
    const seg = document.createElement('div');
    seg.className = 'bar-segment';
    const pct = (p.value / total) * 100;
    seg.style.flex = `${pct} 1 0`;
    seg.style.background = p.color;
    if (pct > 15 && p.value > 0) seg.textContent = p.display ?? String(p.value);
    if (p.names && p.names.length > 0) seg.title = p.names.join('\n');
    bar.appendChild(seg);
  }
  const legend = document.createElement('div');
  legend.className = 'bar-legend';
  for (const p of parts) {
    const item = document.createElement('div');
    item.className = 'bar-legend-item';
    item.innerHTML = `<span class="bar-legend-dot" style="background:${p.color}"></span>${p.label}`;
    legend.appendChild(item);
  }
  const wrapper = document.createElement('div');
  wrapper.className = 'bar-wrapper';
  wrapper.appendChild(legend);
  wrapper.appendChild(bar);
  return wrapper;
}
