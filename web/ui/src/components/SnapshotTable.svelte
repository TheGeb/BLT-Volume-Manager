<script lang="ts">
  import type { Snapshot } from '../lib/types';

  export let snapshots: Snapshot[] = [];
  export let sizes: Record<string, string> = {};
  export let selectedVolume = '';
  export let sortNewestFirst = true;
  export let typeFilter = 'all';
  export let hostFilter = '';
  export let hosts: string[] = [];
  export let loading = false;
  export let restorePointLoading: Record<string, boolean> = {};
  export let sizeLoading: Record<string, boolean> = {};
  export let onToggleSort: () => void = () => {};
  export let onTypeFilter: (t: string) => void = () => {};
  export let onHostFilter: (h: string) => void = () => {};
  export let onOpenViewer: (sn: Snapshot) => void = () => {};
  export let onAddTag: (id: string, tag: string, vol: string) => void = () => {};
  export let onRemoveTag: (id: string, tag: string, vol: string) => void = () => {};
  export let onSizeLoaded: (id: string) => void = () => {};
  export let restorePointID = '';
  export let selectedForDeletion: Set<string> = new Set();
  export let onToggleDeletion: (sn: Snapshot) => void = () => {};
  export let onDeleteSelected: () => void = () => {};

  let openFilter: 'type' | 'host' | null = null;

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
    const isRP = sn.id === restorePointID || sn.short_id === restorePointID;
    isRP ? onRemoveTag(sn.id, 'restore-point', selectedVolume) : onAddTag(sn.id, 'restore-point', selectedVolume);
  }
</script>

<svelte:window on:keydown={handleFilterKeydown} on:click={handleDocClick} />

<section class="panel table-panel" style="margin-bottom:0;">
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
              <button aria-label="Filter by type" class="filter-btn" class:active={openFilter === 'type' || typeFilter !== 'all'} on:click|stopPropagation={() => toggleFilter('type')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                </svg>
              </button>
              {#if openFilter === 'type'}
                <!-- svelte-ignore a11y-no-static-element-interactions a11y-click-events-have-key-events -->
                <div class="filter-dropdown" on:click|stopPropagation>
                  {#each ['all', 'hot', 'cold'] as opt (opt)}
                    <button class="filter-opt" class:selected={typeFilter === opt}
                      on:click={() => { onTypeFilter(opt); openFilter = null; }}>
                      {opt === 'all' ? 'All' : opt.charAt(0).toUpperCase() + opt.slice(1)}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </th>
          <th style="text-align:center;width:100px;">Size</th>
          <th>
            <div class="filter-wrap">
              <span class="th-label">Host</span>
              <button aria-label="Filter by host" class="filter-btn" class:active={openFilter === 'host' || hostFilter !== ''} on:click|stopPropagation={() => toggleFilter('host')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                </svg>
              </button>
              {#if openFilter === 'host'}
                <!-- svelte-ignore a11y-no-static-element-interactions a11y-click-events-have-key-events -->
                <div class="filter-dropdown" on:click|stopPropagation>
                  <button class="filter-opt" class:selected={hostFilter === ''}
                    on:click={() => { onHostFilter(''); openFilter = null; }}>All</button>
                  {#each hosts as h (h)}
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
          <tr class:del-row={selectedForDeletion.has(sn.id)}>
             <td style="text-align:center">
               {#if restorePointLoading[sn.id]}
                 <svg width="20" height="20" viewBox="0 0 20 20" class="spin" style="vertical-align:middle;">
                   <circle cx="10" cy="10" r="8" fill="none" stroke-width="2" stroke="var(--accent)" stroke-opacity="0.3"/>
                   <path d="M10 2a8 8 0 0 1 8 8" stroke="var(--accent)" stroke-width="2" fill="none" stroke-linecap="round"/>
                 </svg>
               {:else}
                  <button type="button" class="rp-btn" on:click|stopPropagation={() => handleRPClick(sn)} disabled={restorePointLoading[sn.id]}>
                    <svg width="20" height="20" viewBox="0 0 20 20" style="vertical-align:middle;">
                      <circle cx="10" cy="10" r="8" fill="none" stroke-width="2"
                        stroke={(sn.id === restorePointID || sn.short_id === restorePointID) ? 'var(--accent)' : 'var(--border)'} />
                      {#if sn.id === restorePointID || sn.short_id === restorePointID}
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
               {#each ['hot', 'cold'] as t (t)}
                 {#if sn.tags.includes(t)}
                   <span class="type-badge">{t}</span>
                   {#if t === 'cold' && sn.tags.includes('hot') || t === 'hot' && sn.tags.includes('cold')}, {/if}
                 {/if}
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
                 <button
                   class="button button-xs del-toggle"
                   class:del-selected={selectedForDeletion.has(sn.id)}
                   on:click={() => onToggleDeletion(sn)}>
                   {selectedForDeletion.has(sn.id) ? '×' : 'Delete'}
                 </button>
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
      <tfoot>
        <tr>
          <td class="snap-total">{snapshots.length} snapshot{snapshots.length !== 1 ? 's' : ''}</td>
          <td></td>
          <td></td>
          <td></td>
          <td></td>
          <td class="bulk-count">
            <span class="slide-wrap">
              <span class="slide-inner" class:bulk-hidden={selectedForDeletion.size === 0}>{selectedForDeletion.size || 1} selected</span>
            </span>
          </td>
          <td>
            <span class="slide-wrap">
              <span class="slide-inner" class:bulk-hidden={selectedForDeletion.size === 0}>
                <button class="button del-confirm-btn" on:click={() => onDeleteSelected()}>
                  Delete selected
                </button>
              </span>
            </span>
          </td>
        </tr>
      </tfoot>
    </table>
  </div>
</section>

<style>
  .data-table {
    width: 100%;
    border-collapse: collapse;
    min-width: 550px;
  }

  .data-table th,
  .data-table td {
    padding: 16px 18px;
    text-align: left;
    border-bottom: 1px solid var(--border);
  }

  .data-table th {
    color: var(--muted);
    font-size: 0.95rem;
    letter-spacing: 0.01em;
    white-space: nowrap;
  }

  .data-table tbody tr {
    background: var(--surface);
    transition: background 0.15s ease;
  }

  .data-table tbody tr:hover {
    background: rgb(255 255 255 / 4%);
  }

  .copy-id {
    cursor: pointer;
    font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace;
    transition: color 0.15s;
  }

  .copy-id:hover {
    color: var(--accent);
  }

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
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
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
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    margin-top: 6px; text-align: center;
  }

  .del-toggle {
    background: none;
    border: 1px solid var(--red);
    color: var(--red);
    cursor: pointer;
    border-radius: 6px;
    padding: 2px 8px;
    font-size: 0.8rem;
    font-weight: 600;
    transition: background 0.15s, color 0.15s;
    width: 56px;
    text-align: center;
    display: inline-flex;
    justify-content: center;
    align-items: center;
  }

  .del-toggle:hover {
    background: rgb(255 80 80 / 15%);
  }

  .del-toggle.del-selected {
    background: var(--red);
    color: #fff;
    border-color: var(--red);
    font-weight: 700;
  }

  .del-toggle.del-selected:hover {
    background: color-mix(in srgb, var(--red) 80%, #000);
  }

  .data-table tbody tr.del-row {
    background: rgb(255 80 80 / 6%);
    outline: 1px solid rgb(255 80 80 / 15%);
    outline-offset: -1px;
  }

  .data-table tbody tr.del-row:hover {
    background: rgb(255 80 80 / 14%);
  }

  .data-table tfoot td {
    padding: 10px 18px;
    background: var(--surface);
    border-bottom: none;
  }

  .data-table tfoot td:first-child {
    border-radius: 0 0 0 24px;
  }

  .data-table tfoot td:last-child {
    border-radius: 0 0 24px;
  }

  .data-table tfoot .snap-total {
    font-size: 0.85rem;
    color: var(--muted);
    text-align: center;
  }

  .slide-wrap {
    overflow: hidden;
    display: inline-block;
    vertical-align: middle;
  }

  .slide-inner {
    display: inline-block;
    transition: transform 0.3s ease, opacity 0.3s ease;
  }

  .bulk-hidden {
    transform: translateY(-100%);
    opacity: 0;
    pointer-events: none;
  }

  .bulk-count {
    font-size: 0.85rem;
    color: var(--muted);
  }

  .del-confirm-btn {
    background: var(--red);
    color: #fff;
    border: none;
    padding: 8px 14px;
    border-radius: 8px;
    cursor: pointer;
    font-size: 0.8rem;
    font-weight: 600;
  }

  .del-confirm-btn:hover {
    background: color-mix(in srgb, var(--red) 80%, #000);
  }

  .type-badge {
    background: color-mix(in srgb, var(--muted) 20%, transparent);
    color: var(--muted);
    font-size: 0.75rem;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 4px;
    text-transform: capitalize;
  }

  @media (width <= 900px) {
    .data-table {
      min-width: 0;
    }
  }
</style>
