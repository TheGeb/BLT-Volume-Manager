<script lang="ts">
  import { slide } from 'svelte/transition';
  import { formatExpiration } from '../lib/util';
  import type { VolumeLockInfo } from '../lib/types';
  import { volumes as allVolumes, openCopyVolModal, openRenameVolModal } from '../lib/stores/volumes';

  export let volumes: string[] = [];
  export let loading = false;
  export let onSelect: (vol: string) => void = () => {};
  export let filter = '';
  export let onFilterChange: (f: string) => void;
  export let volumeLockInfo: Record<string, VolumeLockInfo> = {};

  let filterVal = '';
  let statusFilter: 'all' | 'locked' | 'unlocked' = 'all';
  let hostFilterVal = '';
  let showLockBorders = true;
  let actionsReady = false;
  let readyTimer: ReturnType<typeof setTimeout>;

  function onActionsEnter() {
    actionsReady = false;
    clearTimeout(readyTimer);
    readyTimer = setTimeout(() => { actionsReady = true; }, 250);
  }

  function onActionsLeave() {
    actionsReady = false;
    clearTimeout(readyTimer);
  }

  const savedPref = typeof localStorage !== 'undefined' ? localStorage.getItem('showLockBorders') : null;
  if (savedPref !== null) showLockBorders = JSON.parse(savedPref);

  function toggleLockBorders() {
    showLockBorders = !showLockBorders;
    localStorage.setItem('showLockBorders', JSON.stringify(showLockBorders));
  }

  $: filterVal = filter;

  function handleInput() { onFilterChange(filterVal); }
  function handleKeydown(e: KeyboardEvent) { if (e.key === 'Escape') { filterVal = ''; onFilterChange(''); } }

  $: hosts = [...new Set(Object.values(volumeLockInfo).map(i => i.owner).filter(Boolean))].sort();

  $: filtered = volumes.filter(v => {
    if (filter && !v.toLowerCase().includes(filter.toLowerCase())) return false;
    const lockInfo = volumeLockInfo[v];
    if (!lockInfo) return true;
    if (statusFilter === 'locked' && !lockInfo.locked) return false;
    if (statusFilter === 'unlocked' && lockInfo.locked) return false;
    if (hostFilterVal && lockInfo.owner !== hostFilterVal) return false;
    return true;
  });

  interface TreeNode { name: string; path: string; children?: TreeNode[]; }
  interface FlatItem { name: string; path: string; depth: number; isGroup: boolean; }

  $: tree = buildTree(filtered);
  $: fullTree = buildTree($allVolumes);
  let autoExpanded = false;
  $: if (!autoExpanded && Object.keys(expanded).length === 0 && tree.length > 0) {
    expanded = allExpanded(tree);
    autoExpanded = true;
  }
  $: flatItems = flatten(tree, expanded, 0);

  function buildTree(names: string[]): TreeNode[] {
    const root: Record<string, any> = {};
    for (const fullPath of names) {
      const parts = fullPath.split('/');
      let level = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i]!;
        const isLast = i === parts.length - 1;
        if (!level[part]) level[part] = { _leaf: false, _children: {}, name: part, path: fullPath };
        if (isLast) level[part]!._leaf = true;
        level = level[part]!._children;
      }
    }
    function toNodes(obj: any, prefix: string): TreeNode[] {
      const keys = Object.keys(obj);
      const folders: string[] = [];
      const volumes: string[] = [];
      for (const k of keys) {
        if (obj[k]._leaf && Object.keys(obj[k]._children).length === 0) {
          volumes.push(k);
        } else {
          folders.push(k);
        }
      }
      folders.sort();
      volumes.sort();
      return [...folders, ...volumes].map(k => {
        const full = prefix ? `${prefix}/${k}` : k;
        const n = obj[k];
        const childNodes = toNodes(n._children, full);
        const node: TreeNode = { name: k, path: full };
        if (childNodes.length > 0) node.children = childNodes;
        return node;
      });
    }
    return toNodes(root, '');
  }

  function allExpanded(nodes: TreeNode[]): Record<string, boolean> {
    const all: Record<string, boolean> = {};
    function walk(ns: TreeNode[]) { for (const n of ns) { if (n.children) { all[n.path] = true; walk(n.children); } } }
    walk(nodes);
    return all;
  }

  let expanded: Record<string, boolean> = {};

  $: if (filter || statusFilter !== 'all' || hostFilterVal) {
    expanded = allExpanded(tree);
  }

  function toggle(path: string) {
    expanded = { ...expanded, [path]: !expanded[path] };
  }

  function expandAllGroups() {
    const all: Record<string, boolean> = { ...expanded };
    function walk(nodes: TreeNode[]) {
      for (const n of nodes) {
        if (n.children) {
          all[n.path] = true;
          walk(n.children);
        }
      }
    }
    walk(tree);
    expanded = all;
  }

  function collapseAllGroups() {
    expanded = {};
  }

  function flatten(nodes: TreeNode[], exp: Record<string, boolean>, depth: number): FlatItem[] {
    const items: FlatItem[] = [];
    for (const node of nodes) {
      const isGroup = !!node.children;
      items.push({ name: node.name, path: node.path, depth, isGroup });
      if (isGroup && exp[node.path]) items.push(...flatten(node.children!, exp, depth + 1));
    }
    return items;
  }

  function collectLeafVolumes(nodes: TreeNode[]): string[] {
    const leaves: string[] = [];
    function walk(items: TreeNode[]) { for (const n of items) { if (n.children) walk(n.children!); else leaves.push(n.path); } }
    walk(nodes);
    return leaves;
  }

  function computeFolderLocks(nodes: TreeNode[], lockInfo: Record<string, VolumeLockInfo>): Record<string, { owner: string } | null> {
    const result: Record<string, { owner: string } | null> = {};
    function walk(ns: TreeNode[]) {
      for (const node of ns) {
        if (!node.children) continue;
        const leaves = collectLeafVolumes([node]);
        const locks = leaves.map(l => lockInfo[l]).filter(l => l && l.locked);
        if (locks.length > 0 && locks.length === leaves.length) {
          const owners = [...new Set(locks.map(l => l!.owner).filter(Boolean))];
          result[node.path] = owners.length === 1 ? { owner: owners[0]! } : null;
        } else {
          result[node.path] = null;
        }
        walk(node.children);
      }
    }
    walk(nodes);
    return result;
  }

  $: folderLocks = computeFolderLocks(fullTree, volumeLockInfo);

  $: flatItemLocks = flatItems.map(item => {
    let owner = '';
    let status = '';
    if (item.isGroup) {
      const fl = folderLocks[item.path];
      if (fl) {
        owner = fl.owner;
        status = 'locked';
      }
    } else {
      const li = volumeLockInfo[item.path];
      if (li && li.locked) {
        owner = li.owner;
        status = li.status;
      }
    }
    return { path: item.path, owner, status };
  });

  $: lockStyles = flatItemLocks.map((curr, idx) => {
    if (!curr.owner) return null;
    
    const prev = idx > 0 ? flatItemLocks[idx - 1] : null;
    const next = idx < flatItemLocks.length - 1 ? flatItemLocks[idx + 1] : null;
    
    const isSameAsPrev = prev && prev.owner === curr.owner;
    const isSameAsNext = next && next.owner === curr.owner;
    
    if (isSameAsPrev && isSameAsNext) return 'middle';
    if (isSameAsPrev && !isSameAsNext) return 'end';
    if (!isSameAsPrev && isSameAsNext) return 'start';
    return 'single';
  });

  $: labelAtIdx = (() => {
    const map: Record<number, boolean> = {};
    for (let i = 0; i < lockStyles.length; i++) {
      const style = lockStyles[i];
      if (!style) continue;
      if (style === 'single') { map[i] = true; continue; }
      if (style === 'start') {
        let len = 1;
        for (let j = i + 1; j < lockStyles.length; j++) {
          if (!lockStyles[j] || lockStyles[j] === 'start') break;
          len++;
        }
        const mid = i + Math.floor((len - 1) / 2);
        map[mid] = true;
      }
    }
    return map;
  })();

  $: labelOffset = (() => {
    const map: Record<number, string> = {};
    for (let i = 0; i < lockStyles.length; i++) {
      const style = lockStyles[i];
      if (!style) continue;
      if (style === 'start') {
        let len = 1;
        for (let j = i + 1; j < lockStyles.length; j++) {
          if (!lockStyles[j] || lockStyles[j] === 'start') break;
          len++;
        }
        if (len > 1 && len % 2 === 0) {
          const mid = i + len / 2 - 1;
          map[mid] = 'translateY(18px)';
        }
      }
    }
    return map;
  })();

  function handleOutroStart(e: Event) {
    const el = e.target as HTMLElement;
    el.setAttribute('data-fading', 'true');
    const row = el.querySelector('.tree-row');
    if (row) row.setAttribute('data-fading', 'true');
  }

  function handleIntroEnd(e: Event) {
    const el = e.target as HTMLElement;
    el.removeAttribute('data-fading');
    const row = el.querySelector('.tree-row');
    if (row) row.removeAttribute('data-fading');
  }
</script>

<section class="panel" class:lock-mode={showLockBorders} style="display:block;">
  <div class="filter-row">
    <input class="volume-filter-input" type="search" placeholder="Filter volumes..."
      bind:value={filterVal} on:input={handleInput} on:keydown={handleKeydown} />
    <select class="vol-filter-select" bind:value={statusFilter}>
      <option value="all">All statuses</option>
      <option value="locked">Locked</option>
      <option value="unlocked">Unlocked</option>
    </select>
    {#if hosts.length > 0}
      <select class="vol-filter-select" bind:value={hostFilterVal}>
        <option value="">All hosts</option>
        {#each hosts as h (h)}
          <option value={h}>{h}</option>
        {/each}
      </select>
    {/if}
  </div>
  <div class="tree-toolbar">
    <div class="tree-actions">
      <button class="button button-secondary button-xs btn-icon-sm" on:click={expandAllGroups}>
        <svg width="12" height="12" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
        Expand
      </button>
      <button class="button button-secondary button-xs btn-icon-sm" on:click={collapseAllGroups}>
        <svg width="12" height="12" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="18 15 12 9 6 15"/></svg>
        Collapse
      </button>
      <button class="button button-secondary button-xs btn-icon-sm" on:click={toggleLockBorders}>
        {showLockBorders ? 'Hide' : 'Show'} locks
      </button>
    </div>
    <span class="tree-count">{filtered.length} volume{filtered.length !== 1 ? 's' : ''}</span>
  </div>

  {#if loading && volumes.length === 0}
    <div class="tree">
      {#each { length: 6 } as _, i (i)}
        <div class="tree-row">
          <div class="skeleton" style="padding-left: 20px; width: 100%; height: 24px; border-radius: 6px;"></div>
        </div>
      {/each}
    </div>
  {:else if flatItems.length === 0 && !filter && statusFilter === 'all' && !hostFilterVal}
    <p class="empty">No volumes found</p>
  {:else if flatItems.length === 0}
    <p class="empty">No volumes match the current filters</p>
  {:else}
    <div class="tree">
      {#each flatItems as item, idx (item.path)}
        <div class="tree-row-wrap" transition:slide|local on:outrostart={handleOutroStart} on:introend={handleIntroEnd}>
          <div class="tree-row" data-lock={lockStyles[idx] || ''} data-fading="false" style="z-index:{labelOffset[idx] ? 2 : 1}">
            {#if item.isGroup}
              <button class="tree-group" on:click={() => toggle(item.path)} title={item.path} style="padding-left:{20 + item.depth * 20}px;">
              <div style="width:22px; display:flex; justify-content:center; align-items:center;">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" class="chevron"
                  style="transform:rotate({expanded[item.path] ? 0 : -90}deg);opacity:0.5; transition:transform 0.15s;">
                  <path d="M7 10l5 5 5-5H7z"/>
                </svg>
              </div>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="folder-icon">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                </svg>
                <span class="tree-name">{item.name}</span>
              </button>
            {:else}
              <button class="tree-volume" title={item.path} style="padding-left:{20 + item.depth * 20}px;"
                on:click={(e) => { if (e.button === 0 && !e.ctrlKey && !e.metaKey && !e.shiftKey) { onSelect(item.path); } }}>
                <div style="width:22px; display:flex; justify-content:center; align-items:center;"></div>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="volume-icon">
                  <ellipse cx="12" cy="5" rx="9" ry="3"/>
                  <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                  <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
                </svg>
                <span class="tree-name">{item.name}</span>
                {#if !item.isGroup}
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <span class="vol-actions" on:mouseenter={onActionsEnter} on:mouseleave={onActionsLeave}>
                  <span class="vol-action-btn" title="Copy volume" role="button" tabindex="0"
                    on:click|stopPropagation={() => { if (actionsReady) openCopyVolModal(item.path); }}
                    on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { if (actionsReady) openCopyVolModal(item.path); } }}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
                    </svg>
                  </span>
                  <span class="vol-action-btn" title="Rename volume" role="button" tabindex="0"
                    on:click|stopPropagation={() => { if (actionsReady) openRenameVolModal(item.path); }}
                    on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { if (actionsReady) openRenameVolModal(item.path); } }}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/>
                    </svg>
                  </span>
                </span>
                {/if}
              </button>
            {/if}
          </div>
          {#if item.isGroup}
            <span class="lock-info" style="opacity:{showLockBorders ? 1 : 0};transition:opacity 0.25s ease, transform 0.25s ease, left 0.25s ease;transform:{labelOffset[idx] || ''}">
              {#if folderLocks[item.path]}
                <span class="lock-label-content" class:visible={labelAtIdx[idx]}>
                  <span class="lock-badge lock-locked">
                    <span class="lock-text">Locked:</span>
                  </span>
                  <span class="lock-owner">{folderLocks[item.path]?.owner}</span>
                </span>
              {/if}
            </span>
          {:else}
            <span class="lock-info" style="opacity:{showLockBorders ? 1 : 0};transition:opacity 0.25s ease, transform 0.25s ease, left 0.25s ease;transform:{labelOffset[idx] || ''}">
              {#if volumeLockInfo[item.path]}
                {#if !lockStyles[idx] || labelAtIdx[idx]}
                  <span class="lock-badge lock-{volumeLockInfo[item.path]!.status}">
                    <span class="lock-text">{volumeLockInfo[item.path]!.status === 'locked' ? 'Locked:' : 'Unlocked'}</span>
                  </span>
                  {#if volumeLockInfo[item.path]!.owner}
                    <span class="lock-owner">{volumeLockInfo[item.path]!.owner}</span>
                  {/if}
                {/if}
              {:else if !loading && !lockStyles[idx]}
                <span class="lock-badge">—</span>
              {/if}
            </span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .filter-row {
    display: flex; gap: 10px; flex-wrap: wrap; padding: 0 0 16px;
  }

  .volume-filter-input {
    flex: 1 1 240px; min-width: 160px;
    border: 1px solid var(--border); background: rgb(255 255 255 / 4%);
    color: var(--text); padding: 10px 16px; border-radius: 999px;
    font-size: 0.9rem; font-weight: 500; outline: none;
    transition: border-color 0.15s;
  }
  .volume-filter-input:focus { border-color: var(--accent); }
  .volume-filter-input::placeholder { color: var(--muted); }

  .vol-filter-select {
    padding: 8px 12px; border-radius: 10px;
    border: 1px solid var(--border); background: rgb(255 255 255 / 4%);
    color: var(--text); font-size: 0.85rem; cursor: pointer; outline: none;
    transition: border-color 0.15s;
  }
  .vol-filter-select:focus { border-color: var(--accent); }
  .vol-filter-select option { background: var(--surface); color: var(--text); }

  .tree-toolbar {
    display: flex; align-items: center; gap: 8px;
    margin-bottom: 12px;
  }
  .tree-count { color: var(--muted); font-size: 0.85rem; }
  .tree-actions { display: flex; gap: 6px; }
  :global(.btn-icon-sm) { display: inline-flex; align-items: center; gap: 4px; }

  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
  .empty { color: var(--muted); text-align: center; padding: 40px; margin: 0; }
  .tree { display: flex; flex-direction: column; contain: layout style; overflow: hidden; padding-bottom: 2px; }

  .tree-row-wrap { position: relative; display: block; padding-bottom: 6px; margin-bottom: -8px; }

  .tree-row {
    display: flex; align-items: stretch; height: 36px; position: relative; box-sizing: border-box;
    contain: layout style; border: 2px solid transparent; background: transparent;
    border-radius: 0; overflow: hidden;
    width: 100%;
    transition: border-color 0.25s ease, background-color 0.25s ease, width 0.25s ease, border-radius 0.25s ease;
  }

  :global(.lock-mode) .tree-row { width: 75%; }

  .panel:not(:global(.lock-mode)) .tree-row[data-lock]:not([data-lock=""]) {
    border-color: var(--surface);
    background: var(--surface);
  }

  :global(.tree-row[data-lock="start"]) {
    border-color: var(--green);
    border-bottom: 0;
    background: var(--green-bg);
    border-radius: 8px 8px 0 0;
  }

  :global(.tree-row[data-lock="middle"]) {
    border-left: 2px solid var(--green);
    border-right: 2px solid var(--green);
    background: var(--green-bg);
    border-radius: 0;
  }

  :global(.tree-row[data-lock="end"]) {
    border-color: var(--green);
    background: var(--green-bg);
    border-radius: 0 0 8px 8px;
    border-top: 0;
  }

  :global(.tree-row[data-lock="single"]) {
    border-color: var(--green);
    background: var(--green-bg);
    border-radius: 8px;
  }

  :global(.tree-row[data-fading="true"]) {
    border-color: var(--surface);
    background: var(--surface);
  }

.tree-row > * { flex-shrink: 0; }

  .tree-group, .tree-volume {
    display: flex; align-items: center; gap: 8px;
    padding: 6px 10px; border: none; border-radius: 8px;
    background: none; color: var(--text); font-size: 0.9rem;
    font-family: inherit; cursor: pointer; text-align: left;
    width: 100%; box-sizing: border-box; text-decoration: none;
    overflow: hidden;
    transition: background 0.15s ease;
  }

  .tree-row[data-lock=""] .tree-group:hover,
  .tree-row[data-lock=""] .tree-volume:hover {
    background: rgb(255 255 255 / 6%);
  }

  .tree-row:not([data-lock=""]) .tree-group:hover,
  .tree-row:not([data-lock=""]) .tree-volume:hover {
    background: color-mix(in srgb, var(--green-bg), rgb(255 255 255 / 10%));
  }

  .panel:not(:global(.lock-mode)) .tree-row[data-lock]:not([data-lock=""]) .tree-group:hover,
  .panel:not(:global(.lock-mode)) .tree-row[data-lock]:not([data-lock=""]) .tree-volume:hover {
    background: transparent;
  }

  .tree-name { font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .folder-icon, .volume-icon { flex-shrink: 0; opacity: 0.7; }
  .chevron { flex-shrink: 0; transition: transform 0.15s; }

  .lock-info {
    display: flex; align-items: center; gap: 5px; flex-shrink: 0; padding: 0 18px 0 6px;
    position: absolute; left: 100%; top: 0; height: 36px;
    z-index: 10;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.25s ease, left 0.25s ease;
  }

  :global(.lock-mode) .lock-info {
    left: 75%;
    pointer-events: auto;
  }

  :global(.tree-row-wrap[data-fading="true"]) .lock-info {
    opacity: 0 !important;
  }

  .lock-label-content {
    opacity: 0;
    transition: opacity 0.25s ease;
  }

  .lock-label-content.visible {
    opacity: 1;
  }

  .lock-badge {
    display: inline-flex; align-items: center; gap: 5px;
    font-size: 0.9rem; font-weight: 600; color: var(--muted);
  }
  .lock-locked { color: var(--green); }
  .lock-unlocked { color: var(--muted); }
  .lock-owner { font-size: 0.9rem; color: var(--muted); max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .lock-expiry { font-size: 0.78rem; color: var(--muted); white-space: nowrap; }

  .vol-actions {
    display: flex;
    align-items: center;
    gap: 2px;
    flex-shrink: 0;
    margin-left: 6px;
    width: 0;
    opacity: 0;
    overflow: hidden;
    transition: width 0.25s ease, opacity 0.25s ease;
  }

  .tree-volume:hover .vol-actions {
    width: 60px;
    opacity: 1;
  }

  .vol-action-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border: none;
    border-radius: 6px;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }

  .vol-action-btn:hover {
    background: rgb(255 255 255 / 10%);
    color: var(--text);
  }

</style>