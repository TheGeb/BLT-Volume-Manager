<script lang="ts">
  import { tick } from 'svelte';
  import { slide, fade } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import type { Snapshot, FileNode, DiffResult } from '$lib/types';
  import { computeDiff } from '$lib/diff';
  import type { DiffHunk, DiffLine } from '$lib/diff';
  import { formatBytes, versionTag, safeErrorMessage } from '$lib/util';
  import { collectAllPaths, buildDiffMap, buildTree } from '$lib/tree-utils';
  import { colResize, rowResize } from '$lib/resize';
  import * as api from '$lib/api';
  import { showToast } from '$lib/stores/toast';

  import { Button } from 'bits-ui';
  import FileTreeNode from './FileTreeNode.svelte';
  import FileDiff from './FileDiff.svelte';
  import FilterInput from '../../components/FilterInput.svelte';
  import Spinner from '../../components/Spinner.svelte';

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
  export let onClose: () => void = () => {};
  export let initialDiffTarget: string = '';
  export let onDiffChange: (otherId: string) => void = () => {};
  export let onSwapDiff: (snap: Snapshot, newDiffId: string, newSnapshotHash?: string) => void = () => {};
  export let onOpenDiffPicker: () => void = () => {};

  let nodes: FileNode[] = [];
  let loading = true;
  let error = '';
  let fileContent = '';
  let fileContentError = '';
  let fileContentPath = '';
  let fileContentLoading = false;
  let fileRequestGeneration = 0;
  let currentDiffResult: DiffResult | null = null;
  let sideBySide = false;
  let diffOtherId = '';
  let compareSnaps: Snapshot[] = [];
  let selectedCompareId = '';

  let diffLoading = false;
  let diffRequestGeneration = 0;
  let treePanelStartWidth = 280;
  let selectedCompareSnap: Snapshot | undefined;
  let compareSnapshotList: Snapshot[] = [];

  async function loadCompareSnapshots() {
    if (!snapshot.volume) return;
    try {
      const r = await api.fetchSnapshots(snapshot.volume, {});
      compareSnapshotList = r.snapshots;
    } catch (e: unknown) {
      compareSnapshotList = [];
      showToast(safeErrorMessage(e), true);
    }
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
  $: diffOtherSnapshot = diffOtherId ? compareSnapshotList.find(s => s.id === diffOtherId || s.short_id === diffOtherId) : null;
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
    fileRequestGeneration++;
    diffRequestGeneration++;
    hasOpened = false;
    loading = true;
    error = '';
    fileContent = '';
    fileContentError = '';
    fileContentPath = '';
    currentDiffResult = null;
    currentDiffHunks = [];
    diffOtherId = '';
    sideBySide = false;
    try {
      if (initialDiffTarget && compareSnaps.some(s => (s.id === initialDiffTarget || s.short_id === initialDiffTarget) && s.id !== snapshot.id)) {
        const target = compareSnaps.find(s => (s.id === initialDiffTarget || s.short_id === initialDiffTarget) && s.id !== snapshot.id);
        selectedCompareId = target ? target.id : initialDiffTarget;
      }
      const [fetchedNodes] = await Promise.all([
        api.fetchFileTree(snapshot.id, snapshot.volume!, undefined, snapshot.fallbackHash),
        loadCompareSnapshots(),
      ]);
      nodes = fetchedNodes;
    } catch (e: unknown) {
      error = safeErrorMessage(e);
    } finally {
      loading = false;
      hasOpened = true;
    }
    if (initialDiffTarget && selectedCompareId) {
      // eslint-disable-next-line svelte/infinite-reactive-loop
      await doDiff(true);
    }
  }

  $: compareSnaps = compareSnapshotList;
  $: selectedCompareSnap = compareSnaps.find(s => s.id === selectedCompareId || s.short_id === selectedCompareId);
  $: if (compareSnaps.length > 0 && !selectedCompareId) {
    const firstEnabled = compareSnaps.find(s => s.id !== snapshot.id);
    selectedCompareId = firstEnabled ? firstEnabled.id : compareSnaps[0]!.id;
  }
  let lastInitialDiffTarget = '';
  $: if (initialDiffTarget) {
    const found = compareSnaps.find(s => (s.id === initialDiffTarget || s.short_id === initialDiffTarget) && s.id !== snapshot.id);
    if (found) {
      selectedCompareId = found.id;
      if (hasOpened && found.id !== diffOtherId && !loading && !diffLoading) {
        // eslint-disable-next-line svelte/infinite-reactive-loop
        doDiff();
      }
      lastInitialDiffTarget = initialDiffTarget;
    } else {
      lastInitialDiffTarget = initialDiffTarget;
    }
  } else if (selectedCompareId && compareSnaps.length > 0 && (!compareSnaps.some(s => s.id === selectedCompareId || s.short_id === selectedCompareId) || selectedCompareId === snapshot.id)) {
    const firstEnabled = compareSnaps.find(s => s.id !== snapshot.id);
    selectedCompareId = firstEnabled ? firstEnabled.id : compareSnaps[0]!.id;
  }
  $: if (!loading && !diffLoading && !warmupDone && nodes.length > 0) {
    warmupDone = true;
    warmupAnimations();
  }

  async function doDiff(skipCallback = false) {
    if (!selectedCompareId) return;
    const generation = ++diffRequestGeneration;
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
      if (generation !== diffRequestGeneration || diffOtherId !== targetId) return;
      // eslint-disable-next-line svelte/infinite-reactive-loop
      currentDiffResult = result;
      sideBySide = false;
      fileContent = '';
      fileContentError = '';
      fileContentPath = '';
      if (!skipCallback) {
        onDiffChange(targetId);
      }
    } catch (e: unknown) {
      if (generation !== diffRequestGeneration) return;
      error = safeErrorMessage(e);
    } finally {
      // eslint-disable-next-line svelte/infinite-reactive-loop
      if (generation === diffRequestGeneration) diffLoading = false;
    }
  }

  function clearDiff() {
    fileRequestGeneration++;
    diffRequestGeneration++;
    currentDiffResult = null;
    diffOtherId = '';
    currentDiffHunks = [];
    fileContent = '';
    fileContentError = '';
    fileContentPath = '';
    diffLoading = false;
    selectedCompareId = '';
    onDiffChange('');
  }

  function handleSwapDiff() {
    if (!diffOtherId) return;
    const otherSnap = compareSnaps.find(s => s.id === diffOtherId || s.short_id === diffOtherId);
    if (!otherSnap) return;
    onSwapDiff(otherSnap, snapshot.id);
  }

  async function viewFile(path: string) {
    const generation = ++fileRequestGeneration;
    fileContent = '';
    fileContentError = '';
    fileContentLoading = true;
    fileContentPath = path;
    currentDiffHunks = [];
    error = '';
    try {
      const content = await api.fetchFileContent(snapshot.id, snapshot.volume!, path, snapshot.fallbackHash);
      if (generation !== fileRequestGeneration) return;
      fileContent = content;
    } catch (e: unknown) {
      if (generation !== fileRequestGeneration) return;
      fileContentError = safeErrorMessage(e);
    } finally {
      if (generation === fileRequestGeneration) fileContentLoading = false;
    }
  }

  async function viewFileFromId(path: string, id: string) {
    const generation = ++fileRequestGeneration;
    fileContent = '';
    fileContentError = '';
    fileContentLoading = true;
    fileContentPath = path;
    currentDiffHunks = [];
    error = '';
    try {
      const snap = compareSnapshotList.find(s => s.id === id)!;
      const content = await api.fetchFileContent(id, snapshot.volume!, path, snap.fallbackHash);
      if (generation !== fileRequestGeneration) return;
      fileContent = content;
    } catch (e: unknown) {
      if (generation !== fileRequestGeneration) return;
      fileContentError = safeErrorMessage(e);
    } finally {
      if (generation === fileRequestGeneration) fileContentLoading = false;
    }
  }

  async function showFileDiff(path: string, otherId: string) {
    const generation = ++fileRequestGeneration;
    fileContent = '';
    fileContentError = '';
    fileContentPath = path;
    currentDiffHunks = [];
    fileContentLoading = true;
    error = '';
    try {
      const snapA = snapshot;
      const snapB = compareSnapshotList.find(s => s.id === otherId)!;
      const [hashA, hashB] = await Promise.all([
        snapA.fallbackHash,
        snapB.fallbackHash
      ]);
      const [oldContent, newContent] = await Promise.all([
        api.fetchFileContent(snapshot.id, snapshot.volume!, path, hashA),
        api.fetchFileContent(otherId, snapshot.volume!, path, hashB),
      ]);
      if (generation !== fileRequestGeneration) return;
      currentDiffHunks = computeDiff(oldContent.split('\n'), newContent.split('\n'));
      sideBySide = false;
      if (currentDiffHunks.length === 0) fileContent = newContent;
    } catch (e: unknown) {
      if (generation !== fileRequestGeneration) return;
      error = safeErrorMessage(e);
    } finally {
      if (generation === fileRequestGeneration) fileContentLoading = false;
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
  let hasOpened = false;

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

<section class="panel" style="margin-bottom:16px;" bind:this={panelEl}>
  <div style="display:flex;gap:16px;margin-bottom:20px;align-items:flex-start;">
    <div style="min-width:0;">
      <h2 class="eyebrow" style="margin:0 0 4px;">
        <span style="text-transform:none;" title="{snapshot.id}">{(versionTag(snapshot.tags) ?? 'v_._?') + ' (' + snapshot.short_id + ')'}</span>
      </h2>
      <div class="snap-meta">
        {#if snapshot.hostname}
          <span class="snap-meta-item">Host: <strong>{snapshot.hostname}</strong></span>
        {/if}
        <span class="snap-meta-item">{new Date(snapshot.time).toLocaleDateString()} <span class="snap-meta-muted">{new Date(snapshot.time).toLocaleTimeString()}</span></span>
        <span class="snap-meta-item">Type: <strong>{snapshot.tags.includes('hot') ? 'Hot' : 'Cold'}</strong></span>
        <span class="snap-meta-item">Size: {#if snapSizes[snapshot.id]}<strong>{snapSizes[snapshot.id]}</strong>{:else if snapSizeLoading[snapshot.id]}<Spinner size={10} />{:else}<span class="snap-meta-muted">—</span>{/if}</span>
      </div>
    </div>
    <button class="button button-xs mode-toggle diff-compare-btn" aria-label="Compare with another snapshot" data-tip="Compare with another snapshot" onclick={onOpenDiffPicker}>
      <img src="/compare-arrows.svg" width="24" height="24" alt="Compare" style="transform:scale(1.1);display:block;" />
    </button>
    {#if currentDiffResult && diffOtherSnapshot}
      <div style="display:flex;align-items:center;gap:12px;flex:1;min-width:0;overflow:hidden;" transition:slideFade>
        <div class="snap-meta-divider"></div>
        <div style="flex:1;min-width:0;">
          <h2 class="eyebrow" style="margin:0 0 4px;">
            <span style="text-transform:none;" title="{diffOtherSnapshot.id}">{(versionTag(diffOtherSnapshot.tags) ?? 'v_._?') + ' (' + diffOtherSnapshot.short_id + ')'}</span>
          </h2>
          <div class="snap-meta">
            {#if diffOtherSnapshot.hostname}
              <span class="snap-meta-item">Host: <strong>{diffOtherSnapshot.hostname}</strong></span>
            {/if}
            <span class="snap-meta-item">{new Date(diffOtherSnapshot.time).toLocaleDateString()} <span class="snap-meta-muted">{new Date(diffOtherSnapshot.time).toLocaleTimeString()}</span></span>
            <span class="snap-meta-item">Type: <strong>{diffOtherSnapshot.tags.includes('hot') ? 'Hot' : 'Cold'}</strong></span>
            <span class="snap-meta-item">Size: {#if snapSizes[diffOtherSnapshot.id]}<strong>{snapSizes[diffOtherSnapshot.id]}</strong>{:else if snapSizeLoading[diffOtherSnapshot.id]}<Spinner size={10} />{:else}<span class="snap-meta-muted">—</span>{/if}</span>
          </div>
        </div>
      </div>
    {/if}
    <div style="display:flex;align-items:center;gap:8px;flex-shrink:0;padding-top:4px;margin-left:auto;">
      {#if currentDiffResult}
        <div transition:fade style="display:inline-flex;gap:8px;">
          <Button.Root class="button button-xs btn-sm clear-diff-btn" onclick={clearDiff}>Clear diff</Button.Root>
          <Button.Root class="button button-secondary button-xs btn-sm" onclick={handleSwapDiff}>Swap diff</Button.Root>
        </div>
      {/if}
      <Button.Root class="button button-secondary" onclick={onClose} style="padding:10px 16px;font-size:0.9rem;border-radius:10px;">Close</Button.Root>
    </div>
  </div>

  {#if error}
    <div style="color:var(--red);">{error}</div>
  {/if}

   <div id="viewerContent" style="display:flex;flex-direction:column;gap:8px;height:400px;min-height:200px;max-height:calc(100vh - 350px);" bind:this={contentEl}>
    <div style="display:flex;flex:1;min-height:0;">
      <div id="viewerTreePanel" style="flex:0 0 {treePanelStartWidth}px;display:flex;flex-direction:column;gap:6px;min-width:0;" bind:this={treePanelEl}>
         <div style="display:flex;gap:4px;align-items:center;">
          <FilterInput fill bind:value={treeSearchQuery} bind:fullPath={treeSearchFullPath} placeholder="Search files..." onEnter={nextSearchResult} />
          {#if treeSearchQuery}
            <span style="font-size:0.75rem;color:var(--muted);white-space:nowrap;font-variant-numeric:tabular-nums;">
              {treeSearchResults.length > 0 ? treeSearchIndex + 1 : 0}/{treeSearchResults.length}
            </span>
            <button class="button button-secondary button-xs" style="padding:4px 6px;line-height:1;" disabled={treeSearchResults.length === 0} onclick={prevSearchResult}>▲</button>
            <button class="button button-secondary button-xs" style="padding:4px 6px;line-height:1;" disabled={treeSearchResults.length === 0} onclick={nextSearchResult}>▼</button>
          {/if}
        </div>
        <div style="display:flex;gap:4px;flex-wrap:wrap;">
          <button class="button button-secondary button-xs btn-icon-sm" style="flex:1 0 auto;min-width:70px;position:relative;" onclick={() => toggleAll(true)}>
            <span style="position:absolute;left:8px;">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
            </span>
            <span style="flex:1;text-align:center;">Expand</span>
          </button>
          <button class="button button-secondary button-xs btn-icon-sm" style="flex:1 0 auto;min-width:70px;position:relative;" onclick={() => toggleAll(false)}>
            <span style="position:absolute;left:8px;">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="18 15 12 9 6 15"/></svg>
            </span>
            <span style="flex:1;text-align:center;">Collapse</span>
          </button>
        </div>
        <div id="viewerTree" style="overflow:auto;scrollbar-gutter:stable;border:1px solid var(--border);border-radius:12px;padding:8px;flex:1;opacity:{treeOpacity};will-change:transform;" bind:this={treeEl}>
          {#if loading}
            <div style="text-align:center;padding:40px;">
              <Spinner />
            </div>
          {:else if diffLoading}
            <div style="text-align:center;padding:40px;">
              <Spinner />
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
              <Spinner />
            </div>
          {:else if fileContentPath}
          <div style="font-size:0.82rem;color:{fileContentDiffColor || 'var(--accent)'};font-weight:600;padding:0 0 8px;margin-bottom:8px;border-bottom:1px solid var(--border);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-family:system-ui,sans-serif;">
            {fileContentPath.replace(/^\//, '')}
          </div>
          {#if currentDiffHunks.length > 0}
            <FileDiff
              diffHunks={currentDiffHunks}
              {sideBySide}
              oldLabel={(versionTag(diffOtherSnapshot?.tags ?? []) ?? 'v_._?') + ' (' + diffOtherSnapshot?.short_id + ')'}
              newLabel={(versionTag(snapshot.tags) ?? 'v_._?') + ' (' + snapshot.short_id + ')'}
              onToggleLayout={toggleDiffLayout} />
          {:else if fileContent}<div style="white-space:pre-wrap;">{fileContent.trimStart()}</div>
          {:else if fileContentLoading}
            <div style="text-align:center;padding:40px;">
              <Spinner />
            </div>
          {:else if fileContentError}
            <div style="text-align:center;padding:40px;color:var(--red);font-size:0.9rem;">
              {fileContentError}
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

  .mode-toggle {
    position: relative;
  }

  .diff-compare-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 7px;
    line-height: 1;
    color: #fff;
    background: linear-gradient(to bottom, var(--accent), color-mix(in srgb, var(--accent) 35%, var(--accent-soft)));
    border: 1px solid var(--accent);
  }

  .diff-compare-btn:hover {
    background: linear-gradient(to bottom, color-mix(in srgb, var(--accent) 85%, #fff), color-mix(in srgb, color-mix(in srgb, var(--accent) 35%, var(--accent-soft)) 85%, #fff));
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
