<script lang="ts">
  import { onMount } from 'svelte';
  import { Button } from 'bits-ui';
  import { hostFilter, typeFilter, reloadWithFilters, allHosts, loadHosts } from '$lib/stores/snapshots';
  import { selectedVolume } from '$lib/stores/volumes';
  import DropSelect from '../../components/DropSelect.svelte';

  let host = $hostFilter;
  let tags: string[] = $typeFilter !== 'all' ? [$typeFilter] : [];

  onMount(() => {
    if (!$allHosts || $allHosts.length === 0) {
      loadHosts($selectedVolume);
    }
  });

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
  <Button.Root class="search-btn" onclick={search}>Search</Button.Root>
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
</style>
