<script lang="ts">
  import { tick } from 'svelte';
  import { slide, fade } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import type { Snapshot, FileNode, DiffResult } from '$lib/types';
  import { computeDiff } from '$lib/diff';
  import type { DiffHunk, DiffLine } from '$lib/diff';
  import { formatBytes } from '$lib/util';
  import { collectAllPaths, buildDiffMap, buildTree } from '$lib/tree-utils';
  import { colResize, rowResize } from '$lib/resize';
  import * as api from '$lib/api';

  import { Button } from 'bits-ui';
  import FileTreeNode from './FileTreeNode.svelte';
  import FileDiff from './FileDiff.svelte';
  import DropSelect from '../../components/DropSelect.svelte';

  function slideFade(node: Element, { duration = 250, direction = 1 } = {}) {
    const opacityEasing = (t: number) => 1 - Math.pow(1 - t, 2);
    return {
      duration,
      easing: cubicOut,
      css: (t: number) => `
        transform: translateY(${(1 - t) * 10 * direction}px);
        opacity: ${opacityEasing(t)};
      `
    };
  }

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
  let treePanelStartWidth = 280;
  let selectWrapEl: HTMLDivElement;

  $: if (selectWrapEl) {
    treePanelStartWidth = selectWrapEl.offsetWidth;
  }

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
  let searchJustNavigated = false;
  let treeSearchFullPath = false;

  $: allTreePaths = rootNode ? collectAllPaths(rootNode) : [];
  $: treeSearchResults = treeSearchQuery
    ? allTreePaths.filter(p => {
        const target = treeSearchFullPath ? p : (p.split('/').pop() || p);
        return target.toLowerCase().includes(treeSearchQuery.toLowerCase());
      })
    : [];

  $: if (treeSearchResults.length > 0) {
    if (searchJustNavigated) {
      searchJustNavigated = false;
    } else {
      const activeIdx = fileContentPath ? treeSearchResults.indexOf(fileContentPath) : -1;
      if (activeIdx >= 0) {
        treeSearchIndex = activeIdx;
      } else if (treeSearchIndex < 0 || treeSearchIndex >= treeSearchResults.length) {
        treeSearchIndex = 0;
      }
    }
  } else if (treeSearchQuery) {
    treeSearchIndex = -1;
  } else {
    treeSearchIndex = -1;
  }

  let searchAncestorPaths: Set<string> = new Set();

  $: if (treeSearchResults.length > 0) {
    // eslint-disable-next-line svelte/prefer-svelte-reactivity
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
    ? treeSearchResults[treeSearchIndex] ?? ''
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
    searchJustNavigated = true;
  }

  function prevSearchResult() {
    if (treeSearchResults.length === 0) return;
    treeSearchIndex = (treeSearchIndex - 1 + treeSearchResults.length) % treeSearchResults.length;
    searchNavCount++;
    searchJustNavigated = true;
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
      const fetchedNodes = await api.fetchFileTree(snapshot.id, snapshot.volume!, undefined, snapshot.fallbackHash);
      nodes = fetchedNodes;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
    if (initialDiffTarget && selectedCompareId) {
      // eslint-disable-next-line svelte/infinite-reactive-loop
      await doDiff(true);
    }
  }

  $: compareSnaps = allSnapshots.filter(s => s.id !== snapshot.id);
  $: if (compareSnaps.length > 0 && !selectedCompareId) {
    selectedCompareId = compareSnaps[0]!.id;
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
    selectedCompareId = compareSnaps[0]!.id;
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
        snapA.fallbackHash,
        snapB.fallbackHash
      ]);
      const result = await api.fetchDiff(snapshot.id, targetId, snapshot.volume!, hashA, hashB);
      if (diffOtherId !== targetId) return;
      // eslint-disable-next-line svelte/infinite-reactive-loop
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
      fileContent = await api.fetchFileContent(snapshot.id, snapshot.volume!, path, snapshot.fallbackHash);
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
      fileContent = await api.fetchFileContent(id, snapshot.volume!, path, snap.fallbackHash);
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
        snapA.fallbackHash,
        snapB.fallbackHash
      ]);
      const [oldContent, newContent] = await Promise.all([
        api.fetchFileContent(snapshot.id, snapshot.volume!, path, hashA),
        api.fetchFileContent(otherId, snapshot.volume!, path, hashB),
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

  let panelEl: HTMLElement;

  $: if (snapshot) {
    currentDiffResult = null;
    warmupDone = false;
    // eslint-disable-next-line svelte/infinite-reactive-loop
    open();
  }
</script>

<section class="panel" style="margin-bottom:16px;position:relative;" bind:this={panelEl}>
  <Button.Root class="button button-secondary" onclick={onClose} style="position:absolute;top:24px;right:24px;padding:10px 16px;font-size:0.9rem;border-radius:10px;">Close</Button.Root>

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
        <span class="snap-meta-item">Type: <strong>{snapshot.tags.includes('hot') ? 'Hot' : 'Cold'}</strong></span>
        <span class="snap-meta-item">Size: {#if snapSizes[snapshot.id]}<strong>{snapSizes[snapshot.id]}</strong>{:else if snapSizeLoading[snapshot.id]}<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;"><path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/></svg>{:else}<span class="snap-meta-muted">—</span>{/if}</span>
      </div>
    </div>
    {#if currentDiffResult && diffOtherSnapshot}
      <div style="display:flex;align-items:center;gap:12px;flex:1;min-width:0;overflow:hidden;" transition:slideFade>
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
            <span class="snap-meta-item">Type: <strong>{diffOtherSnapshot.tags.includes('hot') ? 'Hot' : 'Cold'}</strong></span>
            <span class="snap-meta-item">Size: {#if snapSizes[diffOtherSnapshot.id]}<strong>{snapSizes[diffOtherSnapshot.id]}</strong>{:else if snapSizeLoading[diffOtherSnapshot.id]}<svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;"><path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/><path d="M12 2a10 10 0 0 1 10 10"/></svg>{:else}<span class="snap-meta-muted">—</span>{/if}</span>
          </div>
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
    <div style="gap:12px;margin-bottom:12px;display:flex;align-items:center;">
      {#if compareSnaps.length > 0}
        <div bind:this={selectWrapEl}>
          <DropSelect
            options={compareSnaps.map(cs => ({ value: cs.id, label: `${cs.short_id.slice(0, 8)}… (${new Date(cs.time).toLocaleDateString()} ${new Date(cs.time).toLocaleTimeString()} - ${cs.hostname})` }))}
            value={selectedCompareId}
            onValueChange={(v) => selectedCompareId = v}
          />
        </div>
        <Button.Root class="button button-xs" style="padding:9px 12px;font-size:0.85rem;border-radius:10px;" onclick={() => doDiff()}>Diff</Button.Root>
      {/if}
      {#if currentDiffResult}
        <div transition:fade style="display:inline-flex;gap:8px;">
          <Button.Root class="button button-xs clear-diff-btn" style="padding:9px 12px;font-size:0.85rem;border-radius:10px;" onclick={clearDiff}>Clear diff</Button.Root>
          <Button.Root class="button button-secondary button-xs" style="padding:9px 12px;font-size:0.85rem;border-radius:10px;" onclick={handleSwapDiff}>Swap diff</Button.Root>
        </div>
      {/if}
    </div>
  {/if}

  {#if error}
    <div style="color:var(--red);">{error}</div>
  {/if}

   <div id="viewerContent" style="display:flex;flex-direction:column;gap:8px;height:400px;min-height:200px;max-height:calc(100vh - 350px);" bind:this={contentEl}>
    <div style="display:flex;flex:1;min-height:0;">
      <div id="viewerTreePanel" style="flex:0 0 {treePanelStartWidth}px;display:flex;flex-direction:column;gap:6px;min-width:0;" bind:this={treePanelEl}>
         <div style="display:flex;gap:4px;align-items:center;">
          <input type="search" placeholder="Search files..." bind:value={treeSearchQuery}
            style="flex:1;padding:6px 8px;border-radius:8px;border:1px solid var(--border);background:rgb(255 255 255 / 4%);color:var(--text);font-size:0.8rem;outline:none;min-width:0;" on:keydown={(e) => e.key === 'Enter' && nextSearchResult()} />
          <button class="button button-secondary button-xs mode-toggle" style="padding:7px;line-height:1;color:var(--accent);background:color-mix(in srgb, var(--accent) 12%, transparent);" data-tip={treeSearchFullPath ? 'Full path search (on)' : 'Full path search (off)'} on:click={() => treeSearchFullPath = !treeSearchFullPath}>
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
            <FileTreeNode node={rootNode as FileNode} depth={0} {diffMap} otherId={diffOtherId} currentSnapId={snapshot.id}
              onViewFile={viewFile} onViewFileFromId={viewFileFromId} onShowFileDiff={showFileDiff}
              expanded={rootExpanded} expandKey={expandToggle} activePath={fileContentPath}
              searchResults={treeSearchResults} searchActivePath={searchActivePath} searchAncestorPaths={searchAncestorPaths} />
          {/if}
        </div>
      </div>
      <div style="width:12px;cursor:col-resize;display:flex;align-items:center;justify-content:center;flex-shrink:0;user-select:none;"
        use:colResize={{treePanelEl, contentEl}}>
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
            <FileDiff
              diffHunks={currentDiffHunks}
              {sideBySide}
              oldLabel={diffOtherId.slice(0, 8)}
              newLabel={snapshot.short_id.slice(0, 8)}
              onToggleLayout={toggleDiffLayout} />
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
      <div style="height:10px;cursor:row-resize;display:flex;align-items:center;justify-content:center;flex-shrink:0;user-select:none;"
        use:rowResize={{contentEl, panelEl}}>
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
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    margin-bottom: 6px;
  }

  :global(.clear-diff-btn) {
    background: rgb(255 80 80 / 10%);
    border: 1px solid var(--red);
    color: var(--red);
    font-weight: 600;
  }

  :global(.clear-diff-btn:hover) {
    background: rgb(255 80 80 / 15%) !important;
    border-color: var(--red) !important;
    opacity: 1;
  }
</style>
