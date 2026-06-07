<script lang="ts">
  import { onMount } from 'svelte';
  import { Button } from 'bits-ui';
  import {
    hostFilter, typeFilter, versionFrom, versionTo, reloadWithFilters, allHosts, loadHosts,
    timeFrom, timeTo, timeOfDayFrom, timeOfDayTo,
  } from '$lib/stores/snapshots';

  let searchApplied = false;
  import { selectedVolume } from '$lib/stores/volumes';
  import { parseVersion } from '$lib/util';
  import DropSelect from '../../components/DropSelect.svelte';

  let host = $hostFilter;
  let tags: string[] = $typeFilter !== 'all' ? [$typeFilter] : [];
  let vfMajor = ''; let vfMinor = '';
  let vtMajor = ''; let vtMinor = '';

  onMount(() => {
    loadVersionFields();
    if (!$allHosts || $allHosts.length === 0) {
      loadHosts($selectedVolume);
    }
  });

  function loadVersionFields() {
    const f = parseVersion($versionFrom);
    vfMajor = f ? String(f.major) : '';
    vfMinor = f ? String(f.minor) : '';
    const t = parseVersion($versionTo);
    vtMajor = t ? String(t.major) : '';
    vtMinor = t ? String(t.minor) : '';
  }

  function toggleTag(tag: string) {
    if (tags.includes(tag)) {
      tags = tags.filter(t => t !== tag);
    } else {
      tags = tags.filter(t => t !== 'hot' && t !== 'cold');
      tags = [...tags, tag];
    }
  }

  function search() {
    searchApplied = true;
    hostFilter.set(host);
    typeFilter.set(tags.length > 0 ? tags[0]! : 'all');
    const from = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const to = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    versionFrom.set(from);
    versionTo.set(to);
    reloadWithFilters();
  }

  function clear() {
    searchApplied = false;
    host = '';
    tags = [];
    vfMajor = ''; vfMinor = '';
    vtMajor = ''; vtMinor = '';
    hostFilter.set('');
    typeFilter.set('all');
    versionFrom.set('');
    versionTo.set('');
    timeFrom.set(undefined);
    timeTo.set(undefined);
    timeOfDayFrom.set(undefined);
    timeOfDayTo.set(undefined);
    reloadWithFilters();
  }

  function cleanDigits(v: string): string {
    return v.replace(/[^0-9]/g, '');
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
    <div class="version-range">
      <div class="version-input-group">
        <span class="version-prefix">v</span>
        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vfMajor} on:input={() => { vfMajor = cleanDigits(vfMajor); }} size={vfMajor.length || 1}>
        <span class="version-dot">.</span>
        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vfMinor} on:input={() => { vfMinor = cleanDigits(vfMinor); }} size={vfMinor.length || 1}>
      </div>
      <span class="range-sep">to</span>
      <div class="version-input-group">
        <span class="version-prefix">v</span>
        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vtMajor} on:input={() => { vtMajor = cleanDigits(vtMajor); }} size={vtMajor.length || 1}>
        <span class="version-dot">.</span>
        <input type="text" placeholder="0" class="version-segment version-num" bind:value={vtMinor} on:input={() => { vtMinor = cleanDigits(vtMinor); }} size={vtMinor.length || 1}>
      </div>
    </div>
  </div>
  <Button.Root class="search-btn" onclick={search}>Search</Button.Root>
  <Button.Root class="clear-btn {searchApplied ? 'clear-active' : ''}" onclick={clear}>Clear</Button.Root>
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

  .version-range {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .range-sep {
    font-size: 0.75rem;
    color: var(--muted);
  }

  .version-input-group {
    display: inline-flex;
    align-items: center;
    gap: 1px;
    padding: 5px 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface-strong);
  }

  .version-input-group:focus-within {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgb(124 58 237 / 20%);
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

  :global(.search-btn) {
    padding: 8px 18px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 600;
    height: auto;
    background: linear-gradient(135deg, var(--accent), var(--accent-soft));
    color: #fff;
    border: 1px solid transparent;
  }

  :global(.search-btn:hover) {
    border-color: var(--accent);
    background: linear-gradient(135deg, color-mix(in srgb, var(--accent) 70%, #fff), color-mix(in srgb, var(--accent-soft) 70%, #fff));
  }

  :global(.clear-btn) {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 8px 18px;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }

  :global(.clear-btn:hover) {
    background: var(--hover-bg);
  }

  :global(.clear-btn.clear-active) {
    background: var(--red-bg);
    color: var(--red);
    border-color: var(--red);
  }

  :global(.clear-btn.clear-active:hover) {
    background: rgb(248 113 113 / 20%);
  }
</style>
