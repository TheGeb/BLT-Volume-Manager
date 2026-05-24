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
      {#each volumes as vol}
        <button class="volume-pill" class:active={vol === selectedVolume} on:click={() => onSelect(vol)}>
          {vol}
        </button>
      {/each}
    </div>
  </div>
  {#if pillsCachedAt}
    <div class="pills-cached">Cached: {pillsCachedAt}</div>
  {/if}
</section>
