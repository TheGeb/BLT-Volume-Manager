/// <reference path="types.ts" />
/// <reference path="util.ts" />

class SnapshotManager {
  private table: HTMLTableSectionElement;
  private searchInput: HTMLInputElement;
  private sortBtn: HTMLButtonElement;
  private getState: () => AppState;
  private setState: (patch: Partial<AppState>) => void;

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
    this.table.innerHTML = '';
    for (let i = 0; i < 5; i++) {
      const row = document.createElement('tr');
      const td = document.createElement('td');
      td.colSpan = 5;
      td.innerHTML = '<div class="skeleton skeleton-row"></div>';
      row.appendChild(td);
      this.table.appendChild(row);
    }
  }

  async load(): Promise<void> {
    const hasData = !!this.table.querySelector('td');
    if (!hasData) this.showSkeleton();
    try {
      const resp = await fetch('/api/snapshots');
      if (!resp.ok) {
        let msg = 'Unable to load snapshots';
        try { const b = await resp.json(); if (b.error) msg = b.error; } catch {}
        throw new Error(msg);
      }
      const raw = await resp.json() as Snapshot[];
      this.setState({
        snapshots: raw.map(sn => ({ ...sn, tags: Array.isArray(sn.tags) ? sn.tags : [], paths: Array.isArray(sn.paths) ? sn.paths : [] })),
      });
      this.render();
      App.showStatus(`Loaded ${this.getState().snapshots.length} snapshots.`);
    } catch (err) {
      App.showStatus((err as Error).message, true);
      throw err;
    }
  }

  render(): void {
    const st = this.getState();
    const fromSkeleton = !!this.table.querySelector('.skeleton');
    const oldH = fromSkeleton ? this.table.offsetHeight : 0;

    fadeIn(this.table, () => {
    this.table.innerHTML = '';

    const filtered = st.snapshots
      .filter(sn => {
        if (st.selectedVolume && !sn.paths.some(p => extractVolumeName(p) === st.selectedVolume)) return false;
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
      const visibleTags = sn.tags.filter(t => t === 'hot' || t === 'cold');
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
        (cb.checked ? this.addTag(sn.id, 'excluded') : this.removeTag(sn.id, 'excluded'));
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
          const resp = await fetch(`/api/snapshot/${encodeURIComponent(sn.id)}/restore?path=${encodeURIComponent(target)}`, { method: 'POST' });
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

  private async addTag(snapshotID: string, tag: string): Promise<void> {
    App.showStatus(`Adding tag "${tag}"...`);
    try {
      const resp = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}`, { method: 'POST' });
      if (!resp.ok) throw new Error('Failed to add tag');
      await this.load();
    } catch (err) {
      App.showStatus((err as Error).message, true);
    }
  }

  private async removeTag(snapshotID: string, tag: string): Promise<void> {
    App.showStatus(`Removing tag "${tag}"...`);
    try {
      const resp = await fetch(`/api/snapshot/${encodeURIComponent(snapshotID)}/tag?tag=${encodeURIComponent(tag)}`, { method: 'DELETE' });
      if (!resp.ok) throw new Error('Failed to remove tag');
      await this.load();
    } catch (err) {
      App.showStatus((err as Error).message, true);
    }
  }
}
