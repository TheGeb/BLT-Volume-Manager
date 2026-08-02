<script lang="ts">
  import type { FileNode, DiffResult } from '$lib/types';
  import { formatBytes } from '$lib/util';
  import FileTreeNode from './FileTreeNode.svelte';

  export let node: FileNode;
  export let depth = 0;
  export let diffMap: Map<string, string> | null = null;
  export let otherId = '';
  export let currentSnapId = '';
  export let onViewFile: (path: string) => void = () => {};
  export let onViewFileFromId: (path: string, id: string) => void = () => {};
  export let onShowFileDiff: (path: string, otherId: string) => void = () => {};
  export let expanded = true;
  export let expandKey = 0;
  export let activePath = '';
  export let searchResults: string[] = [];
  export let searchActivePath = '';
  export let searchAncestorPaths: Set<string> = new Set();

  let localExpanded = expanded;
  let fromBulk = true;
  let everExpanded = expanded;
  let forceNoAnim = false;
  let prevSearchAncestorPaths: Set<string> = new Set();
  let prevActivePath = '';
  let prevSearchActivePath = '';

  $: if (localExpanded) everExpanded = true;

  $: noAnim = (fromBulk && depth > 1) || forceNoAnim;

  $: {
    expanded;
    expandKey;
    localExpanded = expanded;
    fromBulk = true;
  }

  $: nodePath = node.full_path || node.path;

  $: if (searchAncestorPaths !== prevSearchAncestorPaths) {
    prevSearchAncestorPaths = searchAncestorPaths;
    if (searchAncestorPaths.size > 0 && nodePath && searchAncestorPaths.has(nodePath) && !localExpanded) {
      forceNoAnim = true;
      localExpanded = true;
    }
  }

  $: if (activePath !== prevActivePath) {
    prevActivePath = activePath;
    if (activePath && nodePath && activePath.startsWith(nodePath + '/') && !localExpanded) {
      localExpanded = true;
    }
  }

  $: if (searchActivePath !== prevSearchActivePath) {
    prevSearchActivePath = searchActivePath;
    if (searchActivePath && nodePath && searchActivePath.startsWith(nodePath + '/') && !localExpanded) {
      localExpanded = true;
    }
  }

  $: if (searchAncestorPaths.size === 0) {
    forceNoAnim = false;
  }

  $: chevronStyle = `transform:rotate(${localExpanded ? 0 : -90}deg);opacity:0.5;${fromBulk ? '' : 'transition:transform 0.15s;'}`;

  $: diffType = diffMap?.get(node.full_path ?? '') ?? diffMap?.get((node.path ?? '').replace(/^\//, '')) ?? diffMap?.get(node.name ?? '') ?? '';
  $: isActive = activePath && (node.path === activePath || node.full_path === activePath);
  $: isSearchMatch = searchResults.length > 0 && nodePath && searchResults.includes(nodePath);
  $: isSearchActive = searchActivePath && (node.path === searchActivePath || node.full_path === searchActivePath);

  $: diffColor = !diffType ? ''
    : diffType === 'added' ? 'var(--green)'
    : diffType === 'removed' ? 'var(--red)'
    : diffType === 'modified' ? 'var(--yellow)' : '';

  $: activeColor = isActive ? (diffColor || 'var(--accent)') : '';
  $: fileColor = activeColor || (diffColor || 'var(--text)');
  $: fileWeight = isActive ? 700 : (isSearchActive ? 600 : 400);
  $: fileBg = isActive
    ? `color-mix(in srgb, ${activeColor} 12%, transparent)`
    : isSearchActive
      ? 'color-mix(in srgb, var(--accent) 14%, transparent)'
      : isSearchMatch
        ? 'color-mix(in srgb, var(--accent) 5%, transparent)'
        : '';
  $: fileBorder = isSearchActive ? '2px solid var(--accent)' : '2px solid transparent';

  $: hlIndent = 8 + Math.max(0, depth - 1) * 18;

  $: dirDiffColor = node.dirDiffType
    ? ({ added: 'var(--green)', removed: 'var(--red)', modified: 'var(--yellow)' } as Record<string, string>)[node.dirDiffType] || ''
    : '';
  $: dirBg = isSearchActive
    ? 'color-mix(in srgb, var(--accent) 14%, transparent)'
    : isSearchMatch
      ? 'color-mix(in srgb, var(--accent) 5%, transparent)'
      : '';
  $: dirBorder = isSearchActive ? '2px solid var(--accent)' : '2px solid transparent';

  $: children = node.children ? (Object.values(node.children) as FileNode[]).sort((a, b) => {
    if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
    return a.name.localeCompare(b.name);
  }) : [];

  function handleClick() {
    if (node.type !== 'dir' && !node.children) {
      if (otherId && diffType === 'modified') {
        onShowFileDiff(node.full_path || node.path, otherId);
      } else if (otherId && diffType === 'added') {
        onViewFileFromId(node.full_path || node.path, otherId);
      } else if (otherId && diffType === 'removed') {
        onViewFileFromId(node.full_path || node.path, currentSnapId);
      } else {
        onViewFile(node.full_path || node.path);
      }
    } else {
      forceNoAnim = false;
      fromBulk = false;
      localExpanded = !localExpanded;
    }
  }
</script>

  {#if node.name === '/' && depth === 0}
  {#each children as child (child.path || child.name)}
    <FileTreeNode node={child} depth={depth + 1} {diffMap} {otherId} {currentSnapId}
      {onViewFile} {onViewFileFromId} {onShowFileDiff} {expanded} {expandKey} {activePath} {searchResults} {searchActivePath} {searchAncestorPaths} />
  {/each}
    {:else if node.type === 'dir' || node.children}
<button type="button" class="tree-row" class:highlighted={!!dirBg} style="cursor:pointer;padding:2px 4px;border-radius:4px;font-size:0.9rem;display:flex;align-items:center;gap:4px;color:{dirDiffColor || 'var(--text)'};white-space:nowrap;position:relative;

--hl-indent:{hlIndent}px;"
    on:click={handleClick}
    aria-expanded={localExpanded ? 'true' : 'false'}>
    {#if dirBg}
    <div style="position:absolute;inset:0;margin-left:-{hlIndent}px;width:calc(100% + {hlIndent}px);background:{dirBg};border:{dirBorder};border-radius:4px;pointer-events:none;"></div>
    {/if}
    <div style="width:22px; display:flex; justify-content:center; align-items:center; flex-shrink:0;">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" 
        style={chevronStyle}>
        <path d="M7 10l5 5 5-5H7z"/>
      </svg>
    </div>
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;opacity:0.7;">
      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
    </svg>
    {node.name}
  </button>
  {#if localExpanded || everExpanded}
  <div
    class="slide-grid"
    class:expanded={localExpanded}
    class:no-anim={noAnim}
    style="margin-left:18px"
  >
    <div class="slide-inner">
      {#each children as child (child.path || child.name)}
        <FileTreeNode node={child} depth={depth + 1} {diffMap} {otherId} {currentSnapId}
          {onViewFile} {onViewFileFromId} {onShowFileDiff} {expanded} {expandKey} {activePath} {searchResults} {searchActivePath} {searchAncestorPaths} />
      {/each}
    </div>
  </div>
  {/if}
{:else}
  <button type="button" data-tree-path={node.full_path || node.path} class="tree-row" class:highlighted={!!fileBg} style="cursor:pointer;padding:2px 4px;border-radius:4px;font-size:0.9rem;display:flex;align-items:center;gap:4px;color:{fileColor};font-weight:{fileWeight};white-space:nowrap;position:relative;

--hl-indent:{hlIndent}px;"
    on:click={handleClick}>
    {#if fileBg}
    <div style="position:absolute;inset:0;margin-left:-{hlIndent}px;width:calc(100% + {hlIndent}px);background:{fileBg};border:{fileBorder};border-radius:4px;pointer-events:none;"></div>
    {/if}
    <div style="width:16px; flex-shrink:0;"></div>
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;opacity:0.7;">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>
    </svg>
    <span>{node.name}</span>
    {#if node.size != null}
      <span style="color:var(--muted);font-size:0.75rem;margin-left:auto;">{formatBytes(node.size)}</span>
    {/if}

  </button>
{/if}

<style>
  .slide-grid {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows 0.3s ease, clip-path 0.3s ease;
    contain: layout style;
    transform: translateZ(0);
    clip-path: inset(0 -9999px 100% -9999px);
  }

  .slide-grid.expanded {
    grid-template-rows: 1fr;
    clip-path: inset(0 -9999px 0 -9999px);
  }

  .slide-grid.no-anim {
    transition: none !important;
  }

  .slide-inner {
    min-height: 0;
  }

  .tree-row {
    background: none;
    border: none;
    font: inherit;
    text-align: inherit;
    color: inherit;
  }

  .tree-row::before {
    content: '';
    position: absolute;
    inset: 0;
    margin-left: calc(-1 * var(--hl-indent, 0px));
    width: calc(100% + var(--hl-indent, 0px));
    border-radius: 4px;
    pointer-events: none;
    background: rgb(255 255 255 / 6%);
    opacity: 0;
  }

  .tree-row:hover::before {
    opacity: 1;
  }

  .tree-row.highlighted::before {
    opacity: 0;
  }

  .tree-row.highlighted:hover::before {
    opacity: 1;
  }
</style>
