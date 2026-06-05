<script lang="ts">
  import { Select } from 'bits-ui';

  type Option = { value: string; label: string };

  export let options: Option[] = [];
  export let value: string = '';
  export let onValueChange: (v: string) => void = () => {};

  $: selectedLabel = options.find(o => o.value === value)?.label ?? options[0]?.label ?? '';
</script>

<Select.Root type="single" value={value} onValueChange={onValueChange}>
  <Select.Trigger class="drop-select-trigger">
    <Select.Value placeholder={selectedLabel}>{selectedLabel}</Select.Value>
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="opacity:0.5;flex-shrink:0;">
      <polyline points="6 9 12 15 18 9"/>
    </svg>
  </Select.Trigger>
  <Select.Portal>
    <Select.Content class="drop-select-content">
      {#each options as opt (opt.value)}
        <Select.Item class="drop-select-item" value={opt.value} label={opt.label}>
          {opt.label}
        </Select.Item>
      {/each}
    </Select.Content>
  </Select.Portal>
</Select.Root>

<style>
  :global(.drop-select-trigger) {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    padding: 8px 28px 8px 12px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: color-mix(in srgb, var(--muted) 12%, var(--surface));
    color: var(--text);
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    outline: none;
    transition: border-color 0.15s;
    min-width: 0;
    white-space: nowrap;
  }

  :global(.drop-select-trigger:hover) {
    border-color: var(--muted);
  }

  :global(.drop-select-trigger:focus) {
    border-color: var(--accent);
  }

  :global(.drop-select-trigger[data-state="open"]) {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 20%, transparent);
  }

  :global(.drop-select-content) {
    background: color-mix(in srgb, var(--muted) 12%, var(--surface));
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 4px;
    min-width: var(--bits-select-anchor-width);
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    z-index: 50;
  }

  :global(.drop-select-item) {
    display: block;
    text-align: left;
    padding: 6px 10px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text);
    font-size: 0.85rem;
    cursor: pointer;
    white-space: nowrap;
  }

  :global(.drop-select-item:hover),
  :global(.drop-select-item[data-highlighted]) {
    background: var(--hover-bg);
  }

  :global(.drop-select-item[data-state="checked"]) {
    color: var(--accent);
    font-weight: 600;
  }
</style>
