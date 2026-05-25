<script lang="ts">
  export let volumes: string[] = [];
  export let selectedVolume = '';
  export let volumeFilter = '';
  export let loading = false;
  export let pillsCachedAt = '';
  export let onSelect: (vol: string) => void = () => {};
  export let onFilterChange: (f: string) => void = () => {};

  let filterVal = '';
  $: filterVal = volumeFilter;

  function handleInput() {
    onFilterChange(filterVal);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { filterVal = ''; onFilterChange(''); }
  }
</script>

<section class="panel toolbar-panel">
  <div class="toolbar-row">
    <input class="volume-filter-input" type="search" placeholder="Filter volumes..."
      bind:value={filterVal} on:input={handleInput} on:keydown={handleKeydown} />
    <div class="volume-pills" style="opacity: {loading ? 0.4 : 1}">
      {#if loading && volumes.length === 0}
        {#each { length: 5 } as _}
          <span class="skeleton skeleton-pill"></span>
        {/each}
      {:else}
        {#each volumes as vol}
          <button class="volume-pill" class:active={vol === selectedVolume} on:click={() => onSelect(vol)}>
            {vol}
          </button>
        {/each}
      {/if}
    </div>
  </div>
  {#if pillsCachedAt}
    <div class="pills-cached">Cached: {pillsCachedAt}</div>
  {/if}
</section>

<style>
  .toolbar-panel {
    padding: 16px 24px;
  }

  .toolbar-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .pills-cached {
    font-size: 0.75rem;
    color: var(--muted);
    margin-top: 8px;
  }

  .volume-pills {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    flex: 1;
    min-width: 0;
    max-height: 130px;
    overflow-y: auto;
    transition: opacity 0.15s ease;
  }

  .volume-pill {
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.04);
    color: var(--muted);
    padding: 8px 16px;
    border-radius: 999px;
    cursor: pointer;
    font-size: 0.9rem;
    font-weight: 500;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
    white-space: nowrap;
  }

  .volume-pill:hover {
    background: rgba(255, 255, 255, 0.08);
    color: var(--text);
  }

  .volume-pill.active {
    background: linear-gradient(135deg, var(--accent), var(--accent-soft));
    color: #fff;
    border-color: transparent;
  }

  .volume-filter-input {
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.04);
    color: var(--text);
    padding: 8px 16px;
    border-radius: 999px;
    font-size: 0.9rem;
    font-weight: 500;
    outline: none;
    width: 160px;
    transition: border-color 0.15s;
  }

  .volume-filter-input:focus {
    border-color: var(--accent);
  }

  .volume-filter-input::placeholder {
    color: var(--muted);
  }

  .skeleton-pill {
    display: inline-block;
    height: 38px;
    width: 100px;
    border-radius: 999px;
    border: 1px solid var(--border);
  }
</style>
