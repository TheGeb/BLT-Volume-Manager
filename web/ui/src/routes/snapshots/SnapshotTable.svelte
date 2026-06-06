<script lang="ts">
  import { Button, Popover, ScrollArea } from 'bits-ui';
  import type { Snapshot } from '$lib/types';
  import DateRangeFilter from './DateTimeRange.svelte';

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
  export let timeFrom: number | undefined = undefined;
  export let timeTo: number | undefined = undefined;
  export let onTimeFilter: (from?: number, to?: number) => void = () => {};
  export let timeOfDayFrom: number | undefined = undefined;
  export let timeOfDayTo: number | undefined = undefined;
  export let onTimeOfDayFilter: (from?: number, to?: number) => void = () => {};
  export let totalSnapshots = 0;
  export let allLoaded = false;
  export let onLoadAll: () => void = () => {};

  function handleRPClick(sn: Snapshot) {
    const isRP = sn.id === restorePointID || sn.short_id === restorePointID;
    isRP ? onRemoveTag(sn.id, 'restore-point', selectedVolume) : onAddTag(sn.id, 'restore-point', selectedVolume);
  }
</script>

{#snippet columns()}
  <colgroup>
    <col style="width:18%;">
    <col style="width:14%;">
    <col style="width:8%;">
    <col style="width:10%;">
    <col style="width:14%;">
    <col style="width:12%;">
    <col style="width:24%;">
  </colgroup>
{/snippet}

<section class="panel table-panel" style="margin-bottom:0;">
  <div class="table-scroll-wrapper">
    <table class="header-table">
      {@render columns()}
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
              <Popover.Root>
                <Popover.Trigger class={"filter-btn" + (typeFilter !== 'all' ? ' active' : '')}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                  </svg>
                </Popover.Trigger>
                <Popover.Content class="filter-dropdown">
                  {#each ['all', 'hot', 'cold'] as opt (opt)}
                    <Popover.Close class="filter-opt {typeFilter === opt ? 'selected' : ''}" onclick={() => onTypeFilter(opt)}>
                      {opt === 'all' ? 'All' : opt.charAt(0).toUpperCase() + opt.slice(1)}
                    </Popover.Close>
                  {/each}
                </Popover.Content>
              </Popover.Root>
            </div>
          </th>
          <th style="text-align:center;">Size</th>
          <th>
            <div class="filter-wrap">
              <span class="th-label">Host</span>
              <Popover.Root>
                <Popover.Trigger class={"filter-btn" + (hostFilter !== '' ? ' active' : '')}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                  </svg>
                </Popover.Trigger>
                <Popover.Content class="filter-dropdown">
                  <Popover.Close class="filter-opt {hostFilter === '' ? 'selected' : ''}" onclick={() => onHostFilter('')}>All</Popover.Close>
                  {#each hosts as h (h)}
                    <Popover.Close class="filter-opt {hostFilter === h ? 'selected' : ''}" onclick={() => onHostFilter(h)}>{h}</Popover.Close>
                  {/each}
                </Popover.Content>
              </Popover.Root>
            </div>
          </th>
          <th>
            <DateRangeFilter
              {timeFrom}
              {timeTo}
              {sortNewestFirst}
              {onToggleSort}
              {onTimeFilter}
              {timeOfDayFrom}
              {timeOfDayTo}
              {onTimeOfDayFilter}
            />
          </th>
          <th>Actions</th>
        </tr>
      </thead>
    </table>

    <ScrollArea.Root type="always" style="height:calc(100vh - 390px);overflow:hidden;">
      <ScrollArea.Viewport style="height:100%;width:100%;">
        <table class="body-table">
          {@render columns()}
          <tbody style="opacity:{loading ? 0.4 : 1};transition:opacity 0.15s ease;">
            {#each snapshots as sn (sn.id)}
              <tr class:del-row={selectedForDeletion.has(sn.id)}>
                 <td style="text-align:center">
                   {#if restorePointLoading[sn.id]}
                      <span class="rp-loader">
                        <svg width="20" height="20" viewBox="0 0 20 20" class="spin">
                          <circle cx="10" cy="10" r="8" fill="none" stroke-width="2" stroke="var(--accent)" stroke-opacity="0.3"/>
                          <path d="M10 2a8 8 0 0 1 8 8" stroke="var(--accent)" stroke-width="2" fill="none" stroke-linecap="round"/>
                        </svg>
                      </span>
                   {:else}
                      <button type="button" class="rp-btn" title="Toggle restore point" on:click|stopPropagation={() => handleRPClick(sn)} disabled={restorePointLoading[sn.id]}>
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
                      <Button.Root class="button button-secondary button-xs" onclick={() => onOpenViewer(sn)}>View</Button.Root>
                     <button
                       class="del-toggle"
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
            {#if !allLoaded && totalSnapshots > snapshots.length && snapshots.length > 0}
              <tr>
                <td colspan="7" style="text-align:center;padding:8px;">
                  <button class="load-all-btn" on:click={onLoadAll}>
                    Load all ({totalSnapshots} total)
                  </button>
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      </ScrollArea.Viewport>
      <ScrollArea.Scrollbar style="display:flex;background:transparent;width:8px;padding:2px;" orientation="vertical">
        <ScrollArea.Thumb style="flex:1;background:color-mix(in srgb, var(--muted) 30%, transparent);border-radius:4px;min-height:40px;" />
      </ScrollArea.Scrollbar>
    </ScrollArea.Root>

    <table class="footer-table">
      {@render columns()}
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
                <Button.Root class="del-confirm-btn" onclick={() => onDeleteSelected()}>
                  Delete selected
                </Button.Root>
              </span>
            </span>
          </td>
        </tr>
      </tfoot>
    </table>
  </div>
</section>

<style>
  .header-table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    min-width: 550px;
    border-bottom: 1px solid var(--border);
  }

  .header-table th {
    padding: 16px 18px;
    text-align: left;
    color: var(--muted);
    font-size: 0.95rem;
    letter-spacing: 0.01em;
    white-space: nowrap;
  }

  .body-table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    min-width: 550px;
  }

  .body-table td {
    padding: 16px 18px;
    text-align: left;
    border-bottom: 1px solid var(--border);
  }

  .footer-table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    min-width: 550px;
  }

  .footer-table td {
    padding: 10px 18px;
    background: var(--surface);
    border-bottom: none;
  }

  .footer-table td:first-child {
    border-radius: 0 0 0 24px;
  }

  .footer-table td:last-child {
    border-radius: 0 0 24px;
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

  :global(.filter-btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 26px;
    border-radius: 6px;
    border: none;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    padding: 0;
    line-height: 0;
    transition: color 0.15s, background 0.15s;
  }

  :global(.filter-btn.active) {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
  }

  :global(.filter-btn:hover),
  :global(.filter-btn[data-state="open"]) {
    background: var(--hover-bg);
    color: var(--text);
  }

  :global(.filter-dropdown) {
    z-index: 20;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 4px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    min-width: 100px;
  }

  :global(.filter-opt) {
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

  :global(.filter-opt:hover) {
    background: var(--hover-bg);
  }

  :global(.filter-opt.selected) {
    color: var(--accent);
    font-weight: 600;
  }

  .rp-btn, .size-btn {
    background: none; border: none; padding: 0; cursor: pointer;
    display: inline-flex; align-items: center; justify-content: center;
    line-height: 0;
  }

  .rp-loader {
    display: inline-flex; align-items: center; justify-content: center;
    width: 20px; height: 20px; line-height: 0;
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

  .restore-point-info:hover::after {
    content: attr(data-tip);
    position: absolute; top: 100%; left: 50%; transform: translateX(-50%);
    background: var(--surface-strong); color: var(--text);
    padding: 6px 10px; border-radius: 6px; font-size: 0.75rem; font-weight: 400;
    white-space: normal; width: 260px; z-index: 10; pointer-events: none;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    margin-top: 6px; text-align: center;
  }

  .spin {
    animation: spin 1s linear infinite;
    vertical-align: middle;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .del-toggle {
    background: rgb(255 80 80 / 10%);
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

  .body-table tbody tr.del-row {
    background: rgb(255 80 80 / 6%);
    outline: 1px solid rgb(255 80 80 / 15%);
    outline-offset: -1px;
  }

  .body-table tbody tr.del-row:hover {
    background: rgb(255 80 80 / 14%);
  }

  .footer-table tfoot .snap-total {
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

  :global(.del-confirm-btn) {
    background: var(--red);
    color: #fff;
    border: 1px solid transparent;
    padding: 8px 14px;
    border-radius: 8px;
    cursor: pointer;
    font-size: 0.8rem;
    font-weight: 600;
    transition: background 0.15s, border-color 0.15s;
  }

  :global(.del-confirm-btn:hover) {
    background: color-mix(in srgb, var(--red) 70%, #fff);
    border-color: var(--red);
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
    .header-table,
    .body-table,
    .footer-table {
      min-width: 0;
    }
  }
</style>
