<script lang="ts">
  import { slide } from 'svelte/transition';
  import { formatExpiration } from '../lib/util';
  import type { VolumeLockInfo } from '../lib/types';

  export let volumes: string[] = [];
  export let loading = false;
  export let onSelect: (vol: string) => void = () => {};
  export let filter = '';
  export let onFilterChange: (f: string) => void;
  export let volumeLockInfo: Record<string, VolumeLockInfo> = {};

  let filterVal = '';
  let statusFilter: 'all' | 'locked' | 'unlocked' = 'all';
  let hostFilterVal = '';
  $: filterVal = filter;

  function handleInput() { onFilterChange(filterVal); }
  function handleKeydown(e: KeyboardEvent) { if (e.key === 'Escape') { filterVal = ''; onFilterChange(''); } }

  $: hosts = [...new Set(Object.values(volumeLockInfo).map(i => i.owner).filter(Boolean))].sort();

  $: filtered = volumes.filter(v => {
    if (filter && !v.toLowerCase().includes(filter.toLowerCase())) return false;
    const li = volumeLockInfo[v];
    if (!li) return true;
    if (statusFilter === 'locked' && !li.locked) return false;
    if (statusFilter === 'unlocked' && li.locked) return false;
    if (hostFilterVal && li.owner !== hostFilterVal) return false;
    return true;
  });

  interface TreeNode { name: string; path: string; children?: TreeNode[]; }
  interface FlatItem { name: string; path: string; depth: number; isGroup: boolean; }

  $: tree = buildTree(filtered);
  $: if (Object.keys(expanded).length === 0 && tree.length > 0) {
    expanded = allExpanded(tree);
  }
  $: flatItems = flatten(tree, expanded, 0);

  function buildTree(names: string[]): TreeNode[] {
    const root: Record<string, any> = {};
    for (const fullPath of names) {
      const parts = fullPath.split('/');
      let level = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        const isLast = i === parts.length - 1;
        if (!level[part]) level[part] = { _leaf: false, _children: {}, name: part, path: fullPath };
        if (isLast) level[part]._leaf = true;
        level = level[part]._children;
      }
    }
    function toNodes(obj: any, prefix: string): TreeNode[] {
      return Object.keys(obj).sort().map(k => {
        const full = prefix ? `${prefix}/${k}` : k;
        const n = obj[k];
        const childNodes = toNodes(n._children, full);
        return { name: k, path: full, children: childNodes.length > 0 ? childNodes : undefined };
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
    function walk(ns: TreeNode[]) { for (const n of ns) { if (n.children) walk(n.children!); else leaves.push(n.path); } }
    walk(nodes);
    return leaves;
  }

  function computeFolderLocks(nodes: TreeNode[], lockInfo: Record<string, VolumeLockInfo>): Record<string, { owner: string } | null> {
    const result: Record<string, { owner: string } | null> = {};
    function walk(ns: TreeNode[]) {
      for (const node of ns) {
        if (!node.children) continue;
        const leaves = collectLeafVolumes([node]);
        const locks = leaves.map(l => lockInfo[l]).filter(Boolean);
        if (locks.length > 0 && locks.length === leaves.length) {
          const owners = [...new Set(locks.map(l => l.owner).filter(Boolean))];
          result[node.path] = owners.length === 1 ? { owner: owners[0] } : null;
        } else {
          result[node.path] = null;
        }
        walk(node.children);
      }
    }
    walk(nodes);
    return result;
  }

  $: folderLocks = computeFolderLocks(tree, volumeLockInfo);

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

  $: bracketStyles = flatItemLocks.map((curr, idx) => {
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
    for (let i = 0; i < bracketStyles.length; i++) {
      const style = bracketStyles[i];
      if (!style) continue;
      if (style === 'single') { map[i] = true; continue; }
      if (style === 'start') {
        let len = 1;
        for (let j = i + 1; j < bracketStyles.length; j++) {
          if (!bracketStyles[j] || bracketStyles[j] === 'start') break;
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
    for (let i = 0; i < bracketStyles.length; i++) {
      const style = bracketStyles[i];
      if (!style) continue;
      if (style === 'start') {
        let len = 1;
        for (let j = i + 1; j < bracketStyles.length; j++) {
          if (!bracketStyles[j] || bracketStyles[j] === 'start') break;
          len++;
        }
        if (len > 1 && len % 2 === 0) {
          const mid = i + len / 2 - 1;
          map[mid] = 'translateY(50%)';
        }
      }
    }
    return map;
  })();
</script>

<section class="panel" style="display:block;">
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
        {#each hosts as h}
          <option value={h}>{h}</option>
        {/each}
      </select>
    {/if}
  </div>
  <div class="tree-toolbar">
    <span class="tree-count">{filtered.length} volume{filtered.length !== 1 ? 's' : ''}</span>
    <div class="tree-actions">
      <button class="button button-secondary button-xs btn-icon-sm" on:click={expandAllGroups}>
        <svg width="12" height="12" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="6 9 12 15 18 9"/></svg>
        Expand
      </button>
      <button class="button button-secondary button-xs btn-icon-sm" on:click={collapseAllGroups}>
        <svg width="12" height="12" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><polyline points="18 15 12 9 6 15"/></svg>
        Collapse
      </button>
    </div>
  </div>

  {#if loading && volumes.length === 0}
    <div class="tree">
      {#each { length: 6 } as _}
        <div class="tree-row" style="padding-left: 20px;">
          <div class="skeleton" style="width: 100%; height: 24px; border-radius: 6px;"></div>
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
        <div transition:slide|local>
          <div class="tree-row" class:in-bracket={!!bracketStyles[idx]} data-bracket={bracketStyles[idx] || ''} style="padding-left:{20 + item.depth * 20}px;">
            {#if item.isGroup}
              <button class="tree-group" on:click={() => toggle(item.path)} title={item.path}>
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
                <span class="lock-info" style:transform={labelOffset[idx] || ''}>
                  {#if folderLocks[item.path] && labelAtIdx[idx]}
                    <span class="lock-badge lock-locked">
                      <span class="lock-text">Locked:</span>
                    </span>
                    <span class="lock-owner">{folderLocks[item.path].owner}</span>
                  {/if}
                </span>
              </button>
            {:else}
              <a class="tree-volume" href="/?volume={encodeURIComponent(item.path)}" title={item.path}
                on:click={(e) => { if (e.button === 0 && !e.ctrlKey && !e.metaKey && !e.shiftKey) { e.preventDefault(); onSelect(item.path); } }}>
                <div style="width:16px;"></div>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="volume-icon">
                  <ellipse cx="12" cy="5" rx="9" ry="3"/>
                  <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                  <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
                </svg>
                <span class="tree-name">{item.name}</span>
                <span class="lock-info" style:transform={labelOffset[idx] || ''}>
                  {#if volumeLockInfo[item.path]}
                    {#if !bracketStyles[idx] || labelAtIdx[idx]}
                      <span class="lock-badge lock-{volumeLockInfo[item.path].status}">
                        <span class="lock-text">{volumeLockInfo[item.path].status === 'locked' ? 'Locked:' : 'Unlocked'}</span>
                      </span>
                      {#if volumeLockInfo[item.path].owner}
                        <span class="lock-owner">{volumeLockInfo[item.path].owner}</span>
                      {/if}
                    {/if}
                  {:else if !loading && !bracketStyles[idx]}
                    <span class="lock-badge">—</span>
                  {/if}
                </span>
              </a>
            {/if}

          </div>
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
    border: 1px solid var(--border); background: rgba(255,255,255,0.04);
    color: var(--text); padding: 10px 16px; border-radius: 999px;
    font-size: 0.9rem; font-weight: 500; outline: none;
    transition: border-color 0.15s;
  }
  .volume-filter-input:focus { border-color: var(--accent); }
  .volume-filter-input::placeholder { color: var(--muted); }
  .vol-filter-select {
    padding: 8px 12px; border-radius: 10px;
    border: 1px solid var(--border); background: rgba(255,255,255,0.04);
    color: var(--text); font-size: 0.85rem; cursor: pointer; outline: none;
    transition: border-color 0.15s;
  }
  .vol-filter-select:focus { border-color: var(--accent); }
  .vol-filter-select option { background: var(--surface); color: var(--text); }

  .tree-toolbar {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 12px;
  }
  .tree-count { color: var(--muted); font-size: 0.85rem; }
  .tree-actions { display: flex; gap: 6px; }
  :global(.btn-icon-sm) { display: inline-flex; align-items: center; gap: 4px; }

  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
  .empty { color: var(--muted); text-align: center; padding: 40px; margin: 0; }
  .tree { display: flex; flex-direction: column; }
  .tree-row { display: flex; align-items: center; min-height: 36px; position: relative; box-sizing: border-box; }
  .tree-row.in-bracket {
    background: color-mix(in srgb, var(--green) 8%, transparent);
    border-left: 2px solid var(--green);
    border-right: 2px solid var(--green);
  }
  .tree-row[data-bracket="start"] {
    border-top: 2px solid var(--green);
    border-radius: 8px 8px 0 0;
  }
  .tree-row[data-bracket="end"] {
    border-bottom: 2px solid var(--green);
    border-radius: 0 0 8px 8px;
  }
  .tree-row[data-bracket="single"] {
    border-top: 2px solid var(--green);
    border-bottom: 2px solid var(--green);
    border-radius: 8px;
  }
  .tree-row[data-bracket="end"],
  .tree-row[data-bracket="single"] {
    margin-bottom: -2px;
  }
  .tree-row > * { flex-shrink: 0; }

  .tree-group, .tree-volume {
    display: flex; align-items: center; gap: 8px;
    padding: 6px 10px; border: none; border-radius: 8px;
    background: none; color: var(--text); font-size: 0.9rem;
    font-family: inherit; cursor: pointer; text-align: left;
    width: 100%; text-decoration: none;
    transition: background 0.1s;
  }
  .tree-group:hover, .tree-volume:hover { background: rgba(255,255,255,0.06); }
  .tree-group .lock-badge { margin-left: auto; }
  .tree-name { font-weight: 500; white-space: nowrap; }
  .folder-icon, .volume-icon { flex-shrink: 0; opacity: 0.7; }
  .chevron { flex-shrink: 0; transition: transform 0.15s; }

  .lock-info {
    display: flex; align-items: center; gap: 5px; margin-left: auto; flex-shrink: 0; padding-right: 18px;
  }
  .lock-badge {
    display: inline-flex; align-items: center; gap: 5px;
    font-size: 0.78rem; font-weight: 600; color: var(--muted);
  }
  .lock-locked { color: var(--green); }
  .lock-unlocked { color: var(--muted); }
  .lock-owner { font-size: 0.8rem; color: var(--muted); max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .lock-expiry { font-size: 0.78rem; color: var(--muted); white-space: nowrap; }


</style>