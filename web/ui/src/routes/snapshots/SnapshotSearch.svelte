<script lang="ts">
  import { Button } from 'bits-ui';
  import { hostFilter, typeFilter, searchLatest, reloadWithFilters, allHosts, loadHosts } from '$lib/stores/snapshots';
  import { selectedVolume } from '$lib/stores/volumes';

  let latest = '25';
  let host = $hostFilter;
  let tags: string[] = $typeFilter !== 'all' ? [$typeFilter] : [];
  let hostsLoading = false;
  let hostsLoaded = false;

  async function ensureHosts() {
    if (hostsLoaded || hostsLoading) return;
    hostsLoading = true;
    await loadHosts($selectedVolume);
    hostsLoaded = true;
    hostsLoading = false;
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
    hostFilter.set(host);
    typeFilter.set(tags.length > 0 ? tags[0]! : 'all');
    searchLatest.set(parseInt(latest, 10) || 25);
    reloadWithFilters({ latest: parseInt(latest, 10) || 25 });
  }
</script>

<div class="search-bar">
  <div class="search-field">
    <span class="search-label">Host</span>
    <select class="search-input" bind:value={host} on:mousedown={ensureHosts}>
      <option value="">Any host</option>
      {#if hostsLoading}
        <option disabled>Loading hosts...</option>
      {/if}
      {#each $allHosts as h (h)}
        <option value={h}>{h}</option>
      {/each}
    </select>
  </div>
  <div class="search-field">
    <span class="search-label">Type</span>
    <div class="tag-toggles">
      <button
        class="tag-btn"
        class:tag-active={tags.includes('hot')}
        on:click={() => toggleTag('hot')}
      >Hot</button>
      <button
        class="tag-btn"
        class:tag-active={tags.includes('cold')}
        on:click={() => toggleTag('cold')}
      >Cold</button>
    </div>
  </div>
  <div class="search-field search-field-sm">
    <span class="search-label">Latest</span>
    <input
      type="number"
      class="search-input search-input-num"
      min="1"
      bind:value={latest}
      on:keydown={(e) => e.key === 'Enter' && search()}
    />
  </div>
  <Button.Root class="search-btn" onclick={search}>Search</Button.Root>
</div>

<style>
  .search-bar {
    display: flex;
    align-items: flex-end;
    gap: 12px;
    flex-wrap: wrap;
    padding: 0 0 16px;
  }

  .search-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .search-field-sm {
    max-width: 80px;
  }

  .search-label {
    font-size: 0.7rem;
    color: var(--muted);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .search-input {
    border: 1px solid var(--border);
    background: var(--surface-strong);
    color: var(--text);
    padding: 8px 12px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-family: inherit;
    outline: none;
    transition: border-color 0.15s;
    width: 160px;
  }

  .search-input:focus {
    border-color: var(--accent);
  }

  .search-input-num {
    width: 70px;
  }

  .tag-toggles {
    display: flex;
    gap: 4px;
  }

  .tag-btn {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-strong);
    color: var(--muted);
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
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

  :global(.search-btn) {
    padding: 8px 18px;
    border-radius: 8px;
    font-size: 0.85rem;
    font-weight: 600;
    height: auto;
    background: linear-gradient(135deg, var(--accent), var(--accent-soft));
    color: #fff;
    border: 1px solid transparent;
    margin-left: 12px;
  }

  :global(.search-btn:hover) {
    border-color: var(--accent);
    background: linear-gradient(135deg, color-mix(in srgb, var(--accent) 70%, #fff), color-mix(in srgb, var(--accent-soft) 70%, #fff));
  }
</style>
