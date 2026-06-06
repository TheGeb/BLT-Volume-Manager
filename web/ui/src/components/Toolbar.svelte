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
    <input class="filter-input" type="search" placeholder="Filter volumes..."
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

  .filter-input {
    width: 160px;
    padding: 8px 16px;
  }
</style>
