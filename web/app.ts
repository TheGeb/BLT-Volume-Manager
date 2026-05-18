interface Snapshot {
  id: string;
  short_id: string;
  time: string;
  tags: string[];
  paths: string[];
}

interface LockStatus {
  volume: string;
  locked: boolean;
  owner?: string;
  expires_in?: number;
}

interface RepoStatus {
  initialized: boolean;
  hostname: string;
}

interface StatsResponse {
  snapshots: {
    total: number;
    hot: number;
    cold: number;
    excluded: number;
    volumes: number;
    newest: string;
    oldest: string;
    hot_volumes?: string[];
    cold_volumes?: string[];
    excluded_volumes?: string[];
    other_volumes?: string[];
  };
  repo: {
    total_size?: number;
    total_file_count?: number;
    total_blob_count?: number;
    total_uncompressed_size?: number;
    compressed_size?: number;
    unique_blob_count?: number;
    unique_blob_size?: number;
  };
  locks: {
    total_volumes: number;
    active: number;
    expired: number;
    unlocked: number;
    active_volumes?: string[];
    expired_volumes?: string[];
  };
  total_volumes?: number;
}

interface AppState {
  snapshots: Snapshot[];
  volumes: string[];
  selectedVolume: string;
  volumeFilter: string;
  query: string;
  sortNewestFirst: boolean;
  hostname: string;
  prevStats: StatsResponse | null;
}

const state: AppState = {
  snapshots: [],
  volumes: [],
  selectedVolume: '',
  volumeFilter: '',
  query: '',
  sortNewestFirst: true,
  hostname: 'webadmin',
  prevStats: null,
};

const snapshotTable = document.getElementById('snapshotTable') as HTMLTableSectionElement;
const searchInput = document.getElementById('searchInput') as HTMLInputElement;
const refreshButton = document.getElementById('refreshButton') as HTMLButtonElement;
const sortButton = document.getElementById('sortButton') as HTMLButtonElement;
const themeToggle = document.getElementById('themeToggle') as HTMLButtonElement;
const statusMessage = document.getElementById('statusMessage') as HTMLDivElement;
const volumePills = document.getElementById('volumePills') as HTMLDivElement;
const volumeFilterInput = document.getElementById('volumeFilterInput') as HTMLInputElement;
const createLockButton = document.getElementById('createLockButton') as HTMLButtonElement;
const deleteLocksButton = document.getElementById('deleteLocksButton') as HTMLButtonElement;
const lockPanel = document.getElementById('lockPanel') as HTMLElement;
const lockStatusText = document.getElementById('lockStatusText') as HTMLDivElement;
const lockOwner = document.getElementById('lockOwner') as HTMLDivElement;
const lockExpiry = document.getElementById('lockExpiry') as HTMLDivElement;
const repoInitBanner = document.getElementById('repoInitBanner') as HTMLDivElement;
const initRepoButton = document.getElementById('initRepoButton') as HTMLButtonElement;
const statsPanel = document.getElementById('statsPanel') as HTMLElement;
const statsGrid = document.getElementById('statsGrid') as HTMLDivElement;
const volumeView = document.getElementById('volumeView') as HTMLElement;
const lockPanelContent = document.getElementById('lockPanelContent') as HTMLElement;
const lockPanelSkeleton = document.getElementById('lockPanelSkeleton') as HTMLElement;
const themeIcon = document.getElementById('themeIcon') as HTMLElement;
const errorBanner = document.getElementById('errorBanner') as HTMLDivElement;
const errorBannerText = document.getElementById('errorBannerText') as HTMLSpanElement;

const moonSvg = '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>';
const sunSvg = themeIcon.innerHTML;

window.addEventListener('load', () => {
  searchInput.addEventListener('input', () => {
    state.query = searchInput.value.trim().toLowerCase();
    renderTable();
  });

  volumeFilterInput.addEventListener('input', () => {
    state.volumeFilter = volumeFilterInput.value.trim().toLowerCase();
    renderVolumePills();
  });

  volumePills.addEventListener('click', (e) => {
    const pill = (e.target as HTMLElement).closest('.volume-pill') as HTMLElement | null;
    if (!pill) return;
    const vol = pill.dataset.volume || '';
    state.selectedVolume = vol === state.selectedVolume ? '' : vol;
    volumeFilterInput.value = '';
    state.volumeFilter = '';
    renderVolumePills();
    onVolumeChange();
  });

  createLockButton.addEventListener('click', () => {
    createVolumeLock(state.selectedVolume);
  });

  deleteLocksButton.addEventListener('click', () => {
    if (!confirm(`Delete all locks for volume ${state.selectedVolume}?`)) {
      return;
    }
    deleteVolumeLocks(state.selectedVolume);
  });

  refreshButton.addEventListener('click', async () => {
    setErrorBanner('');
    try {
      await fetch('/api/stats/refresh', { method: 'POST' });
    } catch {
      // fall through
    }
    loadStats();
    loadSnapshots();
    if (state.selectedVolume) refreshLockStatus();
  });

  sortButton.addEventListener('click', () => {
    state.sortNewestFirst = !state.sortNewestFirst;
    sortButton.textContent = state.sortNewestFirst ? 'Sort by newest' : 'Sort by oldest';
    renderTable();
  });

  initRepoButton.addEventListener('click', initRepo);

  themeToggle.addEventListener('click', () => {
    const isLight = document.body.classList.toggle('light');
    themeIcon.innerHTML = isLight ? moonSvg : sunSvg;
  });

  showPillsSkeleton();
  Promise.all([
    checkRepoStatus(),
    loadPills(),
    loadSnapshots(),
    loadStats(),
  ]);
});

function onVolumeChange(): void {
  if (state.selectedVolume) {
    statsPanel.style.display = 'none';
    volumeView.style.display = 'grid';
    lockPanel.style.display = 'flex';
    renderTable();
    refreshLockStatus();
  } else {
    statsPanel.style.display = '';
    volumeView.style.display = 'none';
    lockPanel.style.display = 'none';
    renderTable();
  }
}

async function loadStats(): Promise<void> {
  const hasData = statsGrid.querySelector('.stat-card') !== null || statsGrid.querySelector('.stat-card-graph') !== null;
  if (!hasData) showStatsSkeleton();
  try {
    const response = await fetch('/api/stats');
    if (response.ok) {
      const data = await response.json() as StatsResponse;
      renderStats(data);
    }
  } catch {
    if (!hasData) {
      const msg = 'Cannot reach server for stats';
      showStatus(msg, true);
      setErrorBanner(msg);
    }
  }
}

function showPillsSkeleton(): void {
  volumePills.innerHTML = '';
  const pillWidth = 100;
  const gap = 8;
  const containerWidth = volumePills.offsetWidth || 600;
  const perRow = Math.max(1, Math.floor((containerWidth + gap) / (pillWidth + gap)));
  const rows = 2;
  for (let i = 0; i < perRow * rows; i++) {
    const pill = document.createElement('div');
    pill.className = 'skeleton skeleton-pill';
    pill.style.width = `${pillWidth}px`;
    volumePills.appendChild(pill);
  }
}

function showStatsSkeleton(): void {
  statsGrid.innerHTML = '';
  for (let i = 0; i < 6; i++) {
    const card = document.createElement('div');
    card.className = 'stat-card skeleton skeleton-card';
    statsGrid.appendChild(card);
  }
  for (let i = 0; i < 3; i++) {
    const card = document.createElement('div');
    card.className = 'stat-card skeleton skeleton-card-graph';
    statsGrid.appendChild(card);
  }
}

function showTableSkeleton(): void {
  snapshotTable.innerHTML = '';
  for (let i = 0; i < 5; i++) {
    const row = document.createElement('tr');
    const td = document.createElement('td');
    td.colSpan = 5;
    td.innerHTML = '<div class="skeleton skeleton-row"></div>';
    row.appendChild(td);
    snapshotTable.appendChild(row);
  }
}

function showLockSkeleton(): void {
  if (lockTimer) { clearInterval(lockTimer); lockTimer = null; }
  lockPanelContent.style.display = 'none';
  lockPanelSkeleton.style.display = 'block';
}

function formatBytes(b: number): string {
  if (b <= 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), units.length - 1);
  const val = b / Math.pow(1024, i);
  const formatted = val.toFixed(2).replace(/\.?0+$/, '');
  return `${formatted} ${units[i]}`;
}

function renderStatBar(parts: { value: number; color: string; label: string; names?: string[]; display?: string }[]): HTMLDivElement {
  const total = parts.reduce((s, p) => s + p.value, 0) || 1;
  const bar = document.createElement('div');
  bar.className = 'bar-stacked';
  parts.forEach(p => {
    const seg = document.createElement('div');
    seg.className = 'bar-segment';
    const pct = (p.value / total) * 100;
    seg.style.flex = `${pct} 1 0`;
    seg.style.background = p.color;
    if (pct > 15 && p.value > 0) seg.textContent = p.display ?? String(p.value);
    if (p.names && p.names.length > 0) {
      seg.title = p.names.join('\n');
    }
    bar.appendChild(seg);
  });

  const legend = document.createElement('div');
  legend.className = 'bar-legend';
  parts.forEach(p => {
    const item = document.createElement('div');
    item.className = 'bar-legend-item';
    item.innerHTML = `<span class="bar-legend-dot" style="background:${p.color}"></span>${p.label}`;
    legend.appendChild(item);
  });

  const wrapper = document.createElement('div');
  wrapper.appendChild(bar);
  wrapper.appendChild(legend);
  return wrapper;
}

function fadeIn(el: HTMLElement, fn: () => void): void {
  el.style.opacity = '0';
  fn();
  requestAnimationFrame(() => { el.style.opacity = '1'; });
}

function animateStatValue(el: HTMLElement, oldVal: number, newVal: number): void {
  const increased = newVal > oldVal;
  const style = getComputedStyle(el);
  const height = el.offsetHeight || parseInt(style.fontSize) * 1.4 || 32;

  el.style.position = 'relative';
  el.style.overflow = 'hidden';
  el.style.height = `${height}px`;
  el.style.display = 'inline-block';
  el.style.verticalAlign = 'baseline';
  el.innerHTML = '';

  const oldEl = document.createElement('span');
  oldEl.textContent = String(oldVal);
  oldEl.style.cssText = `position:absolute;left:0;right:0;top:0;transition:transform 0.35s ease;transform:translateY(0)`;

  const newEl = document.createElement('span');
  newEl.textContent = String(newVal);
  newEl.style.cssText = `position:absolute;left:0;right:0;top:0;transition:transform 0.35s ease`;
  newEl.style.transform = increased ? 'translateY(100%)' : 'translateY(-100%)';

  el.appendChild(oldEl);
  el.appendChild(newEl);

  requestAnimationFrame(() => {
    if (increased) {
      oldEl.style.transform = 'translateY(-100%)';
      newEl.style.transform = 'translateY(0)';
    } else {
      oldEl.style.transform = 'translateY(100%)';
      newEl.style.transform = 'translateY(0)';
    }
  });

  const done = () => {
    el.style.position = '';
    el.style.overflow = '';
    el.style.height = '';
    el.style.display = '';
    el.style.verticalAlign = '';
    el.textContent = String(newVal);
  };
  newEl.addEventListener('transitionend', done, { once: true });
  setTimeout(done, 500);
}

function renderStats(data: StatsResponse): void {
  fadeIn(statsGrid, () => {
  statsGrid.innerHTML = '';

  const prev = state.prevStats;
  state.prevStats = data;

  const smallCards: { value: string | number; label: string; oldValue?: number }[] = [
    { value: data.snapshots.total ?? '-', label: 'Snapshots', oldValue: prev?.snapshots.total },
    { value: data.total_volumes ?? '-', label: 'Volumes', oldValue: prev?.total_volumes },
  ];
  if (data.repo.total_file_count != null) {
    smallCards.push({ value: data.repo.total_file_count, label: 'Files', oldValue: prev?.repo.total_file_count });
  }
  if (data.repo.unique_blob_count != null) {
    smallCards.push({ value: data.repo.unique_blob_count, label: 'Unique blobs', oldValue: prev?.repo.unique_blob_count });
  }
  if (data.repo.compressed_size != null) {
    smallCards.push({ value: formatBytes(data.repo.compressed_size), label: 'Repo size' });
  }

  smallCards.forEach(c => {
    const card = document.createElement('div');
    card.className = 'stat-card';
    card.innerHTML = `<div class="stat-value">${c.value}</div><div class="stat-label">${c.label}</div>`;
    statsGrid.appendChild(card);
    const sv = card.querySelector('.stat-value') as HTMLElement;
    if (c.oldValue != null && typeof c.value === 'number' && c.oldValue !== c.value) {
      animateStatValue(sv, c.oldValue, c.value);
    }
  });

  const total = data.snapshots.hot + data.snapshots.cold + data.snapshots.excluded;
  const other = data.snapshots.total - total;
  if (data.snapshots.total > 0) {
    const card = document.createElement('div');
    card.className = 'stat-card-graph';
    card.innerHTML = '<div class="chart-title">Snapshot tags</div>';
    card.appendChild(renderStatBar([
      { value: data.snapshots.hot, color: 'var(--orange)', label: 'Hot', names: data.snapshots.hot_volumes },
      { value: data.snapshots.cold, color: 'var(--blue)', label: 'Cold', names: data.snapshots.cold_volumes },
      { value: data.snapshots.excluded, color: 'var(--red)', label: 'Excluded', names: data.snapshots.excluded_volumes },
      { value: Math.max(other, 0), color: 'var(--muted)', label: 'Other', names: data.snapshots.other_volumes },
    ]));
    statsGrid.appendChild(card);
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
    statsGrid.appendChild(card);
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
    statsGrid.appendChild(card);
  }

  const parent = statsGrid.parentElement!;
  parent.querySelectorAll('.stat-sub').forEach(el => el.remove());
  if (data.snapshots.newest || data.snapshots.oldest) {
    const sub = document.createElement('div');
    sub.className = 'stat-sub';
    sub.textContent = `Newest: ${data.snapshots.newest ? new Date(data.snapshots.newest).toLocaleString() : '-'}  ·  Oldest: ${data.snapshots.oldest ? new Date(data.snapshots.oldest).toLocaleString() : '-'}`;
    parent.appendChild(sub);
  }
  });
}

async function loadPills(): Promise<void> {
  showPillsSkeleton();
  try {
    const response = await fetch('/api/pills');
    if (response.ok) {
      const data = await response.json();
      state.volumes = data.volumes;
      renderVolumePills();
    }
  } catch {
    // fall through
  }
}

async function loadSnapshots(): Promise<void> {
  const hasData = !!snapshotTable.querySelector('td');
  if (!hasData) showTableSkeleton();

  try {
    const response = await fetch('/api/snapshots');
    if (!response.ok) {
      let msg = 'Unable to load snapshots';
      try {
        const body = await response.json();
        if (body.error) msg = body.error;
      } catch {}
      throw new Error(msg);
    }

    const raw = await response.json() as Snapshot[];
    state.snapshots = raw.map(sn => ({
      ...sn,
      tags: Array.isArray(sn.tags) ? sn.tags : [],
      paths: Array.isArray(sn.paths) ? sn.paths : [],
    }));
    renderTable();
    showStatus(`Loaded ${state.snapshots.length} snapshots.`);
    setErrorBanner('');
  } catch (error) {
    showStatus((error as Error).message, true);
    setErrorBanner((error as Error).message);
  }
}

function renderVolumePills(): void {
  const filter = state.volumeFilter;
  const matched = filter
    ? state.volumes.filter(v => v.toLowerCase().includes(filter))
    : state.volumes;

  fadeIn(volumePills, () => {
  volumePills.innerHTML = '';

  matched.forEach(volume => {
    const pill = document.createElement('button');
    pill.className = 'volume-pill' + (volume === state.selectedVolume ? ' active' : '');
    pill.dataset.volume = volume;
    pill.textContent = volume;
    volumePills.appendChild(pill);
  });
  });
}

function extractVolumeName(path: string): string {
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

function renderTable(): void {
  const fromSkeleton = !!snapshotTable.querySelector('.skeleton');
  const oldH = fromSkeleton ? snapshotTable.offsetHeight : 0;

  fadeIn(snapshotTable, () => {
  snapshotTable.innerHTML = '';

  const filtered = state.snapshots
    .filter(snapshot => {
      if (state.selectedVolume && !snapshot.paths.some(path => extractVolumeName(path) === state.selectedVolume)) {
        return false;
      }
      if (!state.query) {
        return true;
      }
      const pathText = snapshot.paths.join(' ');
      const tagText = snapshot.tags.join(' ');
      return [snapshot.id, snapshot.short_id, pathText, tagText]
        .join(' ')
        .toLowerCase()
        .includes(state.query);
    })
    .sort((a, b) => {
      const aTime = new Date(a.time).getTime();
      const bTime = new Date(b.time).getTime();
      return state.sortNewestFirst ? bTime - aTime : aTime - bTime;
    });

  if (filtered.length === 0) {
    snapshotTable.innerHTML = '<tr><td colspan="5">No backups match the current filter.</td></tr>';
    return;
  }

  filtered.forEach(snapshot => {
    const row = document.createElement('tr');

    const idCell = document.createElement('td');
    idCell.className = 'copy-id';
    idCell.title = 'Click to copy full snapshot ID';
    idCell.textContent = snapshot.short_id;
    idCell.addEventListener('click', () => {
      navigator.clipboard.writeText(snapshot.id);
      showStatus('Snapshot ID copied to clipboard');
      idCell.classList.add('copied');
      setTimeout(() => idCell.classList.remove('copied'), 1500);
    });
    row.appendChild(idCell);

    const pathsCell = document.createElement('td');
    pathsCell.textContent = snapshot.paths.join(', ');
    row.appendChild(pathsCell);

    const tagsCell = document.createElement('td');
    const tagList = document.createElement('div');
    tagList.className = 'tag-list';
    const visibleTags = snapshot.tags.filter(t => t === 'hot' || t === 'cold');
    const isExcluded = snapshot.tags.includes('excluded');
    if (visibleTags.length === 0) {
      tagList.textContent = 'No tags';
    } else {
      visibleTags.forEach(tag => {
        const tagItem = document.createElement('span');
        tagItem.className = 'tag tag-readonly';
        tagItem.textContent = tag;
        tagList.appendChild(tagItem);
      });
    }
    const label = document.createElement('label');
    label.className = 'tag-checkbox';
    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.checked = isExcluded;
    cb.addEventListener('change', () => {
      if (cb.checked) {
        addTag(snapshot.id, 'excluded');
      } else {
        removeTag(snapshot.id, 'excluded');
      }
    });
    label.appendChild(cb);
    label.appendChild(document.createTextNode(' excluded'));
    tagList.appendChild(label);
    tagsCell.appendChild(tagList);
    row.appendChild(tagsCell);

    const timeCell = document.createElement('td');
    timeCell.textContent = new Date(snapshot.time).toLocaleString();
    row.appendChild(timeCell);

    row.appendChild(document.createElement('td'));
    snapshotTable.appendChild(row);
  });

  if (fromSkeleton && oldH > 0) {
    const newH = snapshotTable.offsetHeight;
    if (newH !== oldH) {
      snapshotTable.style.height = `${oldH}px`;
      requestAnimationFrame(() => {
        snapshotTable.style.transition = 'height 0.25s ease';
        snapshotTable.style.height = `${newH}px`;
      });
      setTimeout(() => {
        snapshotTable.style.height = '';
        snapshotTable.style.transition = '';
      }, 260);
    }
  }
  });
}

async function addTag(snapshotID: string, tag: string): Promise<void> {
  showStatus(`Adding tag "${tag}"...`);
  try {
    const response = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}`, {
      method: 'POST',
    });
    if (!response.ok) {
      throw new Error('Failed to add tag');
    }
    await loadSnapshots();
  } catch (error) {
    showStatus((error as Error).message, true);
  }
}

async function removeTag(snapshotID: string, tag: string): Promise<void> {
  showStatus(`Removing tag "${tag}"...`);
  try {
    const response = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      throw new Error('Failed to remove tag');
    }
    await loadSnapshots();
  } catch (error) {
    showStatus((error as Error).message, true);
  }
}

function formatDuration(totalSeconds: number): string {
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

let lockTimer: ReturnType<typeof setInterval> | null = null;

function renderLockInfo(data: LockStatus): void {
  lockPanelSkeleton.style.display = 'none';
  lockPanelContent.style.display = '';

  if (lockTimer) { clearInterval(lockTimer); lockTimer = null; }

  // Animate content height when switching between locked/unlocked states
  const currentH = lockPanelContent.offsetHeight;
  lockPanelContent.style.transition = 'none';
  lockPanelContent.style.height = '';

  if (data.locked) {
    lockStatusText.textContent = 'Locked';
    lockStatusText.style.color = 'var(--yellow)';
    lockOwner.textContent = `by ${data.owner}`;
    const tick = () => {
      if (data.expires_in != null && data.expires_in > 0) {
        data.expires_in--;
        lockExpiry.textContent = formatDuration(data.expires_in);
      } else {
        lockExpiry.textContent = 'expired';
      }
    };
    tick();
    lockTimer = setInterval(tick, 1000);
  } else {
    lockStatusText.textContent = 'Unlocked';
    lockStatusText.style.color = 'var(--green)';
    lockOwner.textContent = '';
    lockExpiry.textContent = '';
  }

  deleteLocksButton.disabled = !data.locked;

  const newH = lockPanelContent.scrollHeight;
  if (currentH > 0 && newH !== currentH) {
    lockPanelContent.style.height = `${currentH}px`;
    requestAnimationFrame(() => {
      lockPanelContent.style.transition = 'height 0.25s ease';
      lockPanelContent.style.height = `${newH}px`;
    });
    setTimeout(() => {
      lockPanelContent.style.height = '';
      lockPanelContent.style.transition = '';
    }, 260);
  }
}

async function refreshLockStatus(): Promise<void> {
  if (!state.selectedVolume) return;
  showLockSkeleton();
  try {
    const response = await fetch(`/api/volume/${encodeURIComponent(state.selectedVolume)}/locks`);
    if (!response.ok) {
      lockStatusText.textContent = 'Error';
      lockOwner.textContent = '';
      lockExpiry.textContent = '';
      deleteLocksButton.disabled = false;
      return;
    }
    const data = await response.json() as LockStatus;
    renderLockInfo(data);
  } catch {
    lockStatusText.textContent = 'Error';
    lockOwner.textContent = '';
    lockExpiry.textContent = '';
    deleteLocksButton.disabled = false;
  }
}

async function createVolumeLock(volumeName: string): Promise<void> {
  const ownerName = prompt('Lock owner name:', state.hostname);
  if (!ownerName) return;
  showStatus(`Creating lock for volume ${volumeName}...`);
  try {
    const response = await fetch(`/api/volume/${encodeURIComponent(volumeName)}/locks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ owner: ownerName }),
    });
    if (!response.ok) {
      const body = await response.json();
      throw new Error(body.error || 'Failed to create lock');
    }
    showStatus(`Lock created for volume ${volumeName}.`);
    await refreshLockStatus();
  } catch (error) {
    showStatus((error as Error).message, true);
  }
}

async function deleteVolumeLocks(volumeName: string): Promise<void> {
  showStatus(`Deleting locks for volume ${volumeName}...`);
  try {
    const response = await fetch(`/api/volume/${encodeURIComponent(volumeName)}/locks`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      const body = await response.json();
      throw new Error(body.error || 'Failed to delete locks');
    }
    showStatus(`Deleted locks for volume ${volumeName}.`);
    await refreshLockStatus();
  } catch (error) {
    showStatus((error as Error).message, true);
  }
}

async function checkRepoStatus(): Promise<void> {
  try {
    const response = await fetch('/api/repo/status');
    if (!response.ok) {
      let msg = 'Failed to check repository status';
      try {
        const body = await response.json();
        if (body.error) msg = body.error;
      } catch {}
      showStatus(msg, true);
      setErrorBanner(msg);
      return;
    }
    const data = await response.json() as RepoStatus;
    repoInitBanner.style.display = data.initialized ? 'none' : 'flex';
    if (data.hostname) state.hostname = data.hostname;
    if (data.initialized === false) {
      setErrorBanner('');
    }
  } catch {
    const msg = 'Cannot reach server';
    showStatus(msg, true);
    setErrorBanner(msg);
  }
}

async function initRepo(): Promise<void> {
  initRepoButton.disabled = true;
  initRepoButton.textContent = 'Initializing...';
  try {
    const response = await fetch('/api/repo/init', { method: 'POST' });
    if (!response.ok) {
      const body = await response.json();
      throw new Error(body.error || 'Failed to initialize repository');
    }
    repoInitBanner.style.display = 'none';
    loadSnapshots();
    loadStats();
  } catch (error) {
    showStatus((error as Error).message, true);
  } finally {
    initRepoButton.disabled = false;
    initRepoButton.textContent = 'Initialize Repository';
  }
}

function showStatus(message: string, isError: boolean = false): void {
  statusMessage.textContent = message;
  statusMessage.style.color = isError ? 'var(--red)' : '';
}

function setErrorBanner(msg: string): void {
  if (msg) {
    errorBannerText.textContent = msg;
    errorBanner.style.display = '';
  } else {
    errorBanner.style.display = 'none';
  }
}
