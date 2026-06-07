<script lang="ts">
  import { Button, Popover, ScrollArea } from 'bits-ui';
  import type { Snapshot } from '$lib/types';
  import DateRangeFilter from './DateTimeRange.svelte';
  import Spinner from '../../components/Spinner.svelte';
  import DropSelect from '../../components/DropSelect.svelte';
  import { versionFilterClearKey, tableVersionFilterActive } from '$lib/stores/snapshots';
  import { versionTag, parseVersion } from '$lib/util';

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
  let tableVersionFrom = '';
  let tableVersionTo = '';

  let vfMajor = ''; let vfMinor = '';
  let vtMajor = ''; let vtMinor = '';
  let versionOpen = false;

  function handleVersionOpenChange(o: boolean) {
    versionOpen = o;
  }

  $: versionChanged = (() => {
    const from = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const to = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    return from !== tableVersionFrom || to !== tableVersionTo;
  })();
  $: versionInvalid = (() => {
    const from = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const to = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    if (!from || !to) return false;
    const fp = parseVersion(from);
    const tp = parseVersion(to);
    if (!fp || !tp) return false;
    return tp.major < fp.major || (tp.major === fp.major && tp.minor < fp.minor);
  })();

  let _vcKey = 0;
  $: if (_vcKey !== $versionFilterClearKey) {
    _vcKey = $versionFilterClearKey;
    clearVersionFilter();
  }

  $: tableVersionFilterActive.set(!!(tableVersionFrom || tableVersionTo));

  $: versionFilteredSnapshots = tableVersionFrom || tableVersionTo
    ? snapshots.filter(sn => {
        const vt = sn.tags.find(t => /^v\d+\.\d+$/.test(t));
        if (!vt) return false;
        const sv = parseVersion(vt);
        if (!sv) return false;
        if (tableVersionFrom) {
          const fv = parseVersion(tableVersionFrom);
          if (fv && (sv.major < fv.major || (sv.major === fv.major && sv.minor < fv.minor))) return false;
        }
        if (tableVersionTo) {
          const tv = parseVersion(tableVersionTo);
          if (tv && (sv.major > tv.major || (sv.major === tv.major && sv.minor > tv.minor))) return false;
        }
        return true;
      })
    : snapshots;

  function applyVersionFilter() {
    if (versionInvalid) return;
    const from = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const to = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    const changed = from !== tableVersionFrom || to !== tableVersionTo;
    tableVersionFrom = from;
    tableVersionTo = to;
    if (changed) {
      versionOpen = false;
    }
  }

  function clearVersionFilter() {
    vfMajor = ''; vfMinor = '';
    vtMajor = ''; vtMinor = '';
    tableVersionFrom = '';
    tableVersionTo = '';
  }

  function cleanDigits(v: string): string {
    return v.replace(/[^0-9]/g, '');
  }
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
  export let page = 1;
  export let pageSize = 25;
  export let hasMore = false;
  export let onGoToPage: (page: number) => void = () => {};
  export let onSetPageSize: (size: number) => void = () => {};

  let pendingPage: string | undefined = undefined;
  $: pageDisplay = pendingPage ?? String(page);

  function onPageInput(e: Event) {
    pendingPage = (e.target as HTMLInputElement).value;
  }

  function commitPageInput() {
    if (pendingPage !== undefined) {
      const n = parseInt(pendingPage, 10);
      if (!isNaN(n) && n >= 1) {
        onGoToPage(n);
      }
      pendingPage = undefined;
    }
  }

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
          <th>
            <div class="filter-wrap">
              <span class="th-label">Version</span>
              <Popover.Root bind:open={versionOpen} onOpenChange={handleVersionOpenChange}>
                <Popover.Trigger class={"filter-btn" + (tableVersionFrom || tableVersionTo ? ' active' : '')}>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
                  </svg>
                </Popover.Trigger>
                <Popover.Content class="filter-dropdown">
                  <div class="version-range-filter">
                    <div class="version-range-section">
                      <span class="filter-label">from</span>
                      <div class="version-input-group">
                        <span class="version-prefix">v</span>
                        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vfMajor} on:input={() => { vfMajor = cleanDigits(vfMajor); }} size={vfMajor.length || 1}>
                        <span class="version-dot">.</span>
                        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vfMinor} on:input={() => { vfMinor = cleanDigits(vfMinor); }} size={vfMinor.length || 1}>
                      </div>
                    </div>
                    <div class="version-range-section">
                      <span class="filter-label">to</span>
                      <div class="version-input-group">
                        <span class="version-prefix">v</span>
                        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vtMajor} on:input={() => { vtMajor = cleanDigits(vtMajor); }} size={vtMajor.length || 1}>
                        <span class="version-dot">.</span>
                        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vtMinor} on:input={() => { vtMinor = cleanDigits(vtMinor); }} size={vtMinor.length || 1}>
                      </div>
                    </div>
                    <div class="filter-actions">
                      <button class="apply-btn" class:apply-btn-active={versionChanged && !versionInvalid} class:apply-btn-invalid={versionInvalid} on:click={applyVersionFilter}>Apply</button>
                      <button class="clear-btn" class:clear-btn-active={!!(vfMajor || vfMinor || vtMajor || vtMinor)} on:click={clearVersionFilter}>Clear</button>
                    </div>
                  </div>
                </Popover.Content>
              </Popover.Root>
            </div>
          </th>
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

    <ScrollArea.Root type="always" style="height:calc(100vh - 460px);overflow:hidden;">
      <ScrollArea.Viewport style="height:100%;width:100%;">
        <div class="body-scroll-inner">
        <table class="body-table">
          {@render columns()}
          <tbody style="opacity:{loading ? 0.4 : 1};transition:opacity 0.15s ease;">
            {#each versionFilteredSnapshots as sn (sn.id)}
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
                 <td class="copy-id" title="{sn.id}"
                  on:click={() => { navigator.clipboard.writeText(sn.id); }}>
                  {versionTag(sn.tags) ?? 'v_._?'}
                </td>
                 <td style="color:var(--muted);font-size:0.9rem;">
                   {#each ['hot', 'cold'] as t (t)}
                     {#if sn.tags.includes(t)}
                       <span class="type-badge">{t}</span>
                     {/if}
                   {:else}—
                   {/each}
                 </td>
                <td style="text-align:center;font-variant-numeric:tabular-nums;white-space:nowrap;">
                  {#if sizes[sn.id]}
                    {sizes[sn.id]}
                    {:else if sizeLoading[sn.id]}
                      <Spinner size={16} />
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
          </tbody>
        </table>
        </div>
      </ScrollArea.Viewport>
      <ScrollArea.Scrollbar style="display:flex;background:transparent;width:8px;padding:2px;" orientation="vertical">
        <ScrollArea.Thumb style="flex:1;background:color-mix(in srgb, var(--muted) 30%, transparent);border-radius:4px;min-height:40px;" />
      </ScrollArea.Scrollbar>
    </ScrollArea.Root>

    <table class="footer-table">
      {@render columns()}
      <tfoot>
        <tr>
          <td class="snap-total" colspan="3">
            <div class="pagination">
              <button class="page-btn" disabled={page <= 1} on:click={() => onGoToPage(1)} title="First page">&laquo;</button>
              <button class="page-btn" disabled={page <= 1} on:click={() => onGoToPage(page - 1)} title="Previous page">&lsaquo;</button>
              <input
                class="page-input"
                type="text"
                inputmode="numeric"
                value={pageDisplay}
                on:input={onPageInput}
                on:keydown={(e) => e.key === 'Enter' && commitPageInput()}
                on:blur={commitPageInput}
              />
              <button class="page-btn" disabled={!hasMore} on:click={() => onGoToPage(page + 1)} title="Next page">&rsaquo;</button>
              <button class="page-btn" disabled={!hasMore} on:click={() => onGoToPage(-1)} title="Last page">&raquo;</button>
              <DropSelect
                options={[
                  { value: '10', label: '10' },
                  { value: '25', label: '25' },
                  { value: '50', label: '50' },
                  { value: '100', label: '100' },
                ]}
                value={String(pageSize)}
                onValueChange={(v) => onSetPageSize(Number(v))}
              />
              <span class="page-size-label">per page</span>
            </div>
          </td>
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
    width: calc(100% - 12px);
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

  .body-scroll-inner {
    padding-right: 12px;
  }

  .body-table td {
    padding: 16px 18px;
    text-align: left;
    border-bottom: 1px solid var(--border);
  }

  .footer-table {
    width: calc(100% - 12px);
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
    outline: none;
    transition: color 0.15s, background 0.15s;
  }

  :global(.filter-btn.active) {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    color: var(--accent);
  }

  :global(.filter-btn:hover) {
    background: var(--hover-bg);
    color: var(--text);
  }

  :global(.filter-btn[data-state="open"]) {
    background: var(--hover-bg);
    color: var(--text);
  }

  :global(.filter-btn:active) {
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

  .pagination {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .page-btn {
    background: none;
    border: 1px solid transparent;
    color: var(--muted);
    border-radius: 6px;
    padding: 4px 10px;
    font-size: 0.9rem;
    font-family: inherit;
    cursor: pointer;
    transition: color 0.15s, background 0.15s, border-color 0.15s;
    line-height: 1;
  }

  .page-btn:hover:not(:disabled) {
    color: var(--text);
    background: var(--hover-bg);
    border-color: var(--border);
  }

  .page-btn:disabled {
    opacity: 0.25;
    cursor: default;
  }

  .page-input {
    width: 44px;
    text-align: center;
    background: var(--surface-strong);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 6px;
    padding: 4px 2px;
    font-size: 0.85rem;
    font-family: inherit;
    font-variant-numeric: tabular-nums;
    outline: none;
    transition: border-color 0.15s;
    margin: 0 4px;
  }

  .page-input:hover {
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  .page-input:focus {
    border-color: var(--muted);
  }

  .page-size-label {
    font-size: 0.8rem;
    color: var(--muted);
    margin-left: 2px;
  }

  @media (width <= 900px) {
    .header-table,
    .body-table,
    .footer-table {
      min-width: 0;
    }
  }

  .version-range-filter {
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .version-range-section {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .filter-label {
    font-size: 0.65rem;
    color: var(--muted);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .version-input-group {
    display: inline-flex;
    align-items: center;
    gap: 1px;
    padding: 5px 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface-strong);
    transition: background 0.15s, border-color 0.15s;
  }

  .version-input-group:hover {
    background: var(--hover-bg);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  .version-input-group:focus-within {
    background: var(--hover-bg);
    border-color: var(--muted);
  }

  .version-prefix,
  .version-dot {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.85rem;
    color: var(--muted);
    padding: 0 1px;
    user-select: none;
  }

  .version-segment {
    padding: 1px 2px;
    border-radius: 3px;
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--text);
    white-space: pre;
  }

  .version-segment:hover {
    background: var(--hover-bg);
  }

  .version-segment:focus {
    background: var(--hover-bg);
    color: var(--text);
  }

  .version-num {
    width: auto;
    min-width: 20px;
    max-width: 80px;
    background: transparent;
    border: none;
    color: inherit;
    font: inherit;
    font-weight: 400;
    text-align: center;
    padding: 0 1px;
  }

  .version-num::placeholder {
    color: var(--muted);
  }

  .version-num:focus {
    outline: none;
    background: var(--hover-bg);
    border-radius: 3px;
  }

  .version-num:focus::placeholder {
    opacity: 0;
  }

  .version-range-filter .filter-actions {
    display: flex;
    gap: 6px;
    margin-top: 2px;
  }

  .version-range-filter .apply-btn {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 12px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s;
  }

  .version-range-filter .apply-btn-active {
    background: var(--accent);
    color: #fff;
    border-color: transparent;
  }

  .version-range-filter .apply-btn:hover {
    background: var(--hover-bg);
  }

  .version-range-filter .apply-btn-active:hover {
    background: color-mix(in srgb, var(--accent) 80%, #000);
  }

  .version-range-filter .apply-btn-invalid {
    background: var(--hover-bg);
    color: var(--muted);
    border-color: var(--border);
    cursor: not-allowed;
    opacity: 0.5;
  }

  .version-range-filter .clear-btn {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 10px;
    font-size: 0.75rem;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }

  .version-range-filter .clear-btn:hover {
    background: var(--hover-bg);
  }

  .version-range-filter .clear-btn-active {
    background: var(--red-bg);
    color: var(--red);
    border-color: var(--red);
  }

  .version-range-filter .clear-btn-active:hover {
    background: rgb(248 113 113 / 20%);
  }
</style>
