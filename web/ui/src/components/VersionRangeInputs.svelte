<script lang="ts">
  import { parseVersion } from '$lib/util';

  export let from = '';
  export let to = '';
  export let onApply: (from: string, to: string) => void = () => {};
  export let onClear: () => void = () => {};

  let vfMajor = ''; let vfMinor = '';
  let vtMajor = ''; let vtMinor = '';

  $: dirty = !!(vfMajor || vfMinor || vtMajor || vtMinor);

  $: versionChanged = (() => {
    const fv = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const tv = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    return fv !== from || tv !== to;
  })();

  $: versionInvalid = (() => {
    const fv = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const tv = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    if (!fv || !tv) return false;
    const fp = parseVersion(fv);
    const tp = parseVersion(tv);
    if (!fp || !tp) return false;
    return tp.major < fp.major || (tp.major === fp.major && tp.minor < fp.minor);
  })();

  function cleanDigits(v: string): string {
    return v.replace(/[^0-9]/g, '');
  }

  export function loadFields() {
    const f = parseVersion(from);
    vfMajor = f ? String(f.major) : '';
    vfMinor = f ? String(f.minor) : '';
    const t = parseVersion(to);
    vtMajor = t ? String(t.major) : '';
    vtMinor = t ? String(t.minor) : '';
  }

  export function getValues(): { from: string; to: string } {
    const fv = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const tv = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    return { from: fv, to: tv };
  }

  function apply() {
    if (versionInvalid) return;
    const fv = vfMajor || vfMinor ? `${vfMajor || '0'}.${vfMinor || '0'}` : '';
    const tv = vtMajor || vtMinor ? `${vtMajor || '0'}.${vtMinor || '0'}` : '';
    onApply(fv, tv);
  }

  function clear() {
    vfMajor = ''; vfMinor = '';
    vtMajor = ''; vtMinor = '';
    onClear();
  }
</script>

<div class="version-range-filter">
  <div class="version-range-section">
    <span class="filter-label">from</span>
    <div class="version-input-group">
      <span class="version-prefix">v</span>
      <input type="text" placeholder="0" class="version-segment version-num" bind:value={vfMajor} on:input={() => { vfMajor = cleanDigits(vfMajor); }} size={vfMajor.length || 1}>
      <span class="version-dot">.</span>
      <input type="text" placeholder="0" class="version-segment version-num" bind:value={vfMinor} on:input={() => { vfMinor = cleanDigits(vfMinor); }} size={vfMinor.length || 1}>
    </div>
  </div>
  <div class="version-range-section">
    <span class="filter-label">to</span>
    <div class="version-input-group">
      <span class="version-prefix">v</span>
      <input type="text" placeholder="0" class="version-segment version-num" bind:value={vtMajor} on:input={() => { vtMajor = cleanDigits(vtMajor); }} size={vtMajor.length || 1}>
      <span class="version-dot">.</span>
      <input type="text" placeholder="0" class="version-segment version-num" bind:value={vtMinor} on:input={() => { vtMinor = cleanDigits(vtMinor); }} size={vtMinor.length || 1}>
    </div>
  </div>
  <div class="filter-actions">
    <button class="apply-btn" class:apply-btn-active={versionChanged && !versionInvalid} class:apply-btn-invalid={versionInvalid} on:click={apply}>Apply</button>
    <button class="clear-btn" class:clear-btn-active={dirty} on:click={clear}>Clear</button>
  </div>
</div>

<style>
  .version-range-filter {
    padding: 8px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .version-range-section {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  .filter-label {
    font-size: 0.65rem;
    color: var(--muted);
    font-weight: 500;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .version-input-group {
    display: inline-flex;
    align-items: center;
    gap: 1px;
    padding: 5px 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--surface-strong);
    transition: background 0.15s, border-color 0.15s;
  }

  .version-input-group:hover {
    background: var(--hover-bg);
    border-color: color-mix(in srgb, var(--muted), var(--bg) 40%);
  }

  .version-input-group:focus-within {
    background: var(--hover-bg);
    border-color: var(--muted);
  }

  .version-prefix,
  .version-dot {
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.85rem;
    color: var(--muted);
    padding: 0 1px;
    user-select: none;
  }

  .version-segment {
    padding: 1px 2px;
    border-radius: 3px;
    font-family: "SF Mono", "Fira Code", monospace;
    font-size: 0.85rem;
    font-weight: 400;
    color: var(--text);
    white-space: pre;
  }

  .version-segment:hover {
    background: var(--hover-bg);
  }

  .version-segment:focus {
    background: var(--hover-bg);
    color: var(--text);
  }

  .version-num {
    width: auto;
    min-width: 20px;
    max-width: 80px;
    background: transparent;
    border: none;
    color: inherit;
    font: inherit;
    font-weight: 400;
    text-align: center;
    padding: 0 1px;
  }

  .version-num::placeholder {
    color: var(--muted);
  }

  .version-num:focus {
    outline: none;
    background: var(--hover-bg);
    border-radius: 3px;
  }

  .version-num:focus::placeholder {
    opacity: 0;
  }

  .filter-actions {
    display: flex;
    gap: 6px;
    margin-top: 2px;
  }

  .apply-btn {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 12px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s;
  }

  .apply-btn:hover {
    background: var(--hover-bg);
  }

  .apply-btn-active {
    background: var(--accent);
    color: #fff;
    border-color: transparent;
  }

  .apply-btn-active:hover {
    background: color-mix(in srgb, var(--accent) 80%, #000);
  }

  .apply-btn-invalid {
    background: var(--hover-bg);
    color: var(--muted);
    border-color: var(--border);
    cursor: not-allowed;
    opacity: 0.5;
  }

  .clear-btn {
    background: var(--hover-bg);
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 10px;
    font-size: 0.75rem;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.15s, color 0.15s, border-color 0.15s;
  }

  .clear-btn:hover {
    background: var(--hover-bg);
  }

  .clear-btn-active {
    background: var(--red-bg);
    color: var(--red);
    border-color: var(--red);
  }

  .clear-btn-active:hover {
    background: rgb(248 113 113 / 20%);
  }
</style>
