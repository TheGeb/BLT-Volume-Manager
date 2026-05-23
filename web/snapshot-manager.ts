/// <reference path="types.ts" />
/// <reference path="util.ts" />

class SnapshotManager {
  private table: HTMLTableSectionElement;
  private searchInput: HTMLInputElement;
  private sortBtn: HTMLButtonElement;
  private getState: () => AppState;
  private setState: (patch: Partial<AppState>) => void;
  private loading = false;
  private pendingDelete: (() => void) | null = null;
  private sizes: Record<string, string> = {};
  private currentVolume = '';

  constructor(
    table: HTMLTableSectionElement,
    searchInput: HTMLInputElement,
    sortBtn: HTMLButtonElement,
    getState: () => AppState,
    setState: (patch: Partial<AppState>) => void,
  ) {
    this.table = table;
    this.searchInput = searchInput;
    this.sortBtn = sortBtn;
    this.getState = getState;
    this.setState = setState;
    this.initDeleteModal();
  }

  private initDeleteModal(): void {
    const modal = document.getElementById('snapshotDeleteModal') as HTMLElement;
    const input = document.getElementById('snapshotDeleteInput') as HTMLInputElement;
    const confirmBtn = document.getElementById('snapshotDeleteConfirm') as HTMLButtonElement;
    const cancelBtn = document.getElementById('snapshotDeleteCancel') as HTMLButtonElement;

    const close = () => { modal.style.display = 'none'; this.pendingDelete = null; };

    input.addEventListener('input', () => {
      confirmBtn.disabled = input.value !== 'delete';
    });

    cancelBtn.addEventListener('click', close);
    modal.addEventListener('click', (e) => { if (e.target === modal) close(); });
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') close(); });

    confirmBtn.addEventListener('click', () => {
      if (input.value !== 'delete') return;
      close();
      const fn = this.pendingDelete;
      this.pendingDelete = null;
      if (fn) fn();
    });
  }

  showSkeleton(): void {
    this.loading = true;
    this.table.innerHTML = '';
    const colWidths = [
      ['110px', '70px', '60px', '28px', '28px', '90px', '100px'],
      ['90px', '70px', '60px', '28px', '28px', '90px', '100px'],
      ['130px', '70px', '60px', '28px', '28px', '90px', '100px'],
      ['100px', '70px', '60px', '28px', '28px', '90px', '100px'],
      ['120px', '70px', '60px', '28px', '28px', '90px', '100px'],
    ];
    for (const cols of colWidths) {
      const row = document.createElement('tr');
      row.className = 'skeleton-row';
      for (const w of cols) {
        const td = document.createElement('td');
        const bar = document.createElement('div');
        bar.className = 'skeleton-table-bar';
        bar.style.width = w;
        td.appendChild(bar);
        row.appendChild(td);
      }
      this.table.appendChild(row);
    }
    const parent = this.table.parentElement;
    if (parent && parent.offsetHeight > 0) {
      this.table.style.minHeight = `${parent.offsetHeight}px`;
    }
  }

  hideSkeleton(): void {
    this.loading = false;
    this.table.innerHTML = '';
  }

  async load(vol: string): Promise<void> {
    if (vol !== this.currentVolume) {
      this.sizes = {};
      this.currentVolume = vol;
    }
    this.showSkeleton();
    try {
      const resp = await fetch(`/api/snapshots?volume=${encodeURIComponent(vol)}`);
      if (!resp.ok) {
        let msg = 'Unable to load snapshots';
        try { const b = await resp.json(); if (b.error) msg = b.error; } catch {}
        throw new Error(msg);
      }
      const raw = (await resp.json() || []) as Snapshot[];
      this.setState({
        snapshots: raw.map(sn => ({ ...sn, tags: Array.isArray(sn.tags) ? sn.tags : [], paths: Array.isArray(sn.paths) ? sn.paths : [] })),
      });
      this.hideSkeleton();
      this.render();
      App.showStatus(`Loaded ${this.getState().snapshots.length} snapshots.`);
    } catch (err) {
      this.hideSkeleton();
      App.setBanner((err as Error).message, true);
      throw err;
    }
  }

  render(): void {
    if (this.loading) return;
    const st = this.getState();
    const fromSkeleton = !!this.table.querySelector('.skeleton-row');
    const oldH = fromSkeleton ? this.table.offsetHeight : 0;

    fadeIn(this.table, () => {
    this.table.innerHTML = '';

    const filtered = st.snapshots
      .filter(sn => {
        if (st.showHot === false && sn.tags.includes('hot')) return false;
        if (!st.query) return true;
        const text = [sn.id, sn.short_id, sn.hostname, sn.tags.join(' ')].join(' ').toLowerCase();
        return text.includes(st.query);
      })
      .sort((a, b) => {
        const aT = new Date(a.time).getTime();
        const bT = new Date(b.time).getTime();
        return st.sortNewestFirst ? bT - aT : aT - bT;
      });

    if (filtered.length === 0) {
      this.table.innerHTML = '<tr><td colspan="7">No backups match the current filter.</td></tr>';
      return;
    }

      // Warn if a volume has multiple restore-point snapshots
      const rpByVolume: Record<string, string[]> = {};
      filtered.forEach(sn => {
        if (sn.tags.includes('restore-point')) {
          sn.paths.forEach(p => {
            const vol = extractVolumeName(p);
            if (vol) {
              if (!rpByVolume[vol]) rpByVolume[vol] = [];
              rpByVolume[vol].push(sn.short_id);
            }
          });
        }
      });
      Object.entries(rpByVolume).forEach(([vol, ids]) => {
        if (ids.length > 1) {
          const wr = document.createElement('tr');
          wr.className = 'warning-row';
          const wc = document.createElement('td');
          wc.colSpan = 7;
          wc.innerHTML = `<span style="color:var(--yellow)">\u26a0</span> Volume <strong>${vol}</strong> has <strong>${ids.length}</strong> restore-point snapshots: ${ids.join(', ')}. Only one should exist.`;
          wr.appendChild(wc);
          this.table.appendChild(wr);
        }
      });

      filtered.forEach(sn => {
      const row = document.createElement('tr');
      const idCell = document.createElement('td');
      idCell.className = 'copy-id';
      idCell.title = 'Click to copy full snapshot ID';
      idCell.textContent = sn.short_id;
      idCell.addEventListener('click', () => {
        navigator.clipboard.writeText(sn.id);
        App.setBanner('Snapshot ID copied to clipboard');
        idCell.classList.add('copied');
        setTimeout(() => idCell.classList.remove('copied'), 1500);
      });
      row.appendChild(idCell);

      const hostCell = document.createElement('td');
      hostCell.textContent = sn.hostname || '-';
      hostCell.style.color = 'var(--muted)';
      hostCell.style.fontSize = '0.9rem';
      row.appendChild(hostCell);

      const tagsCell = document.createElement('td');
      tagsCell.style.color = 'var(--muted)';
      tagsCell.style.fontSize = '0.9rem';
      const visibleTags = sn.tags.filter(t => t === 'hot' || t === 'cold');
      tagsCell.textContent = visibleTags.length ? visibleTags.join(', ') : '—';
      row.appendChild(tagsCell);

      const rpCell = document.createElement('td');
      rpCell.style.textAlign = 'center';
      const rpIndicator = document.createElement('span');
      const isRP = sn.tags.includes('restore-point');
      const svgNs = 'http://www.w3.org/2000/svg';
      const rpSvg = document.createElementNS(svgNs, 'svg');
      rpSvg.setAttribute('width', '20');
      rpSvg.setAttribute('height', '20');
      rpSvg.setAttribute('viewBox', '0 0 20 20');
      rpSvg.style.display = 'block';
      rpSvg.style.margin = '0 auto';
      rpSvg.style.cursor = 'pointer';
      const circle = document.createElementNS(svgNs, 'circle');
      circle.setAttribute('cx', '10');
      circle.setAttribute('cy', '10');
      circle.setAttribute('r', '8');
      circle.setAttribute('fill', 'none');
      circle.setAttribute('stroke-width', '2');
      circle.setAttribute('stroke', isRP ? 'var(--accent)' : 'var(--border)');
      rpSvg.appendChild(circle);
      if (isRP) {
        const path = document.createElementNS(svgNs, 'path');
        path.setAttribute('d', 'M6 10 l3 3 l5 -5');
        path.setAttribute('stroke', 'var(--accent)');
        path.setAttribute('stroke-width', '2');
        path.setAttribute('fill', 'none');
        rpSvg.appendChild(path);
      }
      rpSvg.addEventListener('click', (e) => {
        e.stopPropagation();
        if (isRP) {
          this.removeTag(sn.id, 'restore-point', sn.volume);
        } else {
          this.addTag(sn.id, 'restore-point', sn.volume);
        }
      });
      rpCell.appendChild(rpSvg);
      row.appendChild(rpCell);

      const sizeCell = document.createElement('td');
      sizeCell.style.textAlign = 'center';
      sizeCell.style.fontVariantNumeric = 'tabular-nums';
      sizeCell.style.whiteSpace = 'nowrap';
      if (this.sizes[sn.id]) {
        sizeCell.textContent = this.sizes[sn.id];
      } else {
        const sizeBtn = document.createElement('span');
        sizeBtn.title = 'Compute size';
        sizeBtn.style.cssText = 'display:inline-flex;align-items:center;justify-content:center;width:22px;height:22px;border-radius:4px;cursor:pointer;opacity:0.5;';
        sizeBtn.style.color = 'var(--muted)';
        sizeBtn.addEventListener('mouseenter', () => sizeBtn.style.opacity = '1');
        sizeBtn.addEventListener('mouseleave', () => sizeBtn.style.opacity = '0.5');
        sizeBtn.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>';
        sizeBtn.addEventListener('click', async (e) => {
          e.stopPropagation();
          sizeBtn.innerHTML = '<span style="font-size:12px;">…</span>';
          sizeBtn.style.opacity = '0.6';
          try {
            const st = this.getState();
            const resp = await fetch('/api/snapshot/sizes', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ volume: st.selectedVolume, ids: [sn.id] }),
            });
            if (!resp.ok) throw new Error('Failed');
            const data = await resp.json() as Record<string, number>;
            if (data[sn.id] != null) {
              this.sizes[sn.id] = formatBytes(data[sn.id]);
            } else {
              this.sizes[sn.id] = 'err';
            }
            this.render();
          } catch {
            sizeBtn.style.opacity = '0.5';
          }
        });
        sizeCell.appendChild(sizeBtn);
      }
      row.appendChild(sizeCell);

      const timeCell = document.createElement('td');
      const d = new Date(sn.time);
      timeCell.innerHTML = `${d.toLocaleDateString()}<br><span style="font-size:0.85rem;color:var(--muted);">${d.toLocaleTimeString()}</span>`;
      row.appendChild(timeCell);

      const actionCell = document.createElement('td');
      actionCell.style.whiteSpace = 'nowrap';

      const viewBtn = document.createElement('button');
      viewBtn.className = 'button button-secondary button-xs';
      viewBtn.textContent = 'View';
      viewBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        const ev = new CustomEvent('open-snapshot-viewer', { detail: { snapshot: sn, snapshots: this.getState().snapshots } });
        document.dispatchEvent(ev);
      });
      actionCell.appendChild(viewBtn);

      const delBtn = document.createElement('button');
      delBtn.className = 'button button-secondary button-xs';
      delBtn.textContent = 'Delete';
      delBtn.style.marginLeft = '6px';
      delBtn.style.color = 'var(--red)';
      delBtn.style.borderColor = 'var(--red)';
      delBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const hostnameEl = document.getElementById('snapshotDeleteHostname') as HTMLSpanElement;
        const dateEl = document.getElementById('snapshotDeleteDate') as HTMLSpanElement;
        const tagsEl = document.getElementById('snapshotDeleteTags') as HTMLSpanElement;
        const input = document.getElementById('snapshotDeleteInput') as HTMLInputElement;
        const confirmBtn = document.getElementById('snapshotDeleteConfirm') as HTMLButtonElement;
        hostnameEl.textContent = sn.hostname || '-';
        dateEl.textContent = new Date(sn.time).toLocaleString();
        tagsEl.textContent = sn.tags.length ? sn.tags.join(', ') : 'none';
        input.value = '';
        confirmBtn.disabled = true;
        const modal = document.getElementById('snapshotDeleteModal') as HTMLElement;
        modal.style.display = '';
        this.pendingDelete = async () => {
          delBtn.disabled = true;
          delBtn.textContent = 'Deleting...';
          try {
            const resp = await fetch(`/api/snapshot/${encodeURIComponent(sn.id)}/delete?volume=${encodeURIComponent(sn.volume)}`, { method: 'DELETE' });
            if (!resp.ok) {
              const b = await resp.json();
              throw new Error(b.error || 'delete failed');
            }
            App.setBanner(`Snapshot ${sn.short_id} deleted`);
            await this.load(sn.volume);
          } catch (err) {
            App.setBanner((err as Error).message, true);
            delBtn.disabled = false;
            delBtn.textContent = 'Delete';
          }
        };
      });
      actionCell.appendChild(delBtn);

      row.appendChild(actionCell);
      this.table.appendChild(row);
      });

      if (fromSkeleton && oldH > 0) {
        const newH = this.table.offsetHeight;
        if (newH !== oldH) {
          this.table.style.minHeight = `${oldH}px`;
          this.table.style.height = `${oldH}px`;
          requestAnimationFrame(() => {
            this.table.style.transition = 'height 0.25s ease';
            this.table.style.height = `${newH}px`;
          });
          setTimeout(() => {
            this.table.style.height = '';
            this.table.style.minHeight = '';
            this.table.style.transition = '';
          }, 260);
        } else {
          this.table.style.minHeight = '';
        }
      } else {
        this.table.style.minHeight = '';
      }
    });
  }

  private async addTag(snapshotID: string, tag: string, volume: string): Promise<void> {
    App.setBanner(`Adding tag "${tag}"...`);
    try {
      const resp = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`, { method: 'POST' });
      if (!resp.ok) throw new Error('Failed to add tag');
      await this.load(volume);
    } catch (err) {
      App.setBanner((err as Error).message, true);
    }
  }

  private async removeTag(snapshotID: string, tag: string, volume: string): Promise<void> {
    App.setBanner(`Removing tag "${tag}"...`);
    try {
      const resp = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`, { method: 'DELETE' });
      if (!resp.ok) throw new Error('Failed to remove tag');
      await this.load(volume);
    } catch (err) {
      App.setBanner((err as Error).message, true);
    }
  }
}
