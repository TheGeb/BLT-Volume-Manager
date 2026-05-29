<script lang="ts">
  import type { FileNode, DiffResult } from '../lib/types';
  import { formatBytes } from '../lib/util';
  import FileTreeNode from './FileTreeNode.svelte';

  export let node: any;
  export let depth = 0;
  export let diffMap: Map<string, string> | null = null;
  export let otherId = '';
  export let currentSnapId = '';
  export let onViewFile: (path: string) => void = () => {};
  export let onViewFileFromId: (path: string, id: string) => void = () => {};
  export let onShowFileDiff: (path: string, otherId: string) => void = () => {};
  export let expanded = true;
  export let expandKey = 0;

  let localExpanded = expanded;
  let fromBulk = true;
  let everExpanded = expanded;

  $: if (localExpanded) everExpanded = true;

  $: noAnim = fromBulk && depth > 1;

  $: {
    expanded;
    expandKey;
    localExpanded = expanded;
    fromBulk = true;
  }

  $: chevronStyle = `transform:rotate(${localExpanded ? 0 : -90}deg);opacity:0.5;${fromBulk ? '' : 'transition:transform 0.15s;'}`;

  $: diffType = diffMap?.get(node.full_path ?? '') ?? diffMap?.get((node.path ?? '').replace(/^\//, '')) ?? diffMap?.get(node.name ?? '') ?? '';
  $: diffColor = !diffType ? ''
    : diffType === 'added' ? 'var(--green)'
    : diffType === 'removed' ? 'var(--red)'
    : diffType === 'modified' ? 'var(--yellow)' : '';

  $: dirDiffColor = node.dirDiffType
    ? ({ added: 'var(--green)', removed: 'var(--red)', modified: 'var(--yellow)' } as Record<string, string>)[node.dirDiffType] || ''
    : '';

  $: children = node.children ? Object.values(node.children).sort((a: any, b: any) => {
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
      fromBulk = false;
      localExpanded = !localExpanded;
    }
  }

  function handleMouseEnter(e: MouseEvent) {
    (e.currentTarget as HTMLElement).style.background = 'rgba(255,255,255,0.06)';
  }

  function handleMouseLeave(e: MouseEvent) {
    (e.currentTarget as HTMLElement).style.background = '';
  }
</script>

  {#if node.name === '/' && depth === 0}
  {#each children as child (child.path || child.name)}
    <FileTreeNode node={child} depth={depth + 1} {diffMap} {otherId} {currentSnapId}
      {onViewFile} {onViewFileFromId} {onShowFileDiff} {expanded} {expandKey} />
  {/each}
  {:else if node.type === 'dir' || node.children}
<div role="button" tabindex="0" style="cursor:pointer;padding:2px 4px;border-radius:4px;font-size:0.9rem;display:flex;align-items:center;gap:4px;color:{dirDiffColor || 'var(--text)'};white-space:nowrap;"
    on:click={handleClick}
    on:keydown={(e) => e.key === 'Enter' && handleClick()}
    on:mouseenter={handleMouseEnter}
    on:mouseleave={handleMouseLeave}>
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
  </div>
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
          {onViewFile} {onViewFileFromId} {onShowFileDiff} {expanded} {expandKey} />
      {/each}
    </div>
  </div>
  {/if}
{:else}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div role="button" tabindex="0" style="cursor:pointer;padding:2px 4px;border-radius:4px;font-size:0.9rem;display:flex;align-items:center;gap:4px;color:{diffColor || 'var(--text)'};white-space:nowrap;"
    on:click={handleClick}
    on:keydown={(e) => e.key === 'Enter' && handleClick()}
    on:mouseenter={handleMouseEnter}
    on:mouseleave={handleMouseLeave}>
    <div style="width:16px; flex-shrink:0;"></div>
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;opacity:0.7;">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>
    </svg>
    <span>{node.name}</span>
    {#if node.size != null}
      <span style="color:var(--muted);font-size:0.75rem;margin-left:auto;">{formatBytes(node.size)}</span>
    {/if}
    {#if diffType && otherId}
      {#if diffType === 'modified'}
        <!-- svelte-ignore a11y-invalid-attribute -->
        <a href="#" style="color:var(--accent);font-size:0.75rem;text-decoration:none;margin-left:auto;"
          on:click|preventDefault|stopPropagation={() => onShowFileDiff(node.full_path || node.path, otherId)}>view diff</a>
      {:else}
        <!-- svelte-ignore a11y-invalid-attribute -->
        <a href="#" style="color:var(--accent);font-size:0.75rem;text-decoration:none;margin-left:auto;"
          on:click|preventDefault|stopPropagation={() => onViewFileFromId(node.full_path || node.path, diffType === 'added' ? otherId : currentSnapId)}>
          view {diffType === 'added' ? 'new' : 'old'}
        </a>
      {/if}
    {/if}
  </div>
{/if}

<style>
  .slide-grid {
    display: grid;
    grid-template-rows: 0fr;
    overflow: hidden;
    transition: grid-template-rows 0.3s ease;
    contain: layout style paint;
    transform: translateZ(0);
  }
  .slide-grid.expanded {
    grid-template-rows: 1fr;
  }
  .slide-grid.no-anim {
    transition: none !important;
  }
  .slide-inner {
    min-height: 0;
    overflow: hidden;
    contain: layout style paint;
  }
</style>
