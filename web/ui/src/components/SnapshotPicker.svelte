<script lang="ts">
  import { slide } from 'svelte/transition';
  import { tick } from 'svelte';
  import { Popover } from 'bits-ui';
  import type { Snapshot } from '$lib/types';
  import type { SnapshotListParams } from '$lib/api';
  import * as api from '$lib/api';
  import { showToast } from '$lib/stores/toast';
  import { versionTag, parseVersion, safeErrorMessage } from '$lib/util';
  import { computeDateLabel, computeVersionLabel } from '$lib/format';
  import HostDropdown from './HostDropdown.svelte';
  import VersionRangeInputs from './VersionRangeInputs.svelte';
  import DateTimeRange from './DateTimeRange.svelte';

  let {
    mode = 'single' as 'single' | 'multi',
    value = mode === 'single' ? '' : ([] as string | string[]),
    onValueChange = (_v: string | string[]) => {},
    disabledId = '',
    volume = '',
    restorePointID = '',
  } = $props();

  let filtersExpanded = $state(true);

  let searchQuery = $state('');
  let hostFilter = $state('');
  let typeTags = $state<string[]>([]);
  let committedVfMajor = $state(''); let committedVfMinor = $state('');
  let committedVtMajor = $state(''); let committedVtMinor = $state('');
  let localTimeFrom: number | undefined = $state(undefined);
  let localTimeTo: number | undefined = $state(undefined);
  let localTimeOfDayFrom: number | undefined = $state(undefined);
  let localTimeOfDayTo: number | undefined = $state(undefined);
  let dateOpen = $state(false);
  let versionOpen = $state(false);
  let versionInputs = $state<VersionRangeInputs | null>(null);

  let typeFilter = $derived(typeTags[0] ?? 'all');
  let versionFrom = $derived(committedVfMajor || committedVfMinor ? `${committedVfMajor || '0'}.${committedVfMinor || '0'}` : '');
  let versionTo = $derived(committedVtMajor || committedVtMinor ? `${committedVtMajor || '0'}.${committedVtMinor || '0'}` : '');

  let dateLabel = $derived(computeDateLabel(localTimeFrom, localTimeTo, localTimeOfDayFrom, localTimeOfDayTo));
  let dateActive = $derived(localTimeFrom !== undefined || localTimeTo !== undefined || localTimeOfDayFrom !== undefined || localTimeOfDayTo !== undefined);

  let versionLabel = $derived(computeVersionLabel(committedVfMajor, committedVfMinor, committedVtMajor, committedVtMinor));
  let versionActive = $derived(versionLabel !== 'Any version');

  let hasActiveFilters = $derived(!!(searchQuery || hostFilter || typeTags.length > 0 || dateActive || versionActive));

  let searchDebounced = $state('');

  $effect(() => {
    const q = searchQuery;
    const t = setTimeout(() => { searchDebounced = q; }, 300);
    return () => clearTimeout(t);
  });

  let pickerSnapshots: Snapshot[] = $state([]);
  let pickerHasMore = $state(false);
  let pickerLoading = $state(false);
  let pickerRestorePointID = $state('');
  let pickerGeneration = $state(0);
  let pickerController: AbortController | null = $state(null);

  async function fetchPickerData() {
    pickerGeneration++;
    const currentGen = pickerGeneration;

    if (pickerController) pickerController.abort();
    const controller = new AbortController();
    pickerController = controller;
    pickerLoading = true;
    if (!volume) {
      pickerSnapshots = [];
      pickerHasMore = false;
      pickerLoading = false;
      return;
    }
    try {
      const params: SnapshotListParams = {
        offset: (pickerPage - 1) * pickerPageSize,
        limit: pickerPageSize,
      };
      if (hostFilter) params.host = hostFilter;
      if (typeFilter !== 'all') params.tag = typeFilter;
      if (localTimeFrom !== undefined) params.timeFrom = localTimeFrom;
      if (localTimeTo !== undefined) params.timeTo = localTimeTo;
      if (localTimeOfDayFrom !== undefined) params.timeOfDayFrom = localTimeOfDayFrom;
      if (localTimeOfDayTo !== undefined) params.timeOfDayTo = localTimeOfDayTo;
      if (versionFrom) params.versionFrom = versionFrom;
      if (versionTo) params.versionTo = versionTo;
      if (searchDebounced) params.query = searchDebounced;
      const data = await api.fetchSnapshots(volume, params, controller.signal);
      if (currentGen !== pickerGeneration) return;
      pickerSnapshots = data.snapshots;
      pickerHasMore = data.hasMore ?? false;
      pickerRestorePointID = data.restorePointID ?? '';
    } catch (e: unknown) {
      if (e instanceof DOMException && e.name === 'AbortError') return;
      if (currentGen !== pickerGeneration) return;
      pickerSnapshots = [];
      pickerHasMore = false;
      showToast(safeErrorMessage(e), true);
    } finally {
      if (currentGen === pickerGeneration) pickerLoading = false;
    }
  }

  $effect(() => {
    searchDebounced;
    hostFilter;
    typeFilter;
    localTimeFrom; localTimeTo;
    localTimeOfDayFrom; localTimeOfDayTo;
    versionFrom; versionTo;

    pickerPage = 1;
  });

  $effect(() => {
    searchDebounced;
    hostFilter;
    typeFilter;
    localTimeFrom; localTimeTo;
    localTimeOfDayFrom; localTimeOfDayTo;
    versionFrom; versionTo;
    pickerPage;
    pickerPageSize;
    volume;

    if (volume) {
      const timer = setTimeout(() => fetchPickerData(), 0);
      return () => clearTimeout(timer);
    }
  });

  $effect(() => {
    if (versionOpen) {
      tick().then(() => versionInputs?.loadFields());
    }
  });

  function commitVersion(from: string, to: string) {
    const f = parseVersion(from);
    committedVfMajor = f ? String(f.major) : '';
    committedVfMinor = f ? String(f.minor) : '';
    const t = parseVersion(to);
    committedVtMajor = t ? String(t.major) : '';
    committedVtMinor = t ? String(t.minor) : '';
    versionOpen = false;
  }

  function clearVersionPreview() {
    committedVfMajor = ''; committedVfMinor = '';
    committedVtMajor = ''; committedVtMinor = '';
  }

  function handleTimeFilter(from?: number, to?: number) {
    localTimeFrom = from;
    localTimeTo = to;
  }

  function handleTimeOfDayFilter(from?: number, to?: number) {
    localTimeOfDayFrom = from;
    localTimeOfDayTo = to;
  }

  function handleDateClose() {
    dateOpen = false;
  }

  function toggleTag(tag: string) {
    if (typeTags.includes(tag)) {
      typeTags = typeTags.filter(t => t !== tag);
    } else {
      typeTags = typeTags.filter(t => t !== 'hot' && t !== 'cold');
      typeTags = [...typeTags, tag];
    }
  }

  let pickerPage = $state(1);
  let pickerPageSize = $state(10);
  let pendingPickerPage: string | undefined = $state(undefined);
  let pickerPageDisplay = $derived(pendingPickerPage ?? String(pickerPage));
  let lastPageFetching = $state(false);

  function onPickerPageInput(e: Event) {
    pendingPickerPage = (e.target as HTMLInputElement).value;
  }

  function commitPickerPage() {
    if (pendingPickerPage !== undefined) {
      const n = parseInt(pendingPickerPage, 10);
      if (!isNaN(n) && n >= 1) {
        pickerPage = n;
      }
      pendingPickerPage = undefined;
    }
  }

  async function goToLastPickerPage() {
    if (!volume || lastPageFetching) return;
    lastPageFetching = true;
    pickerGeneration++;
    const currentGen = pickerGeneration;
    if (pickerController) pickerController.abort();
    const controller = new AbortController();
    pickerController = controller;
    try {
      const countParams: SnapshotListParams = { offset: 0, limit: 0 };
      if (hostFilter) countParams.host = hostFilter;
      if (typeFilter !== 'all') countParams.tag = typeFilter;
      if (localTimeFrom !== undefined) countParams.timeFrom = localTimeFrom;
      if (localTimeTo !== undefined) countParams.timeTo = localTimeTo;
      if (localTimeOfDayFrom !== undefined) countParams.timeOfDayFrom = localTimeOfDayFrom;
      if (localTimeOfDayTo !== undefined) countParams.timeOfDayTo = localTimeOfDayTo;
      if (versionFrom) countParams.versionFrom = versionFrom;
      if (versionTo) countParams.versionTo = versionTo;
      if (searchDebounced) countParams.query = searchDebounced;
      const r = await api.fetchSnapshots(volume, countParams, controller.signal);
      if (currentGen !== pickerGeneration) return;
      const total = r.snapshots.length;
      const lastPage = Math.max(1, Math.ceil(total / pickerPageSize));
      pickerPage = lastPage;
    } catch (e: unknown) {
      if (currentGen !== pickerGeneration) return;
      if (e instanceof DOMException && e.name === 'AbortError') return;
      showToast(safeErrorMessage(e), true);
    } finally {
      lastPageFetching = false;
    }
  }

  function clearFilters() {
    searchQuery = '';
    hostFilter = '';
    typeTags = [];
    committedVfMajor = ''; committedVfMinor = '';
    committedVtMajor = ''; committedVtMinor = '';
    localTimeFrom = undefined;
    localTimeTo = undefined;
    localTimeOfDayFrom = undefined;
    localTimeOfDayTo = undefined;
  }

  function isSelected(id: string): boolean {
    if (mode === 'single') return value === id;
    return (value as string[]).includes(id);
  }

  function handleSelect(id: string) {
    if (mode === 'single') {
      onValueChange(id);
    } else {
      const arr = value as string[];
      if (arr.includes(id)) {
        onValueChange(arr.filter(v => v !== id));
      } else {
        onValueChange([...arr, id]);
      }
    }
  }
</script>

<div class="snap-picker">
  <div class="picker-search-bar">
    <input
      type="search"
      class="picker-search-input"
      placeholder="Search snapshots..."
      bind:value={searchQuery}
    />
    <button
      class="picker-filter-toggle"
      class:filter-active={hasActiveFilters}
      onclick={() => filtersExpanded = !filtersExpanded}
    >
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <line x1="4" y1="6" x2="20" y2="6"/><line x1="8" y1="12" x2="20" y2="12"/><line x1="12" y1="18" x2="20" y2="18"/>
      </svg>
      Filters
      {#if hasActiveFilters}
        <span class="filter-dot"></span>
      {/if}
    </button>
  </div>

  {#if filtersExpanded}
    <div class="picker-filters">
      <div transition:slide>
          <div class="filter-row">
            <div class="filter-field">
              <span class="filter-label">Host</span>
              <HostDropdown
                value={hostFilter}
                onValueChange={(v: string) => hostFilter = v}
                {volume}
              />
            </div>
            <div class="filter-field">
              <span class="filter-label">Type</span>
              <div class="filter-toggles">
                <button class="filter-tag-btn" class:tag-on={typeTags.includes('hot')} onclick={() => toggleTag('hot')}>Hot</button>
                <button class="filter-tag-btn" class:tag-on={typeTags.includes('cold')} onclick={() => toggleTag('cold')}>Cold</button>
              </div>
            </div>
            <div class="filter-field">
              <span class="filter-label">Version</span>
              <Popover.Root bind:open={versionOpen}>
                <Popover.Trigger class="filter-trigger {versionActive ? 'filter-trigger-active' : ''}">
                  {versionLabel}
                  <svg class="chevron" width="10" height="6" viewBox="0 0 10 6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1l4 4 4-4"/></svg>
                </Popover.Trigger>
                <Popover.Content class="filter-popover">
                  <VersionRangeInputs
                    bind:this={versionInputs}
                    from={versionFrom}
                    to={versionTo}
                    onApply={commitVersion}
                    onClear={clearVersionPreview}
                  />
                </Popover.Content>
              </Popover.Root>
            </div>
            <div class="filter-field">
              <span class="filter-label">Date/Time</span>
              <Popover.Root bind:open={dateOpen}>
                <Popover.Trigger class="filter-trigger {dateActive ? 'filter-trigger-active' : ''}">
                  {dateLabel}
                  <svg class="chevron" width="10" height="6" viewBox="0 0 10 6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1l4 4 4-4"/></svg>
                </Popover.Trigger>
                <Popover.Content class="filter-popover-date">
                  <DateTimeRange
                    triggerless
                    timeFrom={localTimeFrom}
                    timeTo={localTimeTo}
                    timeOfDayFrom={localTimeOfDayFrom}
                    timeOfDayTo={localTimeOfDayTo}
                    onTimeFilter={handleTimeFilter}
                    onTimeOfDayFilter={handleTimeOfDayFilter}
                    onClose={handleDateClose}
                  />
                </Popover.Content>
              </Popover.Root>
            </div>
            {#if hasActiveFilters}
              <button class="picker-clear-filters" onclick={clearFilters}>Clear</button>
            {/if}
          </div>
        </div>
      </div>
  {/if}

  <div class="picker-list" class:empty={pickerSnapshots.length === 0}>
    {#if pickerLoading}
      <div class="picker-empty">Loading...</div>
    {:else if pickerSnapshots.length === 0}
      <div class="picker-empty">No snapshots found</div>
    {:else}
      <table class="picker-table">
        <tbody>
          {#each pickerSnapshots as sn (sn.id)}
            <tr
              class="picker-row"
              class:selected={isSelected(sn.id)}
              class:disabled={sn.id === disabledId}
              class:restore-point={sn.id === (pickerRestorePointID || restorePointID) || sn.short_id === (pickerRestorePointID || restorePointID)}
              class:disabled-click={sn.id === disabledId}
              onclick={() => !(sn.id === disabledId) && handleSelect(sn.id)}
              onkeydown={(e) => e.key === 'Enter' && !(sn.id === disabledId) && handleSelect(sn.id)}
              role="button"
              tabindex={sn.id === disabledId ? -1 : 0}
            >
              {#if mode === 'multi'}
                <td class="picker-cell-checkbox">
                  <input
                    type="checkbox"
                    class="picker-input"
                    checked={isSelected(sn.id)}
                    disabled={sn.id === disabledId}
                    onchange={() => handleSelect(sn.id)}
                    onclick={(e) => e.stopPropagation()}
                    name="snap-picker"
                  />
                </td>
              {/if}
              <td class="picker-cell-version">{versionTag(sn.tags) ?? 'v_._?'}</td>
              <td class="picker-cell-sid" title="ID: {sn.id}">{sn.short_id}</td>
              <td class="picker-cell-type">
                {#if sn.tags.includes('hot')}
                  <span class="picker-badge">Hot</span>
                {:else if sn.tags.includes('cold')}
                  <span class="picker-badge">Cold</span>
                {/if}
                {#if sn.id === restorePointID || sn.short_id === restorePointID}
                  <span class="picker-rp">RP</span>
                {/if}
              </td>
              <td class="picker-cell-date">{new Date(sn.time).toLocaleDateString()}</td>
              <td class="picker-cell-time">{new Date(sn.time).toLocaleTimeString()}</td>
              <td class="picker-cell-host" title={sn.hostname}>{sn.hostname}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
  <div class="picker-footer">
    <div class="picker-pagination">
      <button class="page-btn" disabled={pickerPage <= 1} onclick={() => pickerPage = 1} title="First page">&laquo;</button>
      <button class="page-btn" disabled={pickerPage <= 1} onclick={() => pickerPage--} title="Previous page">&lsaquo;</button>
      <input
        class="page-input"
        type="text"
        inputmode="numeric"
        value={pickerPageDisplay}
        oninput={onPickerPageInput}
        onkeydown={(e) => e.key === 'Enter' && commitPickerPage()}
        onblur={commitPickerPage}
      />
      <button class="page-btn" disabled={!pickerHasMore || lastPageFetching} onclick={() => pickerPage++} title="Next page">&rsaquo;</button>
      <button class="page-btn" disabled={lastPageFetching} onclick={goToLastPickerPage} title="Last page">&raquo;</button>
    </div>
  </div>
</div>

<style>
  .snap-picker {
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--surface);
    overflow: auto;
  }

  .picker-search-bar {
    display: flex;
    gap: 6px;
    padding: 8px;
    border-bottom: 1px solid var(--border);
  }

  .picker-search-input {
    flex: 1;
    padding: 6px 10px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: rgb(255 255 255 / 4%);
    color: var(--text);
    font-size: 0.8rem;
    outline: none;
    min-width: 0;
  }

  .picker-search-input:hover {
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  .picker-search-input:focus {
    border-color: var(--muted);
  }

  .picker-filter-toggle {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 6px 10px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: rgb(255 255 255 / 4%);
    color: var(--muted);
    font-size: 0.78rem;
    cursor: pointer;
    font-family: inherit;
    white-space: nowrap;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
    position: relative;
  }

  .picker-filter-toggle:hover {
    background: var(--hover-bg);
    color: var(--text);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  .picker-filter-toggle.filter-active {
    color: var(--accent);
    border-color: var(--accent);
  }

  .filter-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    position: absolute;
    top: 4px;
    right: 4px;
  }

  .picker-filters {
    border-bottom: 1px solid var(--border);
    overflow: visible;
  }

  .filter-row {
    display: flex;
    gap: 10px;
    padding: 8px;
    flex-wrap: wrap;
    align-items: flex-end;
  }

  .filter-field {
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

  .filter-toggles {
    display: flex;
    gap: 3px;
  }

  .filter-tag-btn {
    padding: 4px 8px;
    font-size: 0.75rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: var(--surface-strong);
    color: var(--muted);
    cursor: pointer;
    font-family: inherit;
    font-weight: 500;
    transition: all 0.12s;
  }

  .filter-tag-btn:hover {
    border-color: var(--muted);
    color: var(--text);
  }

  .filter-tag-btn.tag-on {
    background: var(--accent);
    color: #fff;
    border-color: transparent;
  }

  .filter-tag-btn.tag-on:hover {
    background: color-mix(in srgb, var(--accent) 80%, #000);
  }

  :global(.picker-filters .filter-trigger) {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface-strong);
    color: var(--muted);
    font-size: 0.75rem;
    cursor: pointer;
    white-space: nowrap;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }

  :global(.picker-filters .drop-select-trigger) {
    padding: 4px 8px;
    font-size: 0.75rem;
  }

  :global(.picker-filters .filter-trigger:hover) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  :global(.picker-filters .filter-trigger[data-state="open"]),
  :global(.picker-filters .filter-trigger-active) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: var(--muted);
  }

  :global(.picker-filters .filter-popover) {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 10px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    z-index: 1010;
  }

  :global(.picker-filters .filter-popover-date) {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 12px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    z-index: 1010;
    max-width: 90vw;
  }

  .chevron {
    flex-shrink: 0;
    opacity: 0.5;
  }

  .picker-clear-filters {
    padding: 4px 10px;
    font-size: 0.75rem;
    border-radius: 5px;
    border: 1px solid var(--red);
    background: transparent;
    color: var(--red);
    cursor: pointer;
    font-family: inherit;
    font-weight: 600;
    transition: background 0.12s;
    height: fit-content;
    align-self: flex-end;
  }

  .picker-clear-filters:hover {
    background: rgb(248 113 113 / 15%);
  }

  .picker-list {
    overflow: hidden auto;
    scrollbar-gutter: stable;
    min-height: 280px;
  }

  .picker-list.empty {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 60px;
  }

  .picker-empty {
    color: var(--muted);
    font-size: 0.8rem;
    padding: 20px;
    text-align: center;
  }

  .picker-table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
  }

  .picker-row {
    cursor: pointer;
    transition: background 0.1s;
    user-select: none;
  }

  .picker-row:hover {
    background: rgb(255 255 255 / 4%);
  }

  .picker-row.selected {
    background: color-mix(in srgb, var(--accent) 10%, transparent);
  }

  .picker-row.disabled {
    opacity: 0.4;
  }

  .picker-row.disabled-click {
    cursor: not-allowed;
    pointer-events: none;
  }

  .picker-row.restore-point {
    background: color-mix(in srgb, var(--accent) 10%, var(--surface));
  }

  .picker-row.restore-point:hover {
    background: color-mix(in srgb, var(--accent) 16%, var(--surface));
  }

  .picker-row td {
    padding: 6px 4px;
    font-size: 0.78rem;
    border-bottom: 1px solid var(--border);
    vertical-align: middle;
  }

  .picker-row:last-child td {
    border-bottom: none;
  }

  .picker-cell-checkbox {
    width: 5%;
    text-align: center;
  }

  .picker-input {
    margin: 0;
    accent-color: var(--accent);
    vertical-align: middle;
  }

  .picker-row td.picker-cell-version {
    font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace;
    color: var(--accent);
    width: 14%;
    text-align: left;
    padding-left: 10px;
  }

  .picker-cell-sid {
    font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace;
    color: var(--muted);
    font-size: 0.72rem;
    width: 11%;
  }

  .picker-cell-type {
    text-align: center;
    width: 14%;
  }

  .picker-badge {
    background: color-mix(in srgb, var(--muted) 18%, transparent);
    color: var(--muted);
    font-size: 0.62rem;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 4px;
    text-transform: capitalize;
    white-space: nowrap;
  }

  .picker-rp {
    background: color-mix(in srgb, var(--accent) 18%, var(--surface));
    color: var(--accent);
    font-size: 0.6rem;
    font-weight: 700;
    padding: 1px 4px;
    border-radius: 4px;
    letter-spacing: 0.02em;
    white-space: nowrap;
  }

  .picker-cell-date {
    color: var(--muted);
    width: 15%;
  }

  .picker-cell-time {
    color: var(--muted);
    opacity: 0.7;
    width: 18%;
  }

  .picker-cell-host {
    color: var(--muted);
    text-align: left;
    font-size: 0.75rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .picker-footer {
    border-top: 1px solid var(--border);
    padding: 6px 12px;
    display: flex;
    justify-content: center;
  }

  .picker-pagination {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  :global(.picker-pagination .page-btn) {
    background: none;
    border: 1px solid transparent;
    color: var(--muted);
    border-radius: 6px;
    padding: 4px 8px;
    font-size: 0.8rem;
    font-family: inherit;
    cursor: pointer;
    transition: color 0.15s, background 0.15s, border-color 0.15s;
    line-height: 1;
  }

  :global(.picker-pagination .page-btn:hover:not(:disabled)) {
    color: var(--text);
    background: var(--hover-bg);
    border-color: var(--border);
  }

  :global(.picker-pagination .page-btn:disabled) {
    opacity: 0.25;
    cursor: default;
  }

  :global(.picker-pagination .page-input) {
    width: 44px;
    text-align: center;
    background: var(--surface-strong);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 6px;
    padding: 4px 2px;
    font-size: 0.8rem;
    font-family: inherit;
    font-variant-numeric: tabular-nums;
    outline: none;
    transition: border-color 0.15s;
    margin: 0 4px;
  }

  :global(.picker-pagination .page-input:hover) {
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  :global(.picker-pagination .page-input:focus) {
    border-color: var(--muted);
  }
</style>
