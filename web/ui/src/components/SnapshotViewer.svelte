<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import type { Snapshot, FileNode, DiffResult } from '../lib/types';
  import { computeDiff } from '../lib/diff';
  import type { DiffHunk, DiffLine } from '../lib/diff';
  import { formatBytes } from '../lib/util';
  import * as api from '../lib/api';
  import { getSnapshotHash } from '../lib/stores';
  import FileTreeNode from './FileTreeNode.svelte';

  export let snapshot: Snapshot;
  export let allSnapshots: Snapshot[] = [];
  export let onClose: () => void = () => {};
  export let initialDiffTarget: string = '';
  export let onDiffChange: (otherId: string) => void = () => {};
  export let onSwapDiff: (newSnapshotId: string, newDiffId: string, newSnapshotHash?: string, newDiffHash?: string) => void = () => {};

  let nodes: FileNode[] = [];
  let loading = true;
  let error = '';
  let fileContent = '';
  let fileContentPath = '';
  let fileContentLoading = false;
  let currentDiffResult: DiffResult | null = null;
  let sideBySide = false;
  let diffOtherId = '';
  let compareSnaps: Snapshot[] = [];
  let selectedCompareId = '';
  let compareLoading = true;
  let diffLoading = false;

  let snapSizes: Record<string, string> = {};
  let snapSizeLoading: Record<string, boolean> = {};

  async function loadSnapSize(id: string) {
    if (snapSizes[id] || snapSizeLoading[id]) return;
    snapSizeLoading = { ...snapSizeLoading, [id]: true };
    try {
      const data = await api.fetchSnapshotSizes(snapshot.volume || '', [id]);
      if (data[id] != null) {
        snapSizes = { ...snapSizes, [id]: formatBytes(data[id]) };
      }
    } catch {
      /* ignore */
    } finally {
      snapSizeLoading = { ...snapSizeLoading, [id]: false };
    }
  }

  let treeEl: HTMLDivElement;
  let treePanelEl: HTMLDivElement;
  let contentEl: HTMLDivElement;

  let treeSearchQuery = '';
  let treeSearchResults: string[] = [];
  let treeSearchIndex = -1;
  let searchNavCount = 0;
  let treeSearchFullPath = false;

  function collectAllPaths(node: any): string[] {
    const paths: string[] = [];
    const p = node.full_path || node.path;
    if (p) paths.push(p);
    if (node.children) {
      const sorted = Object.values(node.children).sort((a: any, b: any) => {
        if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      for (const child of sorted) {
        paths.push(...collectAllPaths(child));
      }
    }
    return paths;
  }

  $: allTreePaths = rootNode ? collectAllPaths(rootNode) : [];
  $: treeSearchResults = treeSearchQuery
    ? allTreePaths.filter(p => {
        const target = treeSearchFullPath ? p : (p.split('/').pop() || p);
        return target.toLowerCase().includes(treeSearchQuery.toLowerCase());
      })
    : [];

  $: if (treeSearchResults.length > 0) {
    const activeIdx = fileContentPath ? treeSearchResults.indexOf(fileContentPath) : -1;
    if (activeIdx >= 0) {
      treeSearchIndex = activeIdx;
    } else if (treeSearchIndex < 0 || treeSearchIndex >= treeSearchResults.length) {
      treeSearchIndex = 0;
    }
  } else if (treeSearchQuery) {
    treeSearchIndex = -1;
  } else {
    treeSearchIndex = -1;
  }

  let searchAncestorPaths: Set<string> = new Set();

  $: if (treeSearchResults.length > 0) {
    const paths = new Set<string>();
    for (const p of treeSearchResults) {
      const parts = p.replace(/^\//, '').split('/');
      let current = '';
      for (let i = 0; i < parts.length - 1; i++) {
        current += '/' + parts[i];
        paths.add(current);
      }
    }
    searchAncestorPaths = paths;
  } else {
    searchAncestorPaths = new Set<string>();
  }

  $: if (searchNavCount || (treeSearchIndex >= 0 && treeSearchResults.length > 0)) {
    tick().then(() => {
      requestAnimationFrame(() => scrollToSearchResult(treeSearchIndex));
    });
  }

  $: searchActivePath = treeSearchResults.length > 0 && treeSearchIndex >= 0
    ? treeSearchResults[treeSearchIndex]
    : '';

  function scrollToSearchResult(index: number) {
    const path = treeSearchResults[index];
    if (!path || !treeEl) return;
    const items = treeEl.querySelectorAll('[data-tree-path]');
    for (const item of items) {
      if (item.getAttribute('data-tree-path') === path) {
        const el = item as HTMLElement;
        const elRect = el.getBoundingClientRect();
        const treeRect = treeEl.getBoundingClientRect();
        const elTop = elRect.top - treeRect.top + treeEl.scrollTop;
        treeEl.scrollTop = elTop - treeEl.clientHeight / 2 + el.offsetHeight / 2;
        break;
      }
    }
  }

  function nextSearchResult() {
    if (treeSearchResults.length === 0) return;
    treeSearchIndex = (treeSearchIndex + 1) % treeSearchResults.length;
    searchNavCount++;
  }

  function prevSearchResult() {
    if (treeSearchResults.length === 0) return;
    treeSearchIndex = (treeSearchIndex - 1 + treeSearchResults.length) % treeSearchResults.length;
    searchNavCount++;
  }

  $: diffMap = currentDiffResult ? buildDiffMap(currentDiffResult) : null;
  $: rootNode = buildTree(nodes, currentDiffResult);
  $: diffOtherSnapshot = diffOtherId ? allSnapshots.find(s => s.id === diffOtherId || s.short_id === diffOtherId) : null;
  $: fileContentDiffType = fileContentPath && diffMap
    ? (diffMap.get(fileContentPath) ?? diffMap.get(fileContentPath.replace(/^\//, '')) ?? '')
    : '';
  $: fileContentDiffColor = !fileContentDiffType ? ''
    : fileContentDiffType === 'added' ? 'var(--green)'
    : fileContentDiffType === 'removed' ? 'var(--red)'
    : fileContentDiffType === 'modified' ? 'var(--yellow)' : '';
  $: if (snapshot) loadSnapSize(snapshot.id);
  $: if (diffOtherId) loadSnapSize(diffOtherId);

  function buildDiffMap(diff: DiffResult): Map<string, string> {
    const m = new Map<string, string>();
    for (const cs of diff.change_sets || []) {
      for (const p of cs.paths || []) {
        m.set(p, cs.type);
        const norm = p.replace(/^\.\//, '').replace(/^\//, '');
        m.set(norm, cs.type);
        if (norm.includes('/')) {
          const parentRel = norm.split('/').slice(1).join('/');
          if (parentRel) m.set(parentRel, cs.type);
        }
      }
    }
    return m;
  }

  function buildTree(allNodes: FileNode[], diff: DiffResult | null): any {
    let nodes = allNodes;
    if (diff) {
      const existingPaths = new Set<string>();
      for (const n of nodes) {
        if (n.path) existingPaths.add(n.path.replace(/^\//, ''));
      }
      for (const cs of diff.change_sets || []) {
        if (cs.type !== 'added') continue;
        for (const p of cs.paths || []) {
          const norm = p.replace(/^\.\//, '').replace(/^\//, '');
          if (!norm || existingPaths.has(norm)) continue;
          nodes = [...nodes, {
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
            cur.children[p] = { name: p, type: 'dir', path: '/' + parts.slice(0, i + 1).join('/'), children: {} };
          } else if (!cur.children[p].children) {
            cur.children[p].children = {};
          }
          cur = cur.children[p];
        }
      }
    }
    computeDirDiffTypes(root, diff ? buildDiffMap(diff) : null);
    return root;
  }

  function computeDirDiffTypes(node: any, dm: Map<string, string> | null): void {
    if (!node.children) return;
    for (const child of Object.values(node.children) as any[]) {
      if (child.children) computeDirDiffTypes(child, dm);
    }
    let result: string | null = null;
    let hasChildWithType = false;
    for (const child of Object.values(node.children) as any[]) {
      let childType: string | null = null;
      if (child.children) {
        childType = child.dirDiffType ?? null;
      } else {
        const t = dm?.get(child.full_path ?? '') ?? dm?.get((child.path ?? '').replace(/^\//, '')) ?? '';
        childType = t || null;
      }
      if (childType === null) continue;
      hasChildWithType = true;
      if (result === null) {
        result = childType;
      } else if (result !== childType) {
        result = null;
        break;
      }
    }
    node.dirDiffType = hasChildWithType ? result : null;
  }

  async function open() {
    loading = true;
    error = '';
    fileContent = '';
    fileContentPath = '';
    currentDiffResult = null;
    currentDiffHunks = [];
    diffOtherId = '';
    sideBySide = false;
    try {
      if (initialDiffTarget && compareSnaps.some(s => s.id === initialDiffTarget || s.short_id === initialDiffTarget)) {
        const target = compareSnaps.find(s => s.id === initialDiffTarget || s.short_id === initialDiffTarget);
        selectedCompareId = target ? target.id : initialDiffTarget;
      }
      const fetchedNodes = await api.fetchFileTree(snapshot.id, snapshot.volume, undefined, snapshot.fallbackHash);
      nodes = fetchedNodes;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
    if (initialDiffTarget && selectedCompareId) {
      await doDiff(true);
    }
  }

  $: compareSnaps = allSnapshots.filter(s => s.id !== snapshot.id);
  $: if (compareSnaps.length > 0 && !selectedCompareId) {
    selectedCompareId = compareSnaps[0].id;
  }
  let lastInitialDiffTarget = '';
  $: if (initialDiffTarget) {
    const found = compareSnaps.find(s => s.id === initialDiffTarget || s.short_id === initialDiffTarget);
    if (found) {
      if (found.id !== selectedCompareId) {
        selectedCompareId = found.id;
      }
      lastInitialDiffTarget = initialDiffTarget;
    } else {
      lastInitialDiffTarget = initialDiffTarget;
    }
  } else if (selectedCompareId && compareSnaps.length > 0 && !compareSnaps.some(s => s.id === selectedCompareId || s.short_id === selectedCompareId)) {
    selectedCompareId = compareSnaps[0].id;
  }
  $: if (allSnapshots.length > 0) {
    compareLoading = false;
  }
  $: if (!loading && !diffLoading && !warmupDone && nodes.length > 0) {
    warmupDone = true;
    warmupAnimations();
  }

  async function doDiff(skipCallback = false) {
    if (!selectedCompareId) return;
    const targetId = selectedCompareId;
    diffLoading = true;
    diffOtherId = targetId;
    try {
      const snapA = snapshot;
      const snapB = compareSnaps.find(s => s.id === targetId || s.short_id === targetId)!;
      const [hashA, hashB] = await Promise.all([
        getSnapshotHash(snapA),
        getSnapshotHash(snapB)
      ]);
      const result = await api.fetchDiff(snapshot.id, targetId, snapshot.volume, hashA, hashB);
      if (diffOtherId !== targetId) return;
      currentDiffResult = result;
      sideBySide = false;
      fileContent = '';
      fileContentPath = '';
      if (!skipCallback) {
        onDiffChange(targetId);
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      diffLoading = false;
    }
  }

  async function clearDiff() {
    currentDiffResult = null;
    diffOtherId = '';
    currentDiffHunks = [];
    fileContent = '';
    fileContentPath = '';
    diffLoading = false;
    onDiffChange('');
  }

  function handleSwapDiff() {
    if (!diffOtherId) return;
    const otherSnap = compareSnaps.find(s => s.id === diffOtherId || s.short_id === diffOtherId);
    if (!otherSnap) return;
    onSwapDiff(otherSnap.id, snapshot.id, otherSnap.fallbackHash, snapshot.fallbackHash);
  }

  async function viewFile(path: string) {
    fileContent = '';
    fileContentLoading = true;
    fileContentPath = path;
    currentDiffHunks = [];
    error = '';
    try {
      fileContent = await api.fetchFileContent(snapshot.id, snapshot.volume, path, await getSnapshotHash(snapshot));
    } catch (e: any) {
      fileContent = 'Error: ' + e.message;
    } finally {
      fileContentLoading = false;
    }
  }

  async function viewFileFromId(path: string, id: string) {
    fileContent = '';
    fileContentLoading = true;
    fileContentPath = path;
    currentDiffHunks = [];
    error = '';
    try {
      const snap = allSnapshots.find(s => s.id === id)!;
      fileContent = await api.fetchFileContent(id, snapshot.volume, path, await getSnapshotHash(snap));
    } catch (e: any) {
      fileContent = 'Error: ' + e.message;
    } finally {
      fileContentLoading = false;
    }
  }

  async function showFileDiff(path: string, otherId: string) {
    fileContent = '';
    fileContentPath = path;
    currentDiffHunks = [];
    fileContentLoading = true;
    error = '';
    try {
      const snapA = snapshot;
      const snapB = allSnapshots.find(s => s.id === otherId)!;
      const [hashA, hashB] = await Promise.all([
        getSnapshotHash(snapA),
        getSnapshotHash(snapB)
      ]);
      const [oldContent, newContent] = await Promise.all([
        api.fetchFileContent(snapshot.id, snapshot.volume, path, hashA),
        api.fetchFileContent(otherId, snapshot.volume, path, hashB),
      ]);
      currentDiffHunks = computeDiff(oldContent.split('\n'), newContent.split('\n'));
      sideBySide = false;
      if (currentDiffHunks.length === 0) fileContent = newContent;
    } catch (e: any) {
      error = e.message;
    } finally {
      fileContentLoading = false;
    }
  }

  let currentDiffHunks: DiffHunk[] = [];

  function toggleDiffLayout() {
    sideBySide = !sideBySide;
  }

  let rootExpanded = true;
  let expandToggle = 0;
  let treeOpacity = 1;
  let warmupDone = false;

  async function warmupAnimations() {
    treeOpacity = 0;
    await tick();
    for (let i = 0; i < 2; i++) {
      toggleAll(false);
      await tick();
      await new Promise(r => requestAnimationFrame(r));
      toggleAll(true);
      await tick();
      await new Promise(r => requestAnimationFrame(r));
    }
    treeOpacity = 1;
  }

  function toggleAll(open: boolean) {
    rootExpanded = open;
    expandToggle++;
  }

  let colDragging = false;
  let rowDragging = false;
  let atBottom = false;
  let atBottomStartHeight = 0;
  let panelEl: HTMLElement;
  let tabPanelEl: HTMLElement | null = null;
  let startX = 0;
  let startY = 0;
  let startWidth = 0;
  let startHeight = 0;
  let startMaxHeight = 0;

  function startColDrag(e: MouseEvent) {
    e.preventDefault();
    colDragging = true;
    startX = e.pageX;
    startWidth = treePanelEl ? treePanelEl.offsetWidth : 300;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  function startRowDrag(e: MouseEvent) {
    e.preventDefault();
    rowDragging = true;
    if (tabPanelEl) tabPanelEl.style.marginBottom = '';
    tabPanelEl = panelEl.closest('.tab-panel') as HTMLElement | null;
    startY = e.clientY;
    startHeight = contentEl!.offsetHeight;
    startMaxHeight = window.innerHeight - contentEl!.getBoundingClientRect().top - 40;
    atBottom = false;
    atBottomStartHeight = startHeight;
    document.body.style.overflow = 'hidden';
    document.body.style.cursor = 'row-resize';
    document.body.style.userSelect = 'none';
  }

  function onMouseMove(e: MouseEvent) {
    if (colDragging && treePanelEl && contentEl) {
      const rect = contentEl.getBoundingClientRect();
      const minW = 200;
      const maxW = rect.width - 200;
      const deltaX = e.pageX - startX;
      let w = startWidth + deltaX;
      if (w < minW) w = minW;
      if (w > maxW) w = maxW;
      treePanelEl.style.minWidth = '0';
      treePanelEl.style.flex = `0 0 ${w}px`;
    }
    if (rowDragging && contentEl) {
      const deltaY = e.clientY - startY;
      const minH = 200;
      let h = startHeight + deltaY;
      if (h < minH) h = minH;
      if (h > startMaxHeight) h = startMaxHeight;
      contentEl.style.height = h + 'px';
      if (h < startHeight) {
        if (!atBottom && window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 1) {
          atBottom = true;
          atBottomStartHeight = h;
        }
        if (atBottom && h < atBottomStartHeight && tabPanelEl) {
          tabPanelEl.style.marginBottom = (atBottomStartHeight - h) + 'px';
        }
      } else if (atBottom) {
        atBottom = false;
        if (tabPanelEl) tabPanelEl.style.marginBottom = '';
      }
    }
  }

  function onMouseUp() {
    if (colDragging) {
      colDragging = false;
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
    if (rowDragging) {
      rowDragging = false;
      atBottom = false;
      document.body.style.overflow = '';
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
  }

  function shrinkMarginOnScroll() {
    if (tabPanelEl) {
      const s = tabPanelEl.style.marginBottom;
      if (s && s !== '' && s !== '0px' && s !== '0') {
        const m = parseFloat(s);
        if (m > 0) {
          const maxScroll = document.documentElement.scrollHeight - window.innerHeight;
          const distFromBottom = maxScroll - window.scrollY;
          if (distFromBottom > 0) {
            tabPanelEl.style.marginBottom = Math.max(0, m - distFromBottom) + 'px';
          }
        }
      }
    }
  }

  $: if (snapshot) {
    currentDiffResult = null;
    warmupDone = false;
    open();
  }

  onMount(() => {
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
    window.addEventListener('scroll', shrinkMarginOnScroll);
  });

  onDestroy(() => {
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
    window.removeEventListener('scroll', shrinkMarginOnScroll);
    if (tabPanelEl) tabPanelEl.style.marginBottom = '';
  });
</script>

<section class="panel" style="margin-bottom:16px;position:relative;" bind:this={panelEl}>
  <button class="button button-secondary button-xs" on:click={onClose} style="position:absolute;top:24px;right:24px;">Close</button>

  <div style="display:flex;gap:12px;margin-bottom:20px;padding-right:70px;">
    <div style="flex:1;min-width:0;">
      <h2 class="eyebrow" style="margin:0 0 4px;">
        Snapshot <span style="text-transform:none;">{snapshot.short_id}</span>
      </h2>
      <div class="snap-meta">
        {#if snapshot.hostname}
          <span class="snap-meta-item">Host: <strong>{snapshot.hostname}</strong></span>
        {/if}
        <span class="snap-meta-item">{new Date(snapshot.time).toLocaleDateString()} <span class="snap-meta-muted">{new Date(snapshot.time).toLocaleTimeString()}</span></span>
        {#if snapshot.tags.length}
          <span class="snap-meta-item">Tags: <strong>{snapshot.tags.join(', ')}</strong></span>
        {/if}
        <span class="snap-meta-item">Size: {#if snapSizes[snapshot.id]}<strong>{snapSizes[snapshot.id]}</strong>{:else if snapSizeLoading[snapshot.id]}<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;"><path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/></svg>{:else}<span class="snap-meta-muted">—</span>{/if}</span>
      </div>
    </div>
    {#if currentDiffResult && diffOtherSnapshot}
      <div style="display:flex;align-items:center;flex-shrink:0;">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="var(--muted)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>
        </svg>
      </div>
      <div class="snap-meta-divider"></div>
      <div style="flex:1;min-width:0;">
        <h2 class="eyebrow" style="margin:0 0 4px;">
          Diff <span style="text-transform:none;">{diffOtherSnapshot.short_id}</span>
        </h2>
        <div class="snap-meta">
          {#if diffOtherSnapshot.hostname}
            <span class="snap-meta-item">Host: <strong>{diffOtherSnapshot.hostname}</strong></span>
          {/if}
          <span class="snap-meta-item">{new Date(diffOtherSnapshot.time).toLocaleDateString()} <span class="snap-meta-muted">{new Date(diffOtherSnapshot.time).toLocaleTimeString()}</span></span>
          {#if diffOtherSnapshot.tags.length}
            <span class="snap-meta-item">Tags: <strong>{diffOtherSnapshot.tags.join(', ')}</strong></span>
          {/if}
          <span class="snap-meta-item">Size: {#if snapSizes[diffOtherSnapshot.id]}<strong>{snapSizes[diffOtherSnapshot.id]}</strong>{:else if snapSizeLoading[diffOtherSnapshot.id]}<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;"><path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/></svg>{:else}<span class="snap-meta-muted">—</span>{/if}</span>
        </div>
      </div>
    {/if}
  </div>

  {#if compareLoading}
    <div id="viewerCompareSkeleton" style="display:flex;margin-bottom:12px;gap:8px;align-items:center;">
      <span class="skeleton-select-bar" style="width:240px;"></span>
      <span class="skeleton-btn-bar"></span>
    </div>
  {:else}
    <div style="margin-bottom:20px;">
      {#if compareSnaps.length > 0}
        <select bind:value={selectedCompareId}>
          {#each compareSnaps as cs}
            <option value={cs.id}>{cs.short_id.slice(0, 8)}… ({new Date(cs.time).toLocaleDateString()})</option>
          {/each}
        </select>
        <button class="button button-xs" style="margin-left:6px;" on:click={() => doDiff()}>Diff</button>
      {/if}
      {#if currentDiffResult}
        <button class="button button-secondary button-xs" style="margin-left:6px;" on:click={clearDiff}>Clear diff</button>
        <button class="button button-secondary button-xs" style="margin-left:6px;" on:click={handleSwapDiff}>Swap diff</button>
      {/if}
    </div>
  {/if}

  {#if error}
    <div style="color:var(--red);">{error}</div>
  {/if}

   <div id="viewerContent" style="display:flex;flex-direction:column;gap:8px;height:400px;min-height:200px;max-height:calc(100vh - 350px);" bind:this={contentEl}>
    <div style="display:flex;flex:1;min-height:0;">
      <div id="viewerTreePanel" style="flex:0 0 300px;display:flex;flex-direction:column;gap:6px;min-width:0;" bind:this={treePanelEl}>
         <div style="display:flex;gap:4px;align-items:center;">
          <input type="search" placeholder="Search files..." bind:value={treeSearchQuery}
            style="flex:1;padding:6px 8px;border-radius:8px;border:1px solid var(--border);background:rgba(255,255,255,0.04);color:var(--text);font-size:0.8rem;outline:none;min-width:0;" on:keydown={(e) => e.key === 'Enter' && nextSearchResult()} />
          <button class="button button-secondary button-xs mode-toggle" style="padding:4px 6px;line-height:1;color:var(--accent);background:color-mix(in srgb, var(--accent) 12%, transparent);" data-tip={treeSearchFullPath ? 'Full path search (on)' : 'Full path search (off)'} on:click={() => treeSearchFullPath = !treeSearchFullPath}>
            {#if treeSearchFullPath}
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round">
                <line x1="17" y1="4" x2="7" y2="20"/>
              </svg>
            {:else}
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
              </svg>
            {/if}
          </button>
          {#if treeSearchQuery}
            <span style="font-size:0.75rem;color:var(--muted);white-space:nowrap;font-variant-numeric:tabular-nums;">
              {treeSearchResults.length > 0 ? treeSearchIndex + 1 : 0}/{treeSearchResults.length}
            </span>
            <button class="button button-secondary button-xs" style="padding:4px 6px;line-height:1;" disabled={treeSearchResults.length === 0} on:click={prevSearchResult}>▲</button>
            <button class="button button-secondary button-xs" style="padding:4px 6px;line-height:1;" disabled={treeSearchResults.length === 0} on:click={nextSearchResult}>▼</button>
          {/if}
        </div>
        <div style="display:flex;gap:4px;flex-wrap:wrap;">
          <button class="button button-secondary button-xs btn-icon-sm" style="flex:1 0 auto;min-width:70px;position:relative;" on:click={() => toggleAll(true)}>
            <span style="position:absolute;left:8px;">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
            </span>
            <span style="flex:1;text-align:center;">Expand</span>
          </button>
          <button class="button button-secondary button-xs btn-icon-sm" style="flex:1 0 auto;min-width:70px;position:relative;" on:click={() => toggleAll(false)}>
            <span style="position:absolute;left:8px;">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="18 15 12 9 6 15"/></svg>
            </span>
            <span style="flex:1;text-align:center;">Collapse</span>
          </button>
        </div>
        <div id="viewerTree" style="overflow:auto;scrollbar-gutter:stable;border:1px solid var(--border);border-radius:12px;padding:8px;flex:1;opacity:{treeOpacity};will-change:transform;" bind:this={treeEl}>
          {#if loading}
            <div style="text-align:center;padding:40px;">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;">
                <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
                <path d="M12 2a10 10 0 0 1 10 10" />
              </svg>
            </div>
          {:else if diffLoading}
            <div style="text-align:center;padding:40px;">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;">
                <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
                <path d="M12 2a10 10 0 0 1 10 10" />
              </svg>
              <div style="color:var(--muted);font-size:0.85rem;margin-top:8px;">Computing diff...</div>
            </div>
          {:else}
            <FileTreeNode node={rootNode} depth={0} {diffMap} otherId={diffOtherId} currentSnapId={snapshot.id}
              onViewFile={viewFile} onViewFileFromId={viewFileFromId} onShowFileDiff={showFileDiff}
              expanded={rootExpanded} expandKey={expandToggle} activePath={fileContentPath}
              searchResults={treeSearchResults} searchActivePath={searchActivePath} searchAncestorPaths={searchAncestorPaths} />
          {/if}
        </div>
      </div>
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div style="width:12px;cursor:col-resize;display:flex;align-items:center;justify-content:center;flex-shrink:0;user-select:none;"
        on:mousedown={startColDrag}>
        <div style="width:3px;height:32px;border-radius:2px;background:var(--border);"></div>
      </div>
      <div id="viewerDetail" style="flex:1;overflow-y:auto;border:1px solid var(--border);border-radius:12px;padding:12px;font-family:monospace;">
        {#if loading}
          <div style="text-align:center;padding:40px;">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;">
              <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
              <path d="M12 2a10 10 0 0 1 10 10" />
            </svg>
          </div>
        {:else if fileContentPath}
          <div style="font-size:0.82rem;color:{fileContentDiffColor || 'var(--accent)'};font-weight:600;padding:0 0 8px;margin-bottom:8px;border-bottom:1px solid var(--border);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-family:system-ui,sans-serif;">
            {fileContentPath.replace(/^\//, '')}
          </div>
          {#if currentDiffHunks.length > 0}
            <div style="display:flex;gap:8px;margin-bottom:8px;align-items:center;">
              <span style="font-size:0.85rem;color:var(--muted);">Diff: {snapshot.short_id.slice(0, 8)} vs {diffOtherId.slice(0, 8)}</span>
              <button class="button button-secondary button-xs" style="margin-left:auto;" on:click={toggleDiffLayout}>
                {sideBySide ? 'Inline' : 'Side-by-side'}
              </button>
            </div>
            {#if sideBySide}
              <div style="display:flex;gap:0;border:1px solid var(--border);border-radius:8px;overflow:hidden;">
                <div style="flex:1;overflow-x:auto;border-right:1px solid var(--border);">
                  <div style="padding:4px 8px;font-size:0.75rem;color:var(--muted);border-bottom:1px solid var(--border);background:rgba(255,255,255,0.03);">Old ({diffOtherId.slice(0, 8)})</div>
                  {#each currentDiffHunks as hunk}
                    <div style="padding:2px 8px;font-size:0.75rem;color:var(--muted);background:rgba(255,255,255,0.03);border-bottom:1px solid var(--border);font-family:monospace;">
                      @@ -{hunk.oldStart},{hunk.oldLen} +{hunk.newStart},{hunk.newLen} @@
                    </div>
                    {#each hunk.lines as entry}
                      {#if entry.type === 'add'}
                        <div style="padding:0 4px;font-size:0.85rem;background:var(--green-bg);">&nbsp;</div>
                      {:else}
                        <div style="display:flex;padding:0 4px;font-size:0.85rem;background:{entry.type === 'del' ? 'var(--red-bg)' : ''};">
                          <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.oldLineNo || ''}</span>
                          <span style="width:1ch;flex-shrink:0;color:{entry.type === 'del' ? 'var(--red)' : ''};">{entry.type === 'del' ? '-' : ' '}</span>
                          <span style="flex:1;white-space:pre-wrap;">{entry.content}</span>
                        </div>
                      {/if}
                    {/each}
                  {/each}
                </div>
                <div style="flex:1;overflow-x:auto;">
                  <div style="padding:4px 8px;font-size:0.75rem;color:var(--muted);border-bottom:1px solid var(--border);background:rgba(255,255,255,0.03);">New ({snapshot.short_id.slice(0, 8)})</div>
                  {#each currentDiffHunks as hunk}
                    <div style="padding:2px 8px;font-size:0.75rem;color:var(--muted);background:rgba(255,255,255,0.03);border-bottom:1px solid var(--border);font-family:monospace;">
                      @@ -{hunk.oldStart},{hunk.oldLen} +{hunk.newStart},{hunk.newLen} @@
                    </div>
                    {#each hunk.lines as entry}
                      {#if entry.type === 'del'}
                        <div style="padding:0 4px;font-size:0.85rem;background:var(--red-bg);">&nbsp;</div>
                      {:else}
                        <div style="display:flex;padding:0 4px;font-size:0.85rem;background:{entry.type === 'add' ? 'var(--green-bg)' : ''};">
                          <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.newLineNo || ''}</span>
                          <span style="width:1ch;flex-shrink:0;color:{entry.type === 'add' ? 'var(--green)' : ''};">{entry.type === 'add' ? '+' : ' '}</span>
                          <span style="flex:1;white-space:pre-wrap;">{entry.content}</span>
                        </div>
                      {/if}
                    {/each}
                  {/each}
                </div>
              </div>
            {:else}
              {#each currentDiffHunks as hunk}
                <div style="padding:2px 8px;margin:4px 0;font-size:0.8rem;color:var(--muted);background:rgba(255,255,255,0.03);border-radius:4px;font-family:monospace;">
                  @@ -{hunk.oldStart},{hunk.oldLen} +{hunk.newStart},{hunk.newLen} @@
                </div>
                {#each hunk.lines as entry}
                  <div style="display:flex;padding:1px 4px;font-size:0.85rem;background:{entry.type === 'add' ? 'var(--green-bg)' : entry.type === 'del' ? 'var(--red-bg)' : ''};border-radius:2px;">
                    <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.type === 'add' ? '' : entry.oldLineNo}</span>
                    <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.type === 'del' ? '' : entry.newLineNo}</span>
                    <span style="width:1.2ch;flex-shrink:0;color:{entry.type === 'add' ? 'var(--green)' : entry.type === 'del' ? 'var(--red)' : ''};">{entry.type === 'add' ? '+' : entry.type === 'del' ? '-' : ' '}</span>
                    <span style="flex:1;white-space:pre-wrap;">{entry.content}</span>
                  </div>
                {/each}
              {/each}
            {/if}
          {:else if fileContent}<div style="white-space:pre-wrap;">{fileContent.trimStart()}</div>
          {:else if fileContentLoading}
            <div style="text-align:center;padding:40px;">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;">
                <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
                <path d="M12 2a10 10 0 0 1 10 10" />
              </svg>
            </div>
          {:else}
            <div style="text-align:center;padding:40px;color:var(--muted);font-size:0.9rem;">
              Select a file to view its contents
            </div>
          {/if}
        {/if}
        </div>
      </div>
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div style="height:10px;cursor:row-resize;display:flex;align-items:center;justify-content:center;flex-shrink:0;user-select:none;"
        on:mousedown={startRowDrag}>
        <div style="width:40px;height:3px;border-radius:2px;background:var(--border);"></div>
      </div>
    </div>
</section>

<style>
  .eyebrow {
    margin: 0 0 8px;
    color: var(--accent);
    font-size: 0.85rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .skeleton-select-bar { height: 32px; width: 240px; border-radius: 8px; background: linear-gradient(90deg, var(--surface) 25%, var(--surface-strong) 37%, var(--surface) 63%); background-size: 200% 100%; animation: shimmer 1.2s ease-in-out infinite; }
  .skeleton-btn-bar { height: 32px; width: 60px; border-radius: 8px; background: linear-gradient(90deg, var(--surface) 25%, var(--surface-strong) 37%, var(--surface) 63%); background-size: 200% 100%; animation: shimmer 1.2s ease-in-out infinite; }
  .snap-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 16px;
    font-size: 0.82rem;
    color: var(--muted);
  }
  .snap-meta-item {
    white-space: nowrap;
  }
  .snap-meta-item strong {
    color: var(--text);
    font-weight: 600;
  }
  .snap-meta-muted {
    color: var(--muted);
  }
  .snap-meta-divider {
    width: 1px;
    background: var(--border);
    flex-shrink: 0;
  }
  .spin {
    animation: spin 1s linear infinite;
    vertical-align: middle;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
  .mode-toggle {
    position: relative;
  }
  .mode-toggle:hover::after {
    content: attr(data-tip);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-strong);
    color: var(--text);
    padding: 6px 10px;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 400;
    white-space: nowrap;
    z-index: 10;
    pointer-events: none;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    margin-bottom: 6px;
  }
</style>
