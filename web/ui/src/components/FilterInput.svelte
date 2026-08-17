<script lang="ts">
  let { value = $bindable(''), fullPath = $bindable(false), placeholder = 'Filter...', fill = false, onEnter, databaseIcon = false }: {
    value?: string;
    fullPath?: boolean;
    placeholder?: string;
    fill?: boolean;
    onEnter?: () => void;
    databaseIcon?: boolean;
  } = $props();
</script>

<input type="search" bind:value {placeholder} class="search-input" class:fill
  onkeydown={(e) => { if (e.key === 'Enter') onEnter?.(); }} />
<button class="button button-secondary button-xs mode-toggle"
  style="padding:7px;line-height:1;color:var(--accent);background:color-mix(in srgb, var(--accent) 12%, transparent);"
  data-tip={fullPath ? 'Full path search (on)' : 'Full path search (off)'}
  onclick={() => fullPath = !fullPath}>
  {#if fullPath}
    <span class="mask-icon" aria-hidden="true" style="mask: url('/material/manage_search.svg') no-repeat center / contain; width:18px;height:18px;"></span>
  {:else}
    <span class="mask-icon" aria-hidden="true" style="mask: url('{databaseIcon ? '/material/database_search.svg' : '/material/document_search.svg'}') no-repeat center / contain; width:18px;height:18px;"></span>
  {/if}
</button>

<style>
  .search-input {
    padding: 7px 8px; border-radius: 8px; border: 1px solid var(--border);
    background: var(--surface-strong); color: var(--text); font-size: 0.8rem; font-family: inherit;
    outline: none; width: 150px;
  }
  .search-input.fill { flex: 1; width: auto; min-width: 0; }
  .search-input:hover { border-color: color-mix(in srgb, var(--muted), var(--bg) 40%); }
  .search-input:focus { border-color: var(--muted); }
  .search-input::placeholder { color: var(--muted); }

  .mode-toggle { position: relative; }

  .mode-toggle:hover::after {
    content: attr(data-tip);
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    background: var(--surface-strong);
    color: var(--text);
    padding: 6px 10px;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 400;
    white-space: nowrap;
    z-index: 10;
    pointer-events: none;
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    margin-bottom: 6px;
  }
</style>
