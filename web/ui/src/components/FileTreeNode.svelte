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

  $: diffType = diffMap?.get(node.full_path ?? '') ?? diffMap?.get((node.path ?? '').replace(/^\//, '')) ?? diffMap?.get(node.name ?? '') ?? '';
  $: diffColor = !diffType ? ''
    : diffType === 'added' ? 'var(--green)'
    : diffType === 'removed' ? 'var(--red)'
    : diffType === 'modified' ? 'var(--yellow)' : '';

  $: children = node.children ? Object.values(node.children).sort((a: any, b: any) => {
    if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
    return a.name.localeCompare(b.name);
  }) : [];

  function handleClick() {
    if (node.type !== 'dir' && !node.children) {
      if (otherId && diffType === 'modified') {
        onShowFileDiff(node.full_path || node.path, otherId);
      } else if (otherId && diffType === 'added') {
        onViewFileFromId(node.full_path || node.path, currentSnapId);
      } else if (otherId && diffType === 'removed') {
        onViewFileFromId(node.full_path || node.path, otherId);
      } else {
        onViewFile(node.full_path || node.path);
      }
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
  {#each children as child}
    <FileTreeNode node={child} depth={depth + 1} {diffMap} {otherId} {currentSnapId}
      {onViewFile} {onViewFileFromId} {onShowFileDiff} />
  {/each}
{:else if node.type === 'dir' || node.children}
  <details open={depth <= 1}>
    <summary style="cursor:pointer;padding:2px 4px;border-radius:4px;color:var(--text);font-size:0.9rem;">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;opacity:0.7;">
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
      </svg>
      {node.name}
    </summary>
    <div style="margin-left:18px">
      {#each children as child}
        <FileTreeNode node={child} depth={depth + 1} {diffMap} {otherId} {currentSnapId}
          {onViewFile} {onViewFileFromId} {onShowFileDiff} />
      {/each}
    </div>
  </details>
{:else}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div style="cursor:pointer;padding:2px 4px;border-radius:4px;font-size:0.9rem;display:flex;align-items:center;gap:4px;color:{diffColor}"
    on:click={handleClick}
    on:mouseenter={handleMouseEnter}
    on:mouseleave={handleMouseLeave}>
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
          on:click|preventDefault|stopPropagation={() => onViewFileFromId(node.full_path || node.path, diffType === 'added' ? currentSnapId : otherId)}>
          view {diffType === 'added' ? 'new' : 'old'}
        </a>
      {/if}
    {/if}
  </div>
{/if}
