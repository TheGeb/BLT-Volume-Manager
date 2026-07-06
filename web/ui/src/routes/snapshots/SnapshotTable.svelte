<script lang="ts">
  import { Button, ScrollArea } from 'bits-ui';
  import type { Snapshot } from '$lib/types';
  import Spinner from '../../components/Spinner.svelte';
  import DropSelect from '../../components/DropSelect.svelte';
  import { versionTag } from '$lib/util';
  import { showToast } from '$lib/stores/toast';

  export let snapshots: Snapshot[] = [];
  export let sizes: Record<string, string> = {};
  export let selectedVolume = '';
  export let loading = false;
  export let restorePointLoading: Record<string, boolean> = {};
  export let sizeLoading: Record<string, boolean> = {};
  export let onOpenViewer: (sn: Snapshot) => void = () => {};
  export let onSetRestorePoint: (id: string, vol: string) => void = () => {};
  export let onDeleteRestorePoint: (id: string, vol: string) => void = () => {};
  export let onSizeLoaded: (id: string) => void = () => {};
  export let restorePointID = '';
  export let selectedForDeletion: Set<string> = new Set();
  export let onToggleDeletion: (sn: Snapshot) => void = () => {};
  export let onDeleteSelected: () => void = () => {};
  export let page = 1;
  export let pageSize = 25;
  export let hasMore = false;
  export let onGoToPage: (page: number) => void = () => {};
  export let onSetPageSize: (size: number) => void = () => {};
  export let sortNewestFirst = true;
  export let onToggleSort: () => void = () => {};

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
    isRP ? onDeleteRestorePoint(sn.id, selectedVolume) : onSetRestorePoint(sn.id, selectedVolume);
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

<div class="table-scroll-wrapper">
    <table class="header-table">
      {@render columns()}
      <thead>
        <tr>
          <th style="text-align:center;white-space:nowrap;">
            Restore Point
            <span class="restore-point-info" data-tip="Each snapshot can optionally be set as the restore point by clicking its radio button. Click an active restore point to unset it. Only one snapshot can be the restore point at a time."></span>
          </th>
          <th>Version</th>
          <th>Type</th>
          <th style="text-align:center;">Size</th>
          <th>Host</th>
          <th>
            <button type="button" class="sort-btn" onclick={onToggleSort}>
              Date
              <svg width="14" height="8" viewBox="0 0 16 10" fill="currentColor" class="sort-chevron" class:sort-desc={sortNewestFirst}>
                <path d="M3 2l5 6 5-6H3z"/>
              </svg>
            </button>
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
                      <button type="button" class="rp-btn" title="Toggle restore point" onclick={(e) => { e.stopPropagation(); handleRPClick(sn); }} disabled={restorePointLoading[sn.id]}>
                        <svg width="20" height="20" viewBox="0 0 20 20" style="vertical-align:middle;">
                          <circle cx="10" cy="10" r="8" fill="none" stroke-width="2"
                            stroke={(sn.id === restorePointID || sn.short_id === restorePointID) ? 'var(--accent)' : 'var(--muted)'} />
                          {#if sn.id === restorePointID || sn.short_id === restorePointID}
                            <circle cx="10" cy="10" r="5" fill="var(--accent)" />
                          {/if}
                        </svg>
                      </button>
                   {/if}
                 </td>
                  <td class="copy-id" title="Restic ID: {sn.id}"
                                      onclick={() => { navigator.clipboard.writeText(sn.id); showToast('Snapshot ID copied'); }}>
                   <span class="copy-id-inner">{versionTag(sn.tags) ?? 'v_._?'}<span class="copy-hint">{sn.short_id}</span></span>
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
                       onclick={() => onSizeLoaded(sn.id)}>
                       <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                         <rect x="4" y="2" width="16" height="20" rx="2"/>
                         <line x1="8" y1="6" x2="16" y2="6"/>
                         <line x1="8" y1="11" x2="10" y2="11"/>
                         <line x1="14" y1="11" x2="16" y2="11"/>
                         <line x1="8" y1="15" x2="10" y2="15"/>
                         <line x1="14" y1="15" x2="16" y2="15"/>
                         <line x1="8" y1="19" x2="10" y2="19"/>
                         <line x1="14" y1="19" x2="16" y2="19"/>
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
                       <Button.Root class="button button-secondary button-xs" onclick={() => onOpenViewer(sn)}>View/Diff</Button.Root>
                     <button
                       class="del-toggle"
                       class:del-selected={selectedForDeletion.has(sn.id)}
                        onclick={() => onToggleDeletion(sn)}>
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
              <button class="page-btn" disabled={page <= 1} onclick={() => onGoToPage(1)} title="First page">&laquo;</button>
              <button class="page-btn" disabled={page <= 1} onclick={() => onGoToPage(page - 1)} title="Previous page">&lsaquo;</button>
              <input
                class="page-input"
                type="text"
                inputmode="numeric"
                value={pageDisplay}
                oninput={onPageInput}
                onkeydown={(e) => e.key === 'Enter' && commitPageInput()}
                onblur={commitPageInput}
              />
              <button class="page-btn" disabled={!hasMore} onclick={() => onGoToPage(page + 1)} title="Next page">&rsaquo;</button>
              <button class="page-btn" disabled={!hasMore} onclick={() => onGoToPage(-1)} title="Last page">&raquo;</button>
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

  .sort-btn {
    font-size: 0.95rem;
    font-weight: 700;
    letter-spacing: 0.01em;
    color: var(--muted);
    background: none;
    border: none;
    cursor: pointer;
    font-family: inherit;
    padding: 0;
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    appearance: none;
  }

  .sort-btn:hover {
    color: var(--text);
  }

  .sort-chevron {
    opacity: 0.8;
    transition: transform 0.15s;
  }

  .sort-desc {
    transform: rotate(180deg);
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

  .body-table tbody tr {
    transition: background 0.15s;
  }

  .body-table tbody tr:hover {
    background: color-mix(in srgb, var(--muted) 8%, transparent);
  }

  .copy-id {
    cursor: pointer;
    font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace;
    transition: color 0.15s;
  }

  .copy-id-inner {
    position: relative;
  }

  .copy-hint {
    position: absolute;
    top: 100%;
    left: 0;
    font-size: 0.65rem;
    color: var(--muted);
    opacity: 0;
    pointer-events: none;
    white-space: nowrap;
    transition: opacity 0.35s;
    letter-spacing: 0.02em;
  }

  .body-table tbody tr:hover .copy-id {
    color: var(--accent);
  }

  .body-table tbody tr:hover .copy-hint {
    opacity: 0.55;
  }

  .rp-btn {
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
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border-radius: 5px;
    opacity: 0.45;
    color: var(--muted);
    line-height: 0;
    transition: opacity 0.15s, background 0.15s, color 0.15s;
  }

  .body-table tbody tr:hover .size-btn {
    opacity: 1;
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    color: var(--accent);
  }

  .restore-point-info {
    display: inline-flex; align-items: center; justify-content: center;
    width: 22px; height: 22px;
    cursor: help;
    position: relative; top: 5px;
  }

  .restore-point-info::before {
    content: '';
    width: 22px; height: 22px;
    background-color: currentcolor;
    mask: url('/info-circle.svg') no-repeat center / contain;
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
</style>
