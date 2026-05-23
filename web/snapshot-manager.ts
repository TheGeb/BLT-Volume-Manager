/// <reference path="types.ts" />
/// <reference path="util.ts" />

class SnapshotManager {
  private table: HTMLTableSectionElement;
  private searchInput: HTMLInputElement;
  private sortBtn: HTMLButtonElement;
  private getState: () => AppState;
  private setState: (patch: Partial<AppState>) => void;
  private loading = false;

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
  }

  showSkeleton(): void {
    this.loading = true;
    this.table.innerHTML = '';
    const colWidths = [
      [['120px', '80px'], ['180px', '100px'], ['80px', '60px'], ['140px', '90px'], ['100px']],
      [['100px', '70px'], ['220px', '140px'], ['100px', '70px'], ['130px', '80px'], ['100px']],
      [['140px', '90px'], ['160px', '110px'], ['80px', '50px'], ['150px', '100px'], ['100px']],
      [['110px', '75px'], ['200px', '130px'], ['120px', '80px'], ['135px', '85px'], ['100px']],
      [['130px', '85px'], ['170px', '120px'], ['80px', '60px'], ['145px', '95px'], ['100px']],
      [['90px', '65px'], ['240px', '150px'], ['100px', '70px'], ['125px', '75px'], ['100px']],
      [['150px', '100px'], ['190px', '130px'], ['80px', '55px'], ['155px', '105px'], ['100px']],
    ];
    for (const cols of colWidths) {
      const row = document.createElement('tr');
      row.className = 'skeleton-row';
      for (const widths of cols) {
        const td = document.createElement('td');
        for (const w of widths) {
          const bar = document.createElement('div');
          bar.className = 'skeleton-table-bar';
          bar.style.width = w;
          td.appendChild(bar);
        }
        row.appendChild(td);
      }
      this.table.appendChild(row);
    }
  }

  hideSkeleton(): void {
    this.loading = false;
    this.table.innerHTML = '';
  }

  async load(vol: string): Promise<void> {
    this.showSkeleton();
    try {
      const resp = await fetch(`/api/snapshots?volume=${encodeURIComponent(vol)}`);
      if (!resp.ok) {
        let msg = 'Unable to load snapshots';
        try { const b = await resp.json(); if (b.error) msg = b.error; } catch {}
        throw new Error(msg);
      }
      const raw = await resp.json() as Snapshot[];
      this.setState({
        snapshots: raw.map(sn => ({ ...sn, tags: Array.isArray(sn.tags) ? sn.tags : [], paths: Array.isArray(sn.paths) ? sn.paths : [] })),
      });
      this.hideSkeleton();
      this.render();
      App.showStatus(`Loaded ${this.getState().snapshots.length} snapshots.`);
    } catch (err) {
      this.hideSkeleton();
      App.showStatus((err as Error).message, true);
      throw err;
    }
  }

  render(): void {
    if (this.loading) return;
    const st = this.getState();
    const fromSkeleton = !!this.table.querySelector('.skeleton-table-row');
    const oldH = fromSkeleton ? this.table.offsetHeight : 0;

    fadeIn(this.table, () => {
    this.table.innerHTML = '';

    const filtered = st.snapshots
      .filter(sn => {
        if (!st.query) return true;
        const text = [sn.id, sn.short_id, sn.paths.join(' '), sn.tags.join(' ')].join(' ').toLowerCase();
        return text.includes(st.query);
      })
      .sort((a, b) => {
        const aT = new Date(a.time).getTime();
        const bT = new Date(b.time).getTime();
        return st.sortNewestFirst ? bT - aT : aT - bT;
      });

    if (filtered.length === 0) {
      this.table.innerHTML = '<tr><td colspan="5">No backups match the current filter.</td></tr>';
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
          wc.colSpan = 5;
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
        App.showStatus('Snapshot ID copied to clipboard');
        idCell.classList.add('copied');
        setTimeout(() => idCell.classList.remove('copied'), 1500);
      });
      row.appendChild(idCell);

      const pathsCell = document.createElement('td');
      pathsCell.textContent = sn.paths.join(', ');
      row.appendChild(pathsCell);

      const tagsCell = document.createElement('td');
      const tagList = document.createElement('div');
      tagList.className = 'tag-list';
      const isRestorePoint = sn.tags.includes('restore-point');
      const visibleTags = sn.tags.filter(t => t === 'hot' || t === 'cold' || t === 'restore-point');
      const isExcluded = sn.tags.includes('excluded');
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
        (cb.checked ? this.addTag(sn.id, 'excluded', sn.volume) : this.removeTag(sn.id, 'excluded', sn.volume));
      });
      label.appendChild(cb);
      label.appendChild(document.createTextNode(' excluded'));
      tagList.appendChild(label);
      tagsCell.appendChild(tagList);
      row.appendChild(tagsCell);

      const timeCell = document.createElement('td');
      timeCell.textContent = new Date(sn.time).toLocaleString();
      row.appendChild(timeCell);

      const actionCell = document.createElement('td');

      const viewBtn = document.createElement('button');
      viewBtn.className = 'button button-secondary button-xs';
      viewBtn.textContent = 'View';
      viewBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        const ev = new CustomEvent('open-snapshot-viewer', { detail: { snapshot: sn } });
        document.dispatchEvent(ev);
      });
      actionCell.appendChild(viewBtn);

      const restoreBtn = document.createElement('button');
      restoreBtn.className = 'button button-secondary button-xs';
      restoreBtn.textContent = 'Restore';
      restoreBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        const target = prompt('Target path for restore:', '/tmp/restore/' + sn.short_id);
        if (!target) return;
        restoreBtn.disabled = true;
        restoreBtn.textContent = 'Restoring...';
        try {
          const resp = await fetch(`/api/snapshot/${encodeURIComponent(sn.id)}/restore?path=${encodeURIComponent(target)}&volume=${encodeURIComponent(sn.volume)}`, { method: 'POST' });
          if (!resp.ok) {
            const b = await resp.json();
            throw new Error(b.error || 'restore failed');
          }
          App.showStatus(`Restore of ${sn.short_id} started – see server logs for results.`);
        } catch (err) {
          App.showStatus((err as Error).message, true);
        } finally {
          restoreBtn.disabled = false;
          restoreBtn.textContent = 'Restore';
        }
      });
      actionCell.appendChild(restoreBtn);

      const rpBtn = document.createElement('button');
      rpBtn.className = 'button button-secondary button-xs';
      rpBtn.textContent = isRestorePoint ? '\u229b Restore Point' : '\u2295 Set Restore Point';
      rpBtn.style.marginLeft = '6px';
      if (isRestorePoint) {
        rpBtn.style.borderColor = 'var(--accent)';
        rpBtn.style.color = 'var(--accent)';
      }
      rpBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        rpBtn.disabled = true;
        try {
          await this.addTag(sn.id, 'restore-point', sn.volume);
          App.showStatus(`Snapshot ${sn.short_id} set as restore point.`);
        } catch (err) {
          App.showStatus((err as Error).message, true);
        } finally {
          rpBtn.disabled = false;
        }
      });
      actionCell.appendChild(rpBtn);
      row.appendChild(actionCell);
      this.table.appendChild(row);
    });

    if (fromSkeleton && oldH > 0) {
      const newH = this.table.offsetHeight;
      if (newH !== oldH) {
        this.table.style.height = `${oldH}px`;
        requestAnimationFrame(() => {
          this.table.style.transition = 'height 0.25s ease';
          this.table.style.height = `${newH}px`;
        });
        setTimeout(() => {
          this.table.style.height = '';
          this.table.style.transition = '';
        }, 260);
      }
    }
    });
  }

  private async addTag(snapshotID: string, tag: string, volume: string): Promise<void> {
    App.showStatus(`Adding tag "${tag}"...`);
    try {
      const resp = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`, { method: 'POST' });
      if (!resp.ok) throw new Error('Failed to add tag');
      await this.load(volume);
    } catch (err) {
      App.showStatus((err as Error).message, true);
    }
  }

  private async removeTag(snapshotID: string, tag: string, volume: string): Promise<void> {
    App.showStatus(`Removing tag "${tag}"...`);
    try {
      const resp = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}&volume=${encodeURIComponent(volume)}`, { method: 'DELETE' });
      if (!resp.ok) throw new Error('Failed to remove tag');
      await this.load(volume);
    } catch (err) {
      App.showStatus((err as Error).message, true);
    }
  }
}
