<script lang="ts">
  export let volumeFilter = '';
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
  </div>
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

  .volume-filter-input {
    border: 1px solid var(--border);
    background: rgb(255 255 255 / 4%);
    color: var(--text);
    padding: 8px 16px;
    border-radius: 999px;
    font-size: 0.9rem;
    font-weight: 500;
    outline: none;
    width: 160px;
    transition: border-color 0.15s;
  }

  .volume-filter-input:hover {
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  .volume-filter-input:focus {
    border-color: var(--muted);
  }

  .volume-filter-input::placeholder {
    color: var(--muted);
  }
</style>
