<script lang="ts">
  let { value = $bindable(''), fullPath = $bindable(false), placeholder = 'Filter...', fill = false, onEnter }: {
    value?: string;
    fullPath?: boolean;
    placeholder?: string;
    fill?: boolean;
    onEnter?: () => void;
  } = $props();
</script>

<input type="search" bind:value {placeholder} class="search-input" class:fill
  onkeydown={(e) => { if (e.key === 'Enter') onEnter?.(); }} />
<button class="button button-secondary button-xs mode-toggle"
  style="padding:7px;line-height:1;color:var(--accent);background:color-mix(in srgb, var(--accent) 12%, transparent);"
  data-tip={fullPath ? 'Full path search (on)' : 'Full path search (off)'}
  onclick={() => fullPath = !fullPath}>
  {#if fullPath}
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round">
      <line x1="17" y1="4" x2="7" y2="20"/>
    </svg>
  {:else}
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
    </svg>
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
