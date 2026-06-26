<script lang="ts">
  import { Select } from 'bits-ui';

  type Option = { value: string; label: string; disabled?: boolean };

  let {
    options = [], value = '', onValueChange = () => {}, onOpenChange = (_open: boolean) => {},
    multiple = false, selected = [] as string[], onSelectedChange = (_vals: string[]) => {},
  }: {
    options: Option[];
    value?: string;
    onValueChange?: (v: string) => void;
    onOpenChange?: (open: boolean) => void;
    multiple?: boolean;
    selected?: string[];
    onSelectedChange?: (vals: string[]) => void;
  } = $props();

  let selectedLabel = $derived(
    multiple
      ? (selected.length === 0 ? 'Any Owner' : selected.length === 1
        ? (options.find(o => o.value === selected[0])?.label ?? selected[0])
        : `${selected.length} selected`)
      : (options.find(o => o.value === value)?.label ?? options.find(o => !o.disabled)?.label ?? '')
  );

  let maxLabelLen = $derived(Math.max(...options.map(o => o.label.length), 10));
  let triggerStyle = $derived(`width:calc(${maxLabelLen}ch + 40px);`);

  let open = $state(false);

  $effect(() => {
    onOpenChange(open);
  });
</script>

{#if multiple}
  <Select.Root type="multiple" value={selected} onValueChange={onSelectedChange} bind:open>
    <Select.Trigger class="dropdown drop-select-trigger {open ? 'open' : ''}" style={triggerStyle}>
      <Select.Value placeholder={selectedLabel}>{selectedLabel}</Select.Value>
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="opacity:0.5;flex-shrink:0;">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </Select.Trigger>
    <Select.Portal>
      <Select.Content class="drop-select-content">
        {#each options as opt, i (i)}
          <Select.Item class="drop-select-item" value={opt.value} label={opt.label} disabled={opt.disabled ?? false}>
            {#if selected.includes(opt.value)}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" class="drop-select-checkmark">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
            {:else}
              <span style="width:14px;display:inline-block;"></span>
            {/if}
            {opt.label}
          </Select.Item>
        {/each}
      </Select.Content>
    </Select.Portal>
  </Select.Root>
{:else}
  <Select.Root type="single" {value} {onValueChange} bind:open>
    <Select.Trigger class="dropdown drop-select-trigger {open ? 'open' : ''}" style={triggerStyle}>
      <Select.Value placeholder={selectedLabel}>{selectedLabel}</Select.Value>
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" style="opacity:0.5;flex-shrink:0;">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </Select.Trigger>
    <Select.Portal>
      <Select.Content class="drop-select-content">
        {#each options as opt, i (i)}
          <Select.Item class="drop-select-item" value={opt.value} label={opt.label} disabled={opt.disabled ?? false}>
            {opt.label}
          </Select.Item>
        {/each}
      </Select.Content>
    </Select.Portal>
  </Select.Root>
{/if}

<style>
  :global(.drop-select-trigger) {
    display: inline-flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    padding: 7px 12px;
    white-space: nowrap;
    font-weight: 600;
    font-size: 0.82rem;
    background: var(--surface-strong);
  }

  :global(.drop-select-trigger:hover) {
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
    color: var(--text);
  }

  :global(.drop-select-trigger.open) {
    border-color: var(--muted);
    color: var(--text);
  }

  :global(.drop-select-content) {
    background: color-mix(in srgb, var(--muted) 12%, var(--surface));
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 4px;
    min-width: var(--bits-select-anchor-width);
    box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
    z-index: 1050;
  }

  :global(.drop-select-item) {
    display: flex;
    align-items: center;
    gap: 6px;
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


  :global(.drop-select-item[data-disabled]) {
    opacity: 0.4;
    cursor: not-allowed;
    pointer-events: none;
  }
</style>
