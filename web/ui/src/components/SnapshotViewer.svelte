<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { Snapshot, FileNode, DiffResult } from '../lib/types';
  import { computeDiff } from '../lib/diff';
  import type { DiffHunk, DiffLine } from '../lib/diff';
  import * as api from '../lib/api';
  import FileTreeNode from './FileTreeNode.svelte';

  export let snapshot: Snapshot;
  export let allSnapshots: Snapshot[] = [];
  export let onClose: () => void = () => {};
  export let initialDiffTarget: string = '';
  export let onDiffChange: (otherId: string) => void = () => {};

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

  let treeEl: HTMLDivElement;
  let treePanelEl: HTMLDivElement;
  let contentEl: HTMLDivElement;

  $: diffMap = currentDiffResult ? buildDiffMap(currentDiffResult) : null;
  $: rootNode = buildTree(nodes, currentDiffResult);

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
            cur.children[p] = { name: p, type: 'dir', children: {} };
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

  async function generateFallbackHash(snap: Snapshot): Promise<string> {
    const msg = snap.hostname + snap.time + snap.paths.sort().join(',');
    const msgUint8 = new TextEncoder().encode(msg);
    const hashBuffer = await crypto.subtle.digest('SHA-256', msgUint8);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const fullHash = hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    return fullHash.substring(0, snap.short_id.length);
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
      nodes = await api.fetchFileTree(snapshot.id, snapshot.volume, undefined, snapshot.fallbackHash);
      if (initialDiffTarget && compareSnaps.some(s => s.id === initialDiffTarget || s.short_id === initialDiffTarget)) {
        const target = compareSnaps.find(s => s.id === initialDiffTarget || s.short_id === initialDiffTarget);
        selectedCompareId = target ? target.id : initialDiffTarget;
        await doDiff();
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $: compareSnaps = allSnapshots.filter(s => s.id !== snapshot.id);
  $: if (compareSnaps.length > 0 && !selectedCompareId) {
    selectedCompareId = compareSnaps[0].id;
  }
  $: if (allSnapshots.length > 0) {
    compareLoading = false;
  }

  async function doDiff() {
    if (!selectedCompareId) return;
    diffLoading = true;
    diffOtherId = selectedCompareId;
    try {
      const snapA = snapshot;
      const snapB = compareSnaps.find(s => s.id === selectedCompareId || s.short_id === selectedCompareId)!;
      const [hashA, hashB] = await Promise.all([
        generateFallbackHash(snapA),
        generateFallbackHash(snapB)
      ]);
      const result = await api.fetchDiff(snapshot.id, selectedCompareId, snapshot.volume, hashA, hashB);
      currentDiffResult = result;
      sideBySide = false;
      fileContent = '';
      fileContentPath = '';
      onDiffChange(selectedCompareId);
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

  async function viewFile(path: string) {
    fileContentLoading = true;
    fileContentPath = path;
    currentDiffHunks = [];
    error = '';
    try {
      const hash = await generateFallbackHash(snapshot);
      fileContent = await api.fetchFileContent(snapshot.id, snapshot.volume, path, hash);
    } catch (e: any) {
      fileContent = 'Error: ' + e.message;
    } finally {
      fileContentLoading = false;
    }
  }

  async function viewFileFromId(path: string, id: string) {
    fileContentLoading = true;
    fileContentPath = path;
    currentDiffHunks = [];
    error = '';
    try {
      const snap = allSnapshots.find(s => s.id === id)!;
      const hash = await generateFallbackHash(snap);
      fileContent = await api.fetchFileContent(id, snapshot.volume, path, hash);
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
        generateFallbackHash(snapA),
        generateFallbackHash(snapB)
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
  function toggleAll(open: boolean) {
    rootExpanded = open;
  }

  let colDragging = false;
  let rowDragging = false;

  function startColDrag(e: MouseEvent) {
    e.preventDefault();
    colDragging = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  function startRowDrag(e: MouseEvent) {
    e.preventDefault();
    rowDragging = true;
    document.body.style.cursor = 'row-resize';
    document.body.style.userSelect = 'none';
  }

  function onMouseMove(e: MouseEvent) {
    if (colDragging && treePanelEl && contentEl) {
      const rect = contentEl.getBoundingClientRect();
      const minW = 40;
      const maxW = rect.width - 200;
      let w = e.clientX - rect.left;
      if (w < minW) w = minW;
      if (w > maxW) w = maxW;
      treePanelEl.style.minWidth = '0';
      treePanelEl.style.flex = `0 0 ${w}px`;
    }
    if (rowDragging && contentEl) {
      const contentRect = contentEl.getBoundingClientRect();
      const minH = 200;
      const maxH = window.innerHeight - contentRect.top - 40;
      let h = e.clientY - contentRect.top;
      if (h < minH) h = minH;
      if (h > maxH) h = maxH;
      contentEl.style.height = h + 'px';
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
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }
  }

  $: if (snapshot) open();

  onMount(() => {
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  });

  onDestroy(() => {
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  });
</script>

<section class="panel" style="margin-bottom:16px;">
  <div class="row gap" style="margin-bottom:12px;">
    <h2 class="eyebrow" style="margin:0;flex:1;">
      Snapshot <span>{snapshot.short_id}</span>
    </h2>
    <button class="button button-secondary button-xs" on:click={onClose}>Close</button>
  </div>

  {#if compareLoading}
    <div id="viewerCompareSkeleton" style="display:flex;margin-bottom:12px;gap:8px;align-items:center;">
      <span class="skeleton-select-bar" style="width:240px;"></span>
      <span class="skeleton-btn-bar"></span>
    </div>
  {:else}
    <div style="margin-bottom:12px;">
      {#if compareSnaps.length > 0}
        <select bind:value={selectedCompareId}>
          {#each compareSnaps as cs}
            <option value={cs.id}>{cs.short_id.slice(0, 8)}… ({new Date(cs.time).toLocaleDateString()})</option>
          {/each}
        </select>
        <button class="button button-xs" style="margin-left:6px;" on:click={doDiff}>Diff</button>
      {/if}
      {#if currentDiffResult}
        <button class="button button-secondary button-xs" style="margin-left:6px;" on:click={clearDiff}>Clear diff</button>
      {/if}
    </div>
  {/if}

  {#if error}
    <div style="color:var(--red);">{error}</div>
  {/if}

   <div id="viewerContent" style="display:flex;gap:0;height:400px;min-height:200px;max-height:calc(100vh - 350px);" bind:this={contentEl}>
    <div id="viewerTreePanel" style="flex:0 0 300px;display:flex;flex-direction:column;gap:6px;min-width:0;" bind:this={treePanelEl}>
      <div style="display:flex;gap:4px;">
        <button class="button button-secondary button-xs btn-icon-sm" style="flex:1;position:relative;" on:click={() => toggleAll(true)}>
          <span style="position:absolute;left:8px;">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
          </span>
          <span style="flex:1;text-align:center;">Expand</span>
        </button>
        <button class="button button-secondary button-xs btn-icon-sm" style="flex:1;position:relative;" on:click={() => toggleAll(false)}>
          <span style="position:absolute;left:8px;">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="18 15 12 9 6 15"/></svg>
          </span>
          <span style="flex:1;text-align:center;">Collapse</span>
        </button>
      </div>
      <div id="viewerTree" style="overflow:auto;border:1px solid var(--border);border-radius:12px;padding:8px;flex:1;" bind:this={treeEl}>
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
            onViewFile={viewFile} onViewFileFromId={viewFileFromId} onShowFileDiff={showFileDiff} expanded={rootExpanded} />
        {/if}
      </div>
    </div>
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div style="width:12px;cursor:col-resize;display:flex;align-items:center;justify-content:center;flex-shrink:0;user-select:none;"
      on:mousedown={startColDrag}>
      <div style="width:3px;height:32px;border-radius:2px;background:var(--border);"></div>
    </div>
    <div id="viewerDetail" style="flex:1;overflow-y:auto;border:1px solid var(--border);border-radius:12px;padding:12px;white-space:pre-wrap;font-family:monospace;">
      {#if loading}
        <div style="text-align:center;padding:40px;">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;">
            <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
            <path d="M12 2a10 10 0 0 1 10 10" />
          </svg>
        </div>
      {:else if currentDiffHunks.length > 0}
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
        {:else if fileContent}
          {fileContent}
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
      </div>
    </div>
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div style="height:10px;cursor:row-resize;display:flex;align-items:center;justify-content:center;user-select:none;margin-top:8px;"
      on:mousedown={startRowDrag}>
      <div style="width:40px;height:3px;border-radius:2px;background:var(--border);"></div>
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
  .spin {
    animation: spin 1s linear infinite;
    vertical-align: middle;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
