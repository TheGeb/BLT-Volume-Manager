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
  private treePanel: HTMLElement;
  private detail: HTMLElement;
  private loading: HTMLElement;
  private snapIdEl: HTMLElement;
  private closeBtn: HTMLElement;
  private compHeader: HTMLElement;
  private compSelect: HTMLSelectElement;
  private compBtn: HTMLElement;
  private clearDiffBtn: HTMLElement;
  private compareSkeleton: HTMLElement;
  private expandAllBtn: HTMLElement;
  private collapseAllBtn: HTMLElement;
  private currentSnapshot: Snapshot | null = null;
  private allSnapshots: Snapshot[] = [];
  private lastNodes: FileNode[] = [];
  private currentDiffResult: DiffResult | null = null;

  constructor() {
    this.panel = document.getElementById('snapshotViewer')!;
    this.treePanel = document.getElementById('viewerTreePanel')!;
    this.tree = document.getElementById('viewerTree')!;
    this.detail = document.getElementById('viewerDetail')!;
    this.loading = document.getElementById('viewerLoading')!;
    this.snapIdEl = document.getElementById('viewerSnapId')!;
    this.closeBtn = document.getElementById('viewerClose')!;
    this.compHeader = document.getElementById('viewerCompareHeader')!;
    this.compSelect = document.getElementById('viewerCompareSelect')! as HTMLSelectElement;
    this.compBtn = document.getElementById('viewerCompareBtn')!;
    this.clearDiffBtn = document.getElementById('viewerClearDiffBtn')!;
    this.compareSkeleton = document.getElementById('viewerCompareSkeleton')!;
    this.expandAllBtn = document.getElementById('viewerExpandAll')!;
    this.collapseAllBtn = document.getElementById('viewerCollapseAll')!;
    this.listen();
    this.initDrag();
  }

  private listen(): void {
    document.addEventListener('open-snapshot-viewer', (e: Event) => {
      const ev = e as CustomEvent;
      this.allSnapshots = ev.detail.snapshots || [];
      this.open(ev.detail.snapshot).catch(err => this.showError(err.message));
    });
    this.closeBtn.addEventListener('click', () => this.close());
    this.compBtn.addEventListener('click', () => this.doDiff());
    this.clearDiffBtn.addEventListener('click', () => this.clearDiff());
    this.expandAllBtn.addEventListener('click', () => this.toggleAll(true));
    this.collapseAllBtn.addEventListener('click', () => this.toggleAll(false));
    document.addEventListener('close-snapshot-viewer', () => this.close());
  }

  private toggleAll(open: boolean): void {
    const details = this.tree.querySelectorAll('details');
    for (const d of details) {
      d.open = open;
    }
  }

  private showError(msg: string): void {
    this.loading.style.display = 'none';
    this.detail.innerHTML = `<span style="color:var(--red)">Error: ${this.escapeHtml(msg)}</span>`;
  }

  async open(snapshot: Snapshot): Promise<void> {
    this.currentSnapshot = snapshot;
    this.snapIdEl.textContent = `${snapshot.short_id} (${snapshot.hostname || '?'})`;
    this.panel.style.display = 'block';
    this.loading.style.display = 'none';
    this.tree.innerHTML = '';
    this.renderSkeleton();
    this.detail.innerHTML = '';
    this.showCompareSkeleton();
    this.compHeader.style.display = 'none';

    let nodes: FileNode[];
    try {
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(snapshot.id)}/ls?volume=${encodeURIComponent(snapshot.volume)}`);
      if (!resp.ok) {
        const body = await resp.text();
        throw new Error(body || 'Failed to list snapshot');
      }
      nodes = await resp.json();
    } catch (err) {
      this.tree.innerHTML = '';
      this.hideCompareSkeleton();
      this.showError((err as Error).message);
      return;
    }

    if (!nodes || nodes.length === 0) {
      this.tree.innerHTML = '';
      this.hideCompareSkeleton();
      this.detail.innerHTML = '<span style="color:var(--muted)">Snapshot is empty.</span>';
      return;
    }

    this.lastNodes = nodes;
    this.currentDiffResult = null;
    this.diffOtherId = '';
    this.clearDiffBtn.style.display = 'none';
    this.renderTree(nodes, null);
    this.populateCompareSelect(snapshot).catch(err => console.warn('populateCompareSelect failed', err));
  }

  private showCompareSkeleton(): void {
    this.compareSkeleton.style.display = 'flex';
    this.compHeader.style.display = 'none';
  }

  private hideCompareSkeleton(): void {
    this.compareSkeleton.style.display = 'none';
  }

  private renderSkeleton(): void {
    this.detail.innerHTML = '';
    const lines = [
      '80%', '60%', '90%', '55%', '70%',
      '85%',
    ];
    for (const w of lines) {
      const bar = document.createElement('div');
      bar.className = 'skeleton-table-bar';
      bar.style.cssText = `width:${w};height:14px;border-radius:6px;margin-bottom:10px;`;
      this.detail.appendChild(bar);
    }

    this.tree.innerHTML = '';
    const btnRow = document.createElement('div');
    btnRow.style.cssText = 'display:flex;gap:4px;margin-bottom:6px;';
    for (let i = 0; i < 2; i++) {
      const btn = document.createElement('div');
      btn.className = 'skeleton';
      btn.style.cssText = 'flex:1;height:28px;border-radius:8px;';
      btnRow.appendChild(btn);
    }
    this.tree.appendChild(btnRow);

    const items = [
      { depth: 0 }, { depth: 1 }, { depth: 1 }, { depth: 2 },
      { depth: 0 }, { depth: 1 },
    ];
    for (const item of items) {
      const row = document.createElement('div');
      row.className = 'skeleton-tree-item';
      row.style.paddingLeft = `${item.depth * 18 + 4}px`;
      const icon = document.createElement('span');
      icon.className = 'skeleton-icon';
      icon.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
      row.appendChild(icon);
      const bar = document.createElement('span');
      bar.className = 'skeleton-tree-bar';
      const widths = [60, 80, 45, 70, 55];
      bar.style.width = `${widths[item.depth % widths.length]}%`;
      row.appendChild(bar);
      this.tree.appendChild(row);
    }
  }

  private initDrag(): void {
    const colHandle = document.getElementById('viewerDragHandle')!;
    const rowHandle = document.getElementById('viewerHeightHandle')!;
    const content = document.getElementById('viewerContent')!;
    let colDragging = false;
    let rowDragging = false;

    colHandle.addEventListener('mousedown', (e) => {
      e.preventDefault();
      colDragging = true;
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
    });

    rowHandle.addEventListener('mousedown', (e) => {
      e.preventDefault();
      rowDragging = true;
      document.body.style.cursor = 'row-resize';
      document.body.style.userSelect = 'none';
    });

    document.addEventListener('mousemove', (e) => {
      if (colDragging) {
        const rect = (colHandle.parentElement as HTMLElement).getBoundingClientRect();
        const minW = 120;
        const maxW = rect.width - 120;
        let w = e.clientX - rect.left;
        if (w < minW) w = minW;
        if (w > maxW) w = maxW;
        this.treePanel.style.flex = `0 0 ${w}px`;
      }
      if (rowDragging) {
        const contentRect = content.getBoundingClientRect();
        const minH = 200;
        const maxH = window.innerHeight - contentRect.top - 40;
        let h = e.clientY - contentRect.top;
        if (h < minH) h = minH;
        if (h > maxH) h = maxH;
        content.style.height = h + 'px';
      }
    });

    document.addEventListener('mouseup', () => {
      if (colDragging) {
        colDragging = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
      if (rowDragging) {
        rowDragging = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
    });
  }

  private renderTree(nodes: FileNode[], diff: DiffResult | null): void {
    this.tree.innerHTML = '';
    const diffMap = diff ? this.buildDiffMap(diff) : null;

    // Inject virtual nodes for added files not in the current snapshot's tree
    let allNodes = nodes;
    if (diff) {
      const existingPaths = new Set<string>();
      for (const n of nodes) {
        if (n.path) existingPaths.add(n.path.replace(/^\//, ''));
      }
      for (const cs of (diff.change_sets || [])) {
        if (cs.type !== 'added') continue;
        for (const p of (cs.paths || [])) {
          const norm = p.replace(/^\.\//, '').replace(/^\//, '');
          if (!norm || existingPaths.has(norm)) continue;
          allNodes = [...allNodes, {
            name: norm.split('/').pop() || norm,
            type: 'file',
            path: '/' + norm,
            full_path: p,
          }];
          existingPaths.add(norm);
        }
      }
    }

    const root: any = { name: '/', type: 'dir', children: {} };
    for (const n of allNodes) {
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
    this.renderNode(root, this.tree, 0, diffMap);
  }

  private buildDiffMap(diff: DiffResult): Map<string, string> {
    const m = new Map<string, string>();
    for (const cs of (diff.change_sets || [])) {
      for (const p of (cs.paths || [])) {
        m.set(p, cs.type);
        // Normalize: strip leading ./ and / so it matches tree node paths
        const norm = p.replace(/^\.\//, '').replace(/^\//, '');
        m.set(norm, cs.type);
        if (norm.includes('/')) {
          // Store parent-relative path too (e.g. "config/app.json")
          const parentRel = norm.split('/').slice(1).join('/');
          if (parentRel) m.set(parentRel, cs.type);
        }
      }
    }
    return m;
  }

  private renderNode(node: any, parent: HTMLElement, depth: number, diffMap: Map<string, string> | null): void {
    if (node.name === '/' && depth === 0) {
      const sorted = Object.values(node.children).sort((a: any, b: any) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      for (const child of sorted as any[]) {
        this.renderNode(child, parent, depth + 1, diffMap);
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
      icon.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;opacity:0.7;"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>';
      summary.prepend(icon);
      details.appendChild(summary);
      const content = document.createElement('div');
      content.style.marginLeft = '18px';
      const sorted = (node.children ? Object.values(node.children) : []).sort((a: any, b: any) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      for (const child of sorted as any[]) {
        this.renderNode(child, content, depth + 1, diffMap);
      }
      details.appendChild(content);
      parent.appendChild(details);
    } else {
      // Determine diff type for this file
      let diffType = '';
      if (diffMap) {
        const nodePath = (node.path || '').replace(/^\//, '');
        diffType = diffMap.get(node.full_path || '') || diffMap.get(nodePath) || diffMap.get(node.name) || '';
      }
      const diffColor = diffType === 'added' ? 'var(--green)' : diffType === 'removed' ? 'var(--red)' : diffType === 'modified' ? 'var(--yellow)' : '';
      const otherId = this.diffOtherId;

      const item = document.createElement('div');
      item.style.cursor = 'pointer';
      item.style.padding = '2px 4px';
      item.style.borderRadius = '4px';
      item.style.fontSize = '0.9rem';
      item.style.display = 'flex';
      item.style.alignItems = 'center';
      item.style.gap = '4px';
      const icon = document.createElement('span');
      icon.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;opacity:0.7;"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
      const nameSpan = document.createElement('span');
      nameSpan.textContent = node.name;
      if (diffColor) {
        nameSpan.style.color = diffColor;
      }
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

      // Add view button for diff items
      if (diffType && otherId) {
        const viewBtn = document.createElement('a');
        viewBtn.href = '#';
        viewBtn.style.color = 'var(--accent)';
        viewBtn.style.fontSize = '0.75rem';
        viewBtn.style.textDecoration = 'none';
        viewBtn.style.marginLeft = 'auto';
        if (diffType === 'modified') {
          viewBtn.textContent = 'view diff';
          viewBtn.addEventListener('click', (e: Event) => {
            e.preventDefault();
            e.stopPropagation();
            this.showFileDiff(node.full_path || node.path, otherId);
          });
        } else {
          const vid = diffType === 'added' ? otherId : this.currentSnapshot!.id;
          viewBtn.textContent = diffType === 'added' ? 'view new' : 'view old';
          viewBtn.addEventListener('click', (e: Event) => {
            e.preventDefault();
            e.stopPropagation();
            this.viewFileFromId(node.full_path || node.path, vid);
          });
        }
        item.appendChild(viewBtn);
      }

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
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(this.currentSnapshot.id)}/dump?path=${encodeURIComponent(path)}&volume=${encodeURIComponent(this.currentSnapshot.volume)}`);
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
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(snapshotId)}/dump?path=${encodeURIComponent(path)}&volume=${encodeURIComponent(this.currentSnapshot!.volume)}`);
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
  async populateCompareSelect(snapshot: Snapshot): Promise<void> {
    const vol = snapshot.volume;
    if (!vol) {
      console.warn('populateCompareSelect: snapshot has no volume', snapshot);
      this.compHeader.style.display = 'none';
      this.hideCompareSkeleton();
      return;
    }

    // If allSnapshots is empty, fetch; otherwise use the cached list
    if (this.allSnapshots.length === 0) {
      const resp = await fetch(`/api/snapshots?volume=${encodeURIComponent(vol)}`);
      if (!resp.ok) {
        console.warn('populateCompareSelect: snapshots API returned', resp.status, await resp.text().catch(() => ''));
        this.compHeader.style.display = 'none';
        this.hideCompareSkeleton();
        return;
      }
      this.allSnapshots = await resp.json() as Snapshot[];
    }

    this.compSelect.innerHTML = '';
    let count = 0;
    for (const sn of this.allSnapshots) {
      if (sn.id === snapshot.id || sn.short_id === snapshot.short_id) continue;
      if (sn.volume !== vol) continue;
      const opt = document.createElement('option');
      opt.value = sn.id;
      opt.textContent = `${sn.short_id} — ${sn.hostname || '?'} (${new Date(sn.time).toLocaleString()})`;
      this.compSelect.appendChild(opt);
      count++;
    }
    if (count > 0) {
      this.compHeader.style.display = 'block';
      this.hideCompareSkeleton();
    } else {
      this.hideCompareSkeleton();
    }
  }

  private diffOtherId = '';
  private sideBySide = false;
  private currentDiffPath = '';
  private currentDiffData: { t: string; line: string }[] = [];

  async doDiff(): Promise<void> {
    if (!this.currentSnapshot) return;
    const otherId = this.compSelect.value;
    const otherLabel = this.compSelect.selectedOptions[0]?.textContent || otherId;
    if (!otherId) {
      this.detail.innerHTML = '<span style="color:var(--muted)">Select a snapshot to compare.</span>';
      return;
    }
    this.diffOtherId = otherId;
    this.detail.innerHTML = `<span style="color:var(--muted);font-size:0.85rem;">Comparing with ${otherLabel}</span>`;
    this.tree.innerHTML = '<div style="text-align:center;padding:40px;color:var(--muted);font-size:0.9rem;">Loading diff...</div>';
    try {
      const resp = await fetch(`/api/snapshot-view/${encodeURIComponent(this.currentSnapshot.id)}/diff/${encodeURIComponent(otherId)}?volume=${encodeURIComponent(this.currentSnapshot.volume)}`);
      if (!resp.ok) throw new Error('Diff failed');
      const result = await resp.json() as DiffResult;
      this.currentDiffResult = result;
      this.clearDiffBtn.style.display = '';
      this.renderTree(this.lastNodes, result);
    } catch (err) {
      this.showError((err as Error).message);
    }
  }

  private clearDiff(): void {
    this.currentDiffResult = null;
    this.diffOtherId = '';
    this.clearDiffBtn.style.display = 'none';
    this.detail.innerHTML = '';
    this.renderTree(this.lastNodes, null);
  }

  async showFileDiff(path: string, otherId: string): Promise<void> {
    if (!this.currentSnapshot) return;
    this.detail.innerHTML = `<span style="color:var(--muted)">Loading diff for ${this.escapeHtml(path)}...</span>`;
    try {
      const vol = this.currentSnapshot.volume;
      const [respA, respB] = await Promise.all([
        fetch(`/api/snapshot-view/${encodeURIComponent(this.currentSnapshot.id)}/dump?path=${encodeURIComponent(path)}&volume=${encodeURIComponent(vol)}`),
        fetch(`/api/snapshot-view/${encodeURIComponent(otherId)}/dump?path=${encodeURIComponent(path)}&volume=${encodeURIComponent(vol)}`),
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
      this.currentDiffPath = path;
      this.currentDiffData = this.computeUnifiedDiff(linesA, linesB);
      this.renderCurrentDiff();
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

  private renderCurrentDiff(): void {
    if (this.sideBySide) {
      this.renderSideBySideDiff(this.currentDiffData, this.currentDiffPath);
    } else {
      this.renderUnifiedDiff(this.currentDiffData, this.currentDiffPath);
    }
  }

  private toggleDiffLayout(): void {
    this.sideBySide = !this.sideBySide;
    this.renderCurrentDiff();
  }

  private renderUnifiedDiff(diff: { t: string; line: string }[], path: string): void {
    const toggleLabel = this.sideBySide ? 'Side by side' : 'Inline';
    const html: string[] = [`<div style="margin-bottom:8px;font-weight:700;display:flex;align-items:center;gap:8px;"><span>Diff: ${this.escapeHtml(path)}</span><button id="diffToggleBtn" style="font-size:0.75rem;padding:2px 8px;cursor:pointer;background:var(--bg);color:var(--text);border:1px solid var(--border);border-radius:4px;">${toggleLabel}</button></div>`];
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
    const btn = document.getElementById('diffToggleBtn');
    if (btn) btn.addEventListener('click', () => this.toggleDiffLayout());
  }

  private renderSideBySideDiff(diff: { t: string; line: string }[], path: string): void {
    const toggleLabel = this.sideBySide ? 'Side by side' : 'Inline';
    const html: string[] = [`<div style="margin-bottom:8px;font-weight:700;display:flex;align-items:center;gap:8px;"><span>Diff: ${this.escapeHtml(path)}</span><button id="diffToggleBtn" style="font-size:0.75rem;padding:2px 8px;cursor:pointer;background:var(--bg);color:var(--text);border:1px solid var(--border);border-radius:4px;">${toggleLabel}</button></div>`];
    html.push('<div style="display:grid;grid-template-columns:1fr 1fr;gap:0;font-size:0.85rem;font-family:monospace;">');
    html.push('<div style="padding:2px 4px;font-weight:600;border-bottom:1px solid var(--border);background:rgba(255,255,255,0.03);">Old</div>');
    html.push('<div style="padding:2px 4px;font-weight:600;border-bottom:1px solid var(--border);background:rgba(255,255,255,0.03);">New</div>');

    let ctxCount = 0;
    const pendingDel: string[] = [];
    const pendingAdd: string[] = [];
    const flushCtx = () => {
      if (ctxCount > 3) {
        html.push(`<div style="grid-column:1/3;color:var(--muted);font-size:0.75rem;padding:1px 4px;">... ${ctxCount - 3} common lines hidden ...</div>`);
      }
      ctxCount = 0;
    };
    const flushPending = () => {
      if (pendingDel.length === 0 && pendingAdd.length === 0) return;
      const maxLen = Math.max(pendingDel.length, pendingAdd.length);
      for (let i = 0; i < maxLen; i++) {
        const del = i < pendingDel.length ? this.escapeHtml(pendingDel[i]) : '';
        const add = i < pendingAdd.length ? this.escapeHtml(pendingAdd[i]) : '';
        const delBg = del ? 'background:rgba(248,113,113,0.1);' : 'background:rgba(255,255,255,0.02);';
        const addBg = add ? 'background:rgba(52,211,153,0.1);' : 'background:rgba(255,255,255,0.02);';
        html.push(`<div style="padding:1px 4px;${delBg}white-space:pre-wrap;">${del}</div>`);
        html.push(`<div style="padding:1px 4px;${addBg}white-space:pre-wrap;">${add}</div>`);
      }
      pendingDel.length = 0;
      pendingAdd.length = 0;
    };

    for (const entry of diff) {
      if (entry.t === 'ctx') {
        flushPending();
        ctxCount++;
        if (ctxCount <= 3) {
          const line = entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line;
          const escaped = this.escapeHtml(line);
          html.push(`<div style="padding:1px 4px;background:rgba(255,255,255,0.02);white-space:pre-wrap;">${escaped}</div>`);
          html.push(`<div style="padding:1px 4px;background:rgba(255,255,255,0.02);white-space:pre-wrap;">${escaped}</div>`);
        }
      } else if (entry.t === 'del') {
        flushCtx();
        pendingDel.push(entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line);
      } else if (entry.t === 'add') {
        flushCtx();
        pendingAdd.push(entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line);
      }
    }
    flushPending();
    flushCtx();
    html.push('</div>');
    this.detail.innerHTML = html.join('\n');
    const btn = document.getElementById('diffToggleBtn');
    if (btn) btn.addEventListener('click', () => this.toggleDiffLayout());
  }

  private escapeHtml(s: string): string {
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }

  close(): void {
    this.panel.style.display = 'none';
    this.currentSnapshot = null;
    this.currentDiffResult = null;
    this.diffOtherId = '';
    this.tree.innerHTML = '';
    this.detail.innerHTML = '';
    this.compHeader.style.display = 'none';
    this.clearDiffBtn.style.display = 'none';
    this.hideCompareSkeleton();
  }
}

new SnapshotViewer();
