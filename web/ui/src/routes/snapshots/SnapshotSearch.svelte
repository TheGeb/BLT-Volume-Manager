<script lang="ts">
  import { onMount } from 'svelte';
  import { Button, Popover } from 'bits-ui';
  import {
    hostFilter, typeFilter, versionFrom, versionTo, reloadWithFilters, allHosts, loadHosts,
    timeFrom, timeTo, timeOfDayFrom, timeOfDayTo, versionFilterClearKey, tableVersionFilterActive,
  } from '$lib/stores/snapshots';

  import { selectedVolume } from '$lib/stores/volumes';
  import { parseVersion } from '$lib/util';
  import DropSelect from '../../components/DropSelect.svelte';
  import DateTimeRange from './DateTimeRange.svelte';
  import VersionRangeInputs from '../../components/VersionRangeInputs.svelte';

  let host = $hostFilter;
  let tags: string[] = $typeFilter !== 'all' ? [$typeFilter] : [];
  let committedVfMajor = ''; let committedVfMinor = '';
  let committedVtMajor = ''; let committedVtMinor = '';
  let dateOpen = false;
  let versionOpen = false;
  let localTimeFrom: number | undefined = undefined;
  let localTimeTo: number | undefined = undefined;
  let localTimeOfDayFrom: number | undefined = undefined;
  let localTimeOfDayTo: number | undefined = undefined;
  let versionInputs: VersionRangeInputs;

  $: dateLabel = computeDateLabel(localTimeFrom, localTimeTo, localTimeOfDayFrom, localTimeOfDayTo);

  function computeDateLabel(
    tf: number | undefined, tt: number | undefined,
    tdf: number | undefined, tdt: number | undefined,
  ): string {
    if (tdf !== undefined || tdt !== undefined) {
      return fmtSOT(tdf) + '\u2013' + fmtSOT(tdt);
    }
    if (tf !== undefined || tt !== undefined) {
      const from = fmtTS(tf);
      const to = fmtTS(tt);
      if (tf && tt && sameUTCDay(tf, tt)) {
        const ft = timePart(tf);
        const ttPart = timePart(tt);
        if (!ft && !ttPart) return from;
        return from.split(' (')[0] + ' (' + (ft || '12 AM') + ' \u2013 ' + (ttPart || '12 AM') + ')';
      }
      return from + ' \u2013 ' + to;
    }
    return 'Any date';
  }

  function sameUTCDay(a: number, b: number): boolean {
    const da = new Date(a);
    const db = new Date(b);
    return da.getUTCFullYear() === db.getUTCFullYear()
      && da.getUTCMonth() === db.getUTCMonth()
      && da.getUTCDate() === db.getUTCDate();
  }

  function timePart(ts: number | undefined): string {
    if (ts === undefined) return '';
    const d = new Date(ts);
    const h = d.getUTCHours();
    const m = d.getUTCMinutes();
    const s = d.getUTCSeconds();
    if (h === 0 && m === 0 && s === 0) return '';
    if (h === 23 && m === 59 && s === 59) return '';
    return formatTime(h, m, s);
  }

  function fmtTS(ts: number | undefined): string {
    if (ts === undefined) return '\u2026';
    const d = new Date(ts);
    const date = String(d.getUTCMonth() + 1) + '/' + String(d.getUTCDate());
    const h = d.getUTCHours();
    const m = d.getUTCMinutes();
    const s = d.getUTCSeconds();
    const isDefault = (h === 0 && m === 0 && s === 0) || (h === 23 && m === 59 && s === 59);
    if (isDefault) return date;
    return date + ' (' + formatTime(h, m, s) + ')';
  }

  function fmtSOT(s: number | undefined): string {
    if (s === undefined) return '--:--';
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    return formatTime(h, m, sec);
  }

  function formatTime(h: number, m: number, s: number): string {
    const ampm = h >= 12 ? 'PM' : 'AM';
    const h12 = h % 12 || 12;
    let result = String(h12);
    if (m !== 0 || s !== 0) result += ':' + String(m).padStart(2, '0');
    if (s !== 0) result += ':' + String(s).padStart(2, '0');
    return result + ' ' + ampm;
  }

  $: dateActive = localTimeFrom !== undefined || localTimeTo !== undefined
    || localTimeOfDayFrom !== undefined || localTimeOfDayTo !== undefined;

  $: versionLabel = computeVersionLabel(committedVfMajor, committedVfMinor, committedVtMajor, committedVtMinor);
  $: versionActive = versionLabel !== 'Any version';

  function computeVersionLabel(fm: string, fn: string, tm: string, tn: string): string {
    const from = fmtVersion(fm, fn);
    const to = fmtVersion(tm, tn);
    const fromEmpty = !from || from === '0';
    const toEmpty = !to || to === '0';
    if (!fromEmpty && !toEmpty) return `v${from} - v${to}`;
    if (!fromEmpty) return `v${from}`;
    if (!toEmpty) return `v${to}`;
    return 'Any version';
  }

  function fmtVersion(major: string, minor: string): string {
    const mNum = parseInt(major || '0', 10);
    const nStr = minor || '0';
    const nNum = parseInt(nStr, 10);
    if (nNum === 0) return String(mNum);
    const trimmed = nStr.replace(/0+$/, '');
    return trimmed ? `${mNum}.${trimmed}` : String(mNum);
  }

  function handleVersionOpenChange(o: boolean) {
    versionOpen = o;
    if (o) versionInputs?.loadFields();
  }

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

  onMount(() => {
    loadVersionFields();
  });

  function onHostOpenChange(open: boolean) {
    if (open && (!$allHosts || $allHosts.length === 0)) {
      loadHosts($selectedVolume);
    }
  }

  function loadVersionFields() {
    const f = parseVersion($versionFrom);
    committedVfMajor = f ? String(f.major) : '';
    committedVfMinor = f ? String(f.minor) : '';
    const t = parseVersion($versionTo);
    committedVtMajor = t ? String(t.major) : '';
    committedVtMinor = t ? String(t.minor) : '';
  }

  function toggleTag(tag: string) {
    if (tags.includes(tag)) {
      tags = tags.filter(t => t !== tag);
    } else {
      tags = tags.filter(t => t !== 'hot' && t !== 'cold');
      tags = [...tags, tag];
    }
  }

  $: hasClearable = host !== '' || tags.length > 0
    || committedVfMajor !== '' || committedVfMinor !== '' || committedVtMajor !== '' || committedVtMinor !== ''
    || localTimeFrom !== undefined || localTimeTo !== undefined
    || localTimeOfDayFrom !== undefined || localTimeOfDayTo !== undefined
    || $hostFilter !== '' || $typeFilter !== 'all'
    || $versionFrom !== '' || $versionTo !== ''
    || $timeFrom !== undefined || $timeTo !== undefined
    || $timeOfDayFrom !== undefined || $timeOfDayTo !== undefined
    || $tableVersionFilterActive;

  function search() {
    if (versionInputs) {
      const vals = versionInputs.getValues();
      commitVersion(vals.from, vals.to);
    }
    hostFilter.set(host);
    typeFilter.set(tags.length > 0 ? tags[0]! : 'all');
    const from = committedVfMajor || committedVfMinor ? `${committedVfMajor || '0'}.${committedVfMinor || '0'}` : '';
    const to = committedVtMajor || committedVtMinor ? `${committedVtMajor || '0'}.${committedVtMinor || '0'}` : '';
    versionFrom.set(from);
    versionTo.set(to);
    const overrides: Record<string, number | undefined> = {};
    if (localTimeFrom !== undefined) overrides.timeFrom = localTimeFrom;
    if (localTimeTo !== undefined) overrides.timeTo = localTimeTo;
    if (localTimeOfDayFrom !== undefined) overrides.timeOfDayFrom = localTimeOfDayFrom;
    if (localTimeOfDayTo !== undefined) overrides.timeOfDayTo = localTimeOfDayTo;
    reloadWithFilters(overrides);
  }

  function clear() {
    host = '';
    tags = [];
    committedVfMajor = ''; committedVfMinor = '';
    committedVtMajor = ''; committedVtMinor = '';
    localTimeFrom = undefined;
    localTimeTo = undefined;
    localTimeOfDayFrom = undefined;
    localTimeOfDayTo = undefined;
    hostFilter.set('');
    typeFilter.set('all');
    versionFrom.set('');
    versionTo.set('');
    timeFrom.set(undefined);
    timeTo.set(undefined);
    timeOfDayFrom.set(undefined);
    timeOfDayTo.set(undefined);
    versionFilterClearKey.update(n => n + 1);
    reloadWithFilters();
  }
</script>

<div class="search-bar">
  <div class="search-field">
    <span class="search-label">Host</span>
    <DropSelect
      options={[
        { value: '', label: 'Any host' },
        ...($allHosts ?? []).map(h => ({ value: h, label: h })),
      ]}
      value={host}
      onValueChange={(v) => host = v}
      onOpenChange={onHostOpenChange}
    />
  </div>
  <div class="search-field">
    <span class="search-label">Type</span>
    <div class="tag-toggles">
      <button
        class="dropdown tag-btn"
        class:tag-active={tags.includes('hot')}
        on:click={() => toggleTag('hot')}
      >Hot</button>
      <button
        class="dropdown tag-btn"
        class:tag-active={tags.includes('cold')}
        on:click={() => toggleTag('cold')}
      >Cold</button>
    </div>
  </div>
  <div class="search-field">
    <span class="search-label">Version</span>
    <Popover.Root bind:open={versionOpen} onOpenChange={handleVersionOpenChange}>
      <Popover.Trigger class="version-trigger {versionActive ? 'version-trigger-active' : ''}">
        {versionLabel}
        <svg class="chevron" width="10" height="6" viewBox="0 0 10 6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1l4 4 4-4"/></svg>
      </Popover.Trigger>
      <Popover.Content class="version-popover">
        <VersionRangeInputs
          bind:this={versionInputs}
          from={committedVfMajor || committedVfMinor ? `${committedVfMajor || '0'}.${committedVfMinor || '0'}` : ''}
          to={committedVtMajor || committedVtMinor ? `${committedVtMajor || '0'}.${committedVtMinor || '0'}` : ''}
          onApply={commitVersion}
          onClear={clearVersionPreview}
        />
      </Popover.Content>
    </Popover.Root>
  </div>
  <div class="search-field">
    <span class="search-label">Date/Time</span>
    <Popover.Root open={dateOpen} onOpenChange={(o) => dateOpen = o}>
      <Popover.Trigger class="date-trigger {dateActive ? 'date-trigger-active' : ''}">
        {dateLabel}
        <svg class="chevron" width="10" height="6" viewBox="0 0 10 6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M1 1l4 4 4-4"/></svg>
      </Popover.Trigger>
      <Popover.Content class="date-popover">
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
  <Button.Root class="search-btn" onclick={search}>Search</Button.Root>
  <Button.Root class="clear-btn {hasClearable ? 'clear-active' : ''}" onclick={hasClearable ? clear : () => {}}>Clear</Button.Root>
</div>

<style>
  .search-bar {
    display: flex;
    align-items: flex-end;
    gap: 12px;
    flex-wrap: wrap;
    padding: 0 0 8px;
  }

  .search-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .search-label {
    font-size: 0.7rem;
    color: var(--muted);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .tag-toggles {
    display: flex;
    gap: 4px;
  }

  .tag-btn {
    font-weight: 500;
  }

  .tag-btn:hover {
    border-color: var(--muted);
    color: var(--text);
  }

  .tag-active {
    background: var(--accent);
    color: #fff;
    border-color: transparent;
  }

  .tag-active:hover {
    background: color-mix(in srgb, var(--accent) 80%, #000);
    color: #fff;
  }

  :global(.version-trigger) {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-strong);
    color: var(--muted);
    font-size: 0.85rem;
    cursor: pointer;
    white-space: nowrap;
    user-select: none;
    font-family: inherit;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  :global(.version-trigger:hover) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  :global(.version-trigger[data-state="open"]) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: var(--muted);
  }

  :global(.version-trigger-active) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: var(--muted);
  }

  :global(.version-trigger-active:hover) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  :global(.date-trigger) {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-strong);
    color: var(--muted);
    font-size: 0.85rem;
    cursor: pointer;
    white-space: nowrap;
    user-select: none;
    font-family: inherit;
    transition: background 0.15s, border-color 0.15s, color 0.15s;
  }

  :global(.date-trigger:hover) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  :global(.date-trigger[data-state="open"]) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: var(--muted);
  }

  :global(.date-trigger-active) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: var(--muted);
  }

  :global(.date-trigger-active:hover) {
    background: var(--hover-bg);
    color: var(--text);
    border-color: var(--muted);
  }

  .chevron {
    flex-shrink: 0;
    opacity: 0.6;
  }

  :global(.version-popover) {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 12px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    z-index: 100;
  }

  :global(.date-popover) {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    z-index: 100;
  }

  :global(.search-btn) {
    padding: 8px 18px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 600;
    height: auto;
    background: linear-gradient(135deg, var(--accent), var(--accent-soft));
    color: #fff;
    border: 1px solid transparent;
    cursor: pointer;
  }

  :global(.search-btn:hover) {
    border-color: var(--accent);
    background: linear-gradient(135deg, color-mix(in srgb, var(--accent) 70%, #fff), color-mix(in srgb, var(--accent-soft) 70%, #fff));
  }

  :global(.clear-btn) {
    background: transparent;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 18px;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: default;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }

  :global(.clear-btn.clear-active) {
    cursor: pointer;
    background: var(--red-bg);
    color: var(--red);
    border-color: var(--red);
  }

  :global(.clear-btn.clear-active:hover) {
    background: rgb(248 113 113 / 20%);
  }
</style>
