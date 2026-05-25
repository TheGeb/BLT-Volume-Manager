<script lang="ts">
  import type { Snapshot } from '../lib/types';

  export let snapshots: Snapshot[] = [];
  export let sizes: Record<string, string> = {};
  export let selectedVolume = '';
  export let sortNewestFirst = true;
  export let query = '';
  export let typeFilter = 'all';
  export let hostFilter = '';
  export let hosts: string[] = [];
  export let loading = false;
  export let rpLoading: Record<string, boolean> = {};
  export let sizeLoading: Record<string, boolean> = {};
  export let onSearch: (q: string) => void = () => {};
  export let onToggleSort: () => void = () => {};
  export let onTypeFilter: (t: string) => void = () => {};
  export let onHostFilter: (h: string) => void = () => {};
  export let onOpenViewer: (sn: Snapshot) => void = () => {};
  export let onAddTag: (id: string, tag: string, vol: string) => void = () => {};
  export let onRemoveTag: (id: string, tag: string, vol: string) => void = () => {};
  export let onDeleteSnapshot: (sn: Snapshot) => void = () => {};
  export let onSizeLoaded: (id: string) => void = () => {};

  let searchVal = '';
  let openFilter: 'type' | 'host' | null = null;

  $: searchVal = query;

  function handleSearch() { onSearch(searchVal); }

  function toggleFilter(f: 'type' | 'host') {
    openFilter = openFilter === f ? null : f;
  }

  function handleFilterKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') openFilter = null;
  }

  function handleDocClick(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('.filter-wrap')) openFilter = null;
  }

  function handleRPClick(sn: Snapshot) {
    const isRP = sn.tags.includes('restore-point');
    isRP ? onRemoveTag(sn.id, 'restore-point', selectedVolume) : onAddTag(sn.id, 'restore-point', selectedVolume);
  }
</script>

<svelte:window on:keydown={handleFilterKeydown} on:click={handleDocClick} />

<section class="panel table-panel" style="margin-bottom:0;">
  <div class="row gap" style="margin-bottom:16px;">
    <input class="input" type="search" placeholder="Filter by name or tag"
      bind:value={searchVal} on:input={handleSearch} />
  </div>
  <div style="overflow-x:auto;">
    <table class="data-table">
      <thead>
        <tr>
          <th style="text-align:center;white-space:nowrap;">
            Restore Point
            <span class="restore-point-info" data-tip="Each snapshot can optionally be set as the restore point by clicking its radio button. Click an active restore point to unset it. Only one snapshot can be the restore point at a time.">i</span>
          </th>
          <th>Snapshot ID</th>
          <th>
            <div class="filter-wrap">
              <span class="th-label">Type</span>
              <button class="filter-btn" class:active={openFilter === 'type' || typeFilter !== 'all'} on:click|stopPropagation={() => toggleFilter('type')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                </svg>
              </button>
              {#if openFilter === 'type'}
                <!-- svelte-ignore a11y-no-static-element-interactions a11y-click-events-have-key-events -->
                <div class="filter-dropdown" on:click|stopPropagation>
                  {#each ['all', 'hot', 'cold'] as opt}
                    <button class="filter-opt" class:selected={typeFilter === opt}
                      on:click={() => { onTypeFilter(opt); openFilter = null; }}>
                      {opt === 'all' ? 'All' : opt}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </th>
          <th style="text-align:center;">Size</th>
          <th>
            <div class="filter-wrap">
              <span class="th-label">Host</span>
              <button class="filter-btn" class:active={openFilter === 'host' || hostFilter !== ''} on:click|stopPropagation={() => toggleFilter('host')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                </svg>
              </button>
              {#if openFilter === 'host'}
                <!-- svelte-ignore a11y-no-static-element-interactions a11y-click-events-have-key-events -->
                <div class="filter-dropdown" on:click|stopPropagation>
                  <button class="filter-opt" class:selected={hostFilter === ''}
                    on:click={() => { onHostFilter(''); openFilter = null; }}>All</button>
                  {#each hosts as h}
                    <button class="filter-opt" class:selected={hostFilter === h}
                      on:click={() => { onHostFilter(h); openFilter = null; }}>{h}</button>
                  {/each}
                </div>
              {/if}
            </div>
          </th>
          <th style="cursor:pointer;user-select:none;white-space:nowrap;" on:click={onToggleSort}>
            Date {sortNewestFirst ? '▼' : '▲'}
          </th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody id="snapshotTable" style="opacity:{loading ? 0.4 : 1};transition:opacity 0.15s ease;">
        {#each snapshots as sn (sn.id)}
          <tr>
             <td style="text-align:center">
               {#if rpLoading[sn.id]}
                 <svg width="20" height="20" viewBox="0 0 20 20" class="spin" style="vertical-align:middle;">
                   <circle cx="10" cy="10" r="8" fill="none" stroke-width="2" stroke="var(--accent)" stroke-opacity="0.3"/>
                   <path d="M10 2a8 8 0 0 1 8 8" stroke="var(--accent)" stroke-width="2" fill="none" stroke-linecap="round"/>
                 </svg>
               {:else}
                 <button type="button" class="rp-btn" on:click|stopPropagation={() => handleRPClick(sn)} disabled={rpLoading[sn.id]}>
                   <svg width="20" height="20" viewBox="0 0 20 20" style="vertical-align:middle;">
                     <circle cx="10" cy="10" r="8" fill="none" stroke-width="2"
                       stroke={sn.tags.includes('restore-point') ? 'var(--accent)' : 'var(--border)'} />
                     {#if sn.tags.includes('restore-point')}
                       <path d="M6 10 l3 3 l5 -5" stroke="var(--accent)" stroke-width="2" fill="none" />
                     {/if}
                   </svg>
                 </button>
               {/if}
             </td>
            <td class="copy-id" title="Click to copy full snapshot ID"
              on:click={() => { navigator.clipboard.writeText(sn.id); }}>
              {sn.short_id.slice(0, 8)}…
            </td>
            <td style="color:var(--muted);font-size:0.9rem;">
              {#each ['hot', 'cold'] as t}
                {#if sn.tags.includes(t)}{t}{#if t === 'cold' && sn.tags.includes('hot') || t === 'hot' && sn.tags.includes('cold')}, {/if}{/if}
              {:else}—
              {/each}
            </td>
            <td style="text-align:center;font-variant-numeric:tabular-nums;white-space:nowrap;">
              {#if sizes[sn.id]}
                {sizes[sn.id]}
               {:else if sizeLoading[sn.id]}
                 <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="3" stroke-linecap="round" class="spin" style="vertical-align:middle;">
                   <path d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10-4.477-10-10-10z" stroke-opacity="0.3"/>
                   <path d="M12 2a10 10 0 0 1 10 10" />
                 </svg>
               {:else}
                <button type="button" class="size-btn" title="Compute size"
                  on:click|stopPropagation={() => onSizeLoaded(sn.id)}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
                    <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
                  </svg>
                </button>
              {/if}
            </td>
            <td style="color:var(--muted);font-size:0.9rem;">{sn.hostname || '—'}</td>
            <td>{new Date(sn.time).toLocaleDateString()}<br>
              <span style="font-size:0.85rem;color:var(--muted);">{new Date(sn.time).toLocaleTimeString()}</span>
            </td>
            <td>
              <div style="display:flex;gap:4px;flex-wrap:nowrap;">
                <button class="button button-secondary button-xs" on:click={() => onOpenViewer(sn)}>View</button>
                <button class="button button-secondary button-xs" on:click={() => onDeleteSnapshot(sn)} style="color:var(--red);">Delete</button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="7" style="text-align:center;color:var(--muted);padding:40px;">
              {loading ? 'Loading...' : 'No snapshots found'}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</section>

<style>
  .filter-wrap {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .th-label {
    white-space: nowrap;
  }
  .filter-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
    line-height: 0;
  }
  .filter-btn:hover, .filter-btn.active {
    background: var(--hover-bg);
    color: var(--text);
  }
  .filter-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 20;
    background: var(--surface-strong);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 4px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    min-width: 100px;
    margin-top: 4px;
  }
  .filter-opt {
    display: block;
    width: 100%;
    text-align: left;
    padding: 4px 10px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: var(--text);
    font-size: 0.85rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .filter-opt:hover {
    background: var(--hover-bg);
  }
  .filter-opt.selected {
    color: var(--accent);
    font-weight: 600;
  }
  .rp-btn, .size-btn {
    background: none; border: none; padding: 0; cursor: pointer;
    display: inline-flex; align-items: center; justify-content: center;
    line-height: 0;
  }
  .rp-btn:disabled { cursor: default; }
  .size-btn {
    width: 22px; height: 22px; border-radius: 4px; opacity: 0.5;
    color: var(--muted);
  }
  .size-btn:hover { opacity: 1; background: var(--hover-bg); }

  .restore-point-info {
    position: relative; display: inline-flex; align-items: center; justify-content: center;
    width: 18px; height: 18px; border-radius: 50%;
    border: 1.5px solid var(--muted); color: var(--muted);
    font-size: 11px; font-weight: 700; cursor: help; font-style: normal;
    margin-left: 6px; vertical-align: middle;
  }
  .spin {
    animation: spin 1s linear infinite;
    vertical-align: middle;
  }
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .restore-point-info:hover::after {
    content: attr(data-tip);
    position: absolute; top: 100%; left: 50%; transform: translateX(-50%);
    background: var(--surface-strong); color: var(--text);
    padding: 6px 10px; border-radius: 6px; font-size: 0.75rem; font-weight: 400;
    white-space: normal; width: 260px; z-index: 10; pointer-events: none;
    box-shadow: 0 4px 12px rgba(0,0,0,0.3);
    margin-top: 6px; text-align: center;
  }
</style>
