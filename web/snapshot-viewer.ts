/// <reference path="types.ts" />
/// <reference path="util.ts" />

interface FileNode {
  name: string;
  type: string;
  path: string;
  full_path?: string;
  size?: number;
}

interface DiffChange {
  type: string;
  paths: string[];
}

interface DiffResult {
  change_sets: DiffChange[];
}

class SnapshotViewer {
  private panel: HTMLElement;
  private tree: HTMLElement;
  private detail: HTMLElement;
  private loading: HTMLElement;
  private snapIdEl: HTMLElement;
  private closeBtn: HTMLElement;
  private compHeader: HTMLElement;
  private compSelect: HTMLSelectElement;
  private compBtn: HTMLElement;
  private currentSnapshot: Snapshot | null = null;
  private allSnapshots: Snapshot[] = [];

  constructor() {
    this.panel = document.getElementById('snapshotViewer')!;
    this.tree = document.getElementById('viewerTree')!;
    this.detail = document.getElementById('viewerDetail')!;
    this.loading = document.getElementById('viewerLoading')!;
    this.snapIdEl = document.getElementById('viewerSnapId')!;
    this.closeBtn = document.getElementById('viewerClose')!;
    this.compHeader = document.getElementById('viewerCompareHeader')!;
    this.compSelect = document.getElementById('viewerCompareSelect')! as HTMLSelectElement;
    this.compBtn = document.getElementById('viewerCompareBtn')!;
    this.listen();
  }

  private listen(): void {
    document.addEventListener('open-snapshot-viewer', (e: Event) => {
      const ev = e as CustomEvent;
      this.open(ev.detail.snapshot).catch(err => this.showError(err.message));
    });
    this.closeBtn.addEventListener('click', () => this.close());
    this.compBtn.addEventListener('click', () => this.doDiff());
  }

  private showError(msg: string): void {
    this.loading.style.display = 'none';
    this.detail.innerHTML = `<span style="color:var(--red)">Error: ${this.escapeHtml(msg)}</span>`;
  }

  async open(snapshot: Snapshot): Promise<void> {
    this.currentSnapshot = snapshot;
    this.snapIdEl.textContent = snapshot.short_id;
    this.panel.style.display = 'block';
    this.loading.style.display = 'block';
    this.tree.innerHTML = '';
    this.detail.innerHTML = '';
    this.compHeader.style.display = 'none';

    let nodes: FileNode[];
    try {
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(snapshot.id)}/ls`);
      if (!resp.ok) {
        const body = await resp.text();
        throw new Error(body || 'Failed to list snapshot');
      }
      nodes = await resp.json();
    } catch (err) {
      this.showError((err as Error).message);
      return;
    }

    if (!nodes || nodes.length === 0) {
      this.loading.style.display = 'none';
      this.detail.innerHTML = '<span style="color:var(--muted)">Snapshot is empty.</span>';
      return;
    }

    this.loading.style.display = 'none';
    this.renderTree(nodes);
    this.populateCompareSelect(snapshot).catch(() => {});
  }

  private renderTree(nodes: FileNode[]): void {
    const root: any = { name: '/', type: 'dir', children: {} };
    for (const n of nodes) {
      if (!n.path || n.path === '/') continue;
      const parts = n.path.replace(/^\//, '').split('/');
      let cur = root;
      for (let i = 0; i < parts.length; i++) {
        const p = parts[i];
        if (p === '') continue;
        if (i === parts.length - 1) {
          (n as any).children = undefined;
          cur.children[p] = n;
        } else {
          if (!cur.children[p]) {
            cur.children[p] = { name: p, type: 'dir', children: {} };
          } else if (!cur.children[p].children) {
            cur.children[p].children = {};
          }
          cur = cur.children[p];
        }
      }
    }
    this.renderNode(root, this.tree, 0);
  }

  private renderNode(node: any, parent: HTMLElement, depth: number): void {
    if (node.name === '/' && depth === 0) {
      const sorted = Object.values(node.children).sort((a: any, b: any) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      for (const child of sorted as any[]) {
        this.renderNode(child, parent, depth + 1);
      }
      return;
    }

    if (node.type === 'dir' || node.children) {
      const details = document.createElement('details');
      if (depth <= 1) details.open = true;
      const summary = document.createElement('summary');
      summary.textContent = ' ' + node.name;
      summary.style.cursor = 'pointer';
      summary.style.padding = '2px 4px';
      summary.style.borderRadius = '4px';
      summary.style.color = 'var(--text)';
      summary.style.fontSize = '0.9rem';
      const icon = document.createElement('span');
      icon.textContent = '\u{1F4C1}';
      summary.prepend(icon);
      details.appendChild(summary);
      const content = document.createElement('div');
      content.style.marginLeft = '18px';
      const sorted = (node.children ? Object.values(node.children) : []).sort((a: any, b: any) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      for (const child of sorted as any[]) {
        this.renderNode(child, content, depth + 1);
      }
      details.appendChild(content);
      parent.appendChild(details);
    } else {
      const item = document.createElement('div');
      item.style.cursor = 'pointer';
      item.style.padding = '2px 4px';
      item.style.borderRadius = '4px';
      item.style.fontSize = '0.9rem';
      item.style.display = 'flex';
      item.style.alignItems = 'center';
      item.style.gap = '4px';
      const icon = document.createElement('span');
      icon.textContent = '\u{1F4C4}';
      const nameSpan = document.createElement('span');
      nameSpan.textContent = node.name;
      if (node.size != null) {
        const sizeSpan = document.createElement('span');
        sizeSpan.style.color = 'var(--muted)';
        sizeSpan.style.fontSize = '0.75rem';
        sizeSpan.style.marginLeft = 'auto';
        sizeSpan.textContent = formatBytes(node.size);
        item.appendChild(sizeSpan);
      }
      item.prepend(icon);
      item.appendChild(nameSpan);
      item.addEventListener('click', () => this.viewFile(node.full_path || node.path));
      item.addEventListener('mouseenter', () => item.style.background = 'rgba(255,255,255,0.06)');
      item.addEventListener('mouseleave', () => item.style.background = '');
      parent.appendChild(item);
    }
  }

  async viewFile(path: string): Promise<void> {
    if (!this.currentSnapshot || !path) {
      this.showError('missing path');
      return;
    }
    this.detail.innerHTML = '<span style="color:var(--muted)">Loading file...</span>';
    try {
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(this.currentSnapshot.id)}/dump?path=${encodeURIComponent(path)}`);
      if (!resp.ok) {
        const body = await resp.text();
        this.showError(body || 'dump failed');
        return;
      }
      const text = await resp.text();
      const ext = path.split('.').pop()!.toLowerCase();
      if (['json','yaml','yml','xml','md','txt','sh','py','js','ts','go','conf','cfg','ini','log','env'].includes(ext) || text.length < 100000) {
        this.detail.textContent = text;
      } else {
        this.detail.textContent = `Binary file or too large (${formatBytes(text.length)}). Download via restic directly.`;
      }
    } catch (err) {
      this.showError((err as Error).message);
    }
  }

  async viewFileFromId(path: string, snapshotId: string): Promise<void> {
    if (!path || !snapshotId) {
      this.showError('missing path or snapshot id');
      return;
    }
    this.detail.innerHTML = '<span style="color:var(--muted)">Loading file...</span>';
    try {
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(snapshotId)}/dump?path=${encodeURIComponent(path)}`);
      if (!resp.ok) {
        const body = await resp.text();
        this.showError(body || 'dump failed');
        return;
      }
      const text = await resp.text();
      const ext = path.split('.').pop()!.toLowerCase();
      if (['json','yaml','yml','xml','md','txt','sh','py','js','ts','go','conf','cfg','ini','log','env'].includes(ext) || text.length < 100000) {
        this.detail.textContent = text;
      } else {
        this.detail.textContent = `Binary file or too large (${formatBytes(text.length)}). Download via restic directly.`;
      }
    } catch (err) {
      this.showError((err as Error).message);
    }
  }
  private matchVolume(path: string, vol: string): boolean {
    const marker = '/volumes/';
    const idx = path.indexOf(marker);
    if (idx >= 0) {
      const subpath = path.slice(idx + marker.length).replace(/^\//, '');
      return subpath.split('/')[0] === vol;
    }
    const parts = path.split('/').filter(Boolean);
    const last = parts[parts.length - 1] || '';
    for (const suffix of ['-cold-snap', '-pre-restore']) {
      if (last.endsWith(suffix)) {
        return last.slice(0, -suffix.length) === vol;
      }
    }
    return last === vol;
  }

  private extractVol(path: string): string {
    const marker = '/volumes/';
    const idx = path.indexOf(marker);
    if (idx >= 0) {
      return path.slice(idx + marker.length).replace(/^\//, '').split('/')[0] || '';
    }
    const parts = path.split('/').filter(Boolean);
    const last = parts[parts.length - 1] || '';
    for (const suffix of ['-cold-snap', '-pre-restore']) {
      if (last.endsWith(suffix)) {
        return last.slice(0, -suffix.length);
      }
    }
    return last;
  }

  async populateCompareSelect(snapshot: Snapshot): Promise<void> {
    const vol = this.extractVol((snapshot.paths || [])[0] || '');
    if (!vol) return;

    const resp = await fetch('/api/snapshots');
    if (!resp.ok) return;
    this.allSnapshots = await resp.json() as Snapshot[];

    this.compSelect.innerHTML = '';
    let count = 0;
    for (const sn of this.allSnapshots) {
      if (sn.id === snapshot.id || sn.short_id === snapshot.short_id) continue;
      if (!sn.paths || !sn.paths.some(p => this.matchVolume(p, vol))) continue;
      const opt = document.createElement('option');
      opt.value = sn.id;
      opt.textContent = `${sn.short_id} (${new Date(sn.time).toLocaleString()})`;
      this.compSelect.appendChild(opt);
      count++;
    }
    if (count > 0) {
      this.compHeader.style.display = 'block';
    }
  }

  private diffOtherId = '';

  async doDiff(): Promise<void> {
    if (!this.currentSnapshot) return;
    const otherId = this.compSelect.value;
    if (!otherId) {
      this.detail.innerHTML = '<span style="color:var(--muted)">Select a snapshot to compare.</span>';
      return;
    }
    this.diffOtherId = otherId;
    this.detail.innerHTML = '<span style="color:var(--muted)">Loading diff...</span>';
    try {
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(this.currentSnapshot.id)}/diff/${encodeURIComponent(otherId)}`);
      if (!resp.ok) throw new Error('Diff failed');
      const result = await resp.json() as DiffResult;
      this.renderDiff(result, otherId);
    } catch (err) {
      this.showError((err as Error).message);
    }
  }

  private renderDiff(result: DiffResult, otherId: string): void {
    const html: string[] = [];
    for (const cs of (result.change_sets || [])) {
      const color = cs.type === 'added' ? 'var(--green)' : cs.type === 'removed' ? 'var(--red)' : 'var(--yellow)';
      const label = cs.type.toUpperCase();
      html.push(`<div style="margin-top:8px;"><span style="background:${color};color:#000;padding:1px 8px;border-radius:4px;font-weight:700;font-size:0.75rem;">${label}</span> (${cs.paths.length} items)</div>`);
      for (const p of (cs.paths || []).slice(0, 50)) {
        if (cs.type === 'modified') {
          html.push(`<div style="padding:1px 0 1px 16px;font-size:0.85rem;color:${color};display:flex;align-items:center;gap:6px;"><span>${this.escapeHtml(p)}</span> <a href="#" style="color:var(--accent);font-size:0.75rem;text-decoration:none;" data-path="${this.escapeHtml(p)}" data-other="${this.escapeHtml(otherId)}" class="file-diff-link">[diff]</a></div>`);
        } else if (cs.type === 'added' || cs.type === 'removed') {
          const viewId = cs.type === 'added' ? otherId : this.currentSnapshot!.id;
          const viewLabel = cs.type === 'added' ? 'new' : 'old';
          html.push(`<div style="padding:1px 0 1px 16px;font-size:0.85rem;color:${color};display:flex;align-items:center;gap:6px;"><span>${this.escapeHtml(p)}</span> <a href="#" style="color:var(--accent);font-size:0.75rem;text-decoration:none;" data-path="${this.escapeHtml(p)}" data-view-id="${this.escapeHtml(viewId)}" class="file-view-link">[view ${viewLabel}]</a></div>`);
        } else {
          html.push(`<div style="padding:1px 0 1px 16px;font-size:0.85rem;color:${color};">${this.escapeHtml(p)}</div>`);
        }
      }
      if (cs.paths.length > 50) {
        html.push(`<div style="padding:1px 0 1px 16px;font-size:0.85rem;color:var(--muted);">... and ${cs.paths.length - 50} more</div>`);
      }
    }
    if (html.length === 0) {
      html.push('<div style="color:var(--muted);padding:20px;text-align:center;">Snapshots are identical.</div>');
    }
    this.detail.innerHTML = html.join('\n');
    this.detail.querySelectorAll('.file-diff-link').forEach(el => {
      el.addEventListener('click', (e: Event) => {
        e.preventDefault();
        const path = el.getAttribute('data-path');
        const other = el.getAttribute('data-other');
        if (path && other) this.showFileDiff(path, other);
      });
    });
    this.detail.querySelectorAll('.file-view-link').forEach(el => {
      el.addEventListener('click', (e: Event) => {
        e.preventDefault();
        const path = el.getAttribute('data-path');
        const viewId = el.getAttribute('data-view-id');
        if (path && viewId) this.viewFileFromId(path, viewId);
      });
    });
  }

  async showFileDiff(path: string, otherId: string): Promise<void> {
    if (!this.currentSnapshot) return;
    this.detail.innerHTML = `<span style="color:var(--muted)">Loading diff for ${this.escapeHtml(path)}...</span>`;
    try {
      const [respA, respB] = await Promise.all([
        fetch(`/api/snapshot-view/${encodeURIComponent(this.currentSnapshot.id)}/dump?path=${encodeURIComponent(path)}`),
        fetch(`/api/snapshot-view/${encodeURIComponent(otherId)}/dump?path=${encodeURIComponent(path)}`),
      ]);
      if (!respA.ok || !respB.ok) {
        const errA = respA.ok ? '' : ' ' + (await respA.text());
        const errB = respB.ok ? '' : ' ' + (await respB.text());
        throw new Error('dump failed' + errA + errB);
      }
      const textA = await respA.text();
      const textB = await respB.text();

      if (textA.length > 500000 || textB.length > 500000) {
        this.detail.textContent = 'File too large to diff inline (max 500 KB per version).';
        return;
      }

      const linesA = textA.split('\n');
      const linesB = textB.split('\n');
      const diff = this.computeUnifiedDiff(linesA, linesB);
      this.renderUnifiedDiff(diff, path);
    } catch (err) {
      this.showError((err as Error).message);
    }
  }

  private computeUnifiedDiff(linesA: string[], linesB: string[]): { t: string; line: string }[] {
    const lcs = this.lcs(linesA, linesB);
    const result: { t: string; line: string }[] = [];
    let a = 0, b = 0, l = 0;
    while (a < linesA.length || b < linesB.length) {
      if (l < lcs.length && a < linesA.length && b < linesB.length && linesA[a] === linesB[b] && linesA[a] === lcs[l]) {
        result.push({ t: 'ctx', line: linesA[a] });
        a++; b++; l++;
      } else if (l < lcs.length && a < linesA.length && linesA[a] !== lcs[l]) {
        result.push({ t: 'del', line: linesA[a] });
        a++;
      } else if (l < lcs.length && b < linesB.length && linesB[b] !== lcs[l]) {
        result.push({ t: 'add', line: linesB[b] });
        b++;
      } else if (a < linesA.length) {
        result.push({ t: 'del', line: linesA[a] });
        a++;
      } else if (b < linesB.length) {
        result.push({ t: 'add', line: linesB[b] });
        b++;
      } else break;
    }
    return result;
  }

  private lcs(a: string[], b: string[]): string[] {
    const m = a.length, n = b.length;
    if (m === 0 || n === 0) return [];
    let prev = new Array(n + 1).fill(0);
    const dir: number[][] = Array.from({ length: m }, () => new Array(n).fill(0));
    for (let i = 0; i < m; i++) {
      const cur = new Array(n + 1).fill(0);
      for (let j = 0; j < n; j++) {
        if (a[i] === b[j]) {
          cur[j + 1] = prev[j] + 1;
          dir[i][j] = 1;
        } else {
          if (prev[j + 1] >= cur[j]) {
            cur[j + 1] = prev[j + 1];
            dir[i][j] = 2;
          } else {
            cur[j + 1] = cur[j];
            dir[i][j] = 3;
          }
        }
      }
      prev = cur;
    }
    const result: string[] = [];
    let i = m - 1, j = n - 1;
    while (i >= 0 && j >= 0) {
      if (dir[i][j] === 1) {
        result.unshift(a[i]);
        i--; j--;
      } else if (dir[i][j] === 2) {
        i--;
      } else {
        j--;
      }
    }
    return result;
  }

  private renderUnifiedDiff(diff: { t: string; line: string }[], path: string): void {
    const html: string[] = [`<div style="margin-bottom:8px;font-weight:700;">Diff: ${this.escapeHtml(path)}</div>`];
    let ctxCount = 0;
    const flushCtx = () => {
      if (ctxCount > 3) {
        html.push(`<div style="color:var(--muted);font-size:0.8rem;padding:2px 0;">... ${ctxCount - 3} common lines hidden ...</div>`);
      }
      ctxCount = 0;
    };

    for (const entry of diff) {
      if (entry.t === 'ctx') {
        ctxCount++;
        if (ctxCount <= 3) {
          const line = entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line;
          html.push(`<div style="padding:1px 4px;font-size:0.85rem;background:rgba(255,255,255,0.02);border-radius:2px;"> ${this.escapeHtml(line)}</div>`);
        }
      } else {
        flushCtx();
        const bg = entry.t === 'add' ? 'rgba(52,211,153,0.1)' : 'rgba(248,113,113,0.1)';
        const prefix = entry.t === 'add' ? '+' : '-';
        const line = entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line;
        html.push(`<div style="padding:1px 4px;font-size:0.85rem;background:${bg};border-radius:2px;white-space:pre-wrap;">${prefix} ${this.escapeHtml(line)}</div>`);
      }
    }
    flushCtx();
    this.detail.innerHTML = html.join('\n');
  }

  private escapeHtml(s: string): string {
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }

  close(): void {
    this.panel.style.display = 'none';
    this.currentSnapshot = null;
    this.tree.innerHTML = '';
    this.detail.innerHTML = '';
    this.compHeader.style.display = 'none';
  }
}

new SnapshotViewer();
