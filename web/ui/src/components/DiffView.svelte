<script lang="ts">
  import { escapeHtml } from '../lib/util';

  let { diff = [], path = '', sideBySide = false, onToggleLayout = () => {} }: {
    diff?: { t: string; line: string }[];
    path?: string;
    sideBySide?: boolean;
    onToggleLayout?: () => void;
  } = $props();

  function renderUnified() {
    let ctxCount = 0;
    const parts: Array<{ type: string; line: string }> = [];

    function flushCtx() {
      if (ctxCount > 3) {
        parts.push({ type: 'ctx-hidden', line: `... ${ctxCount - 3} common lines hidden ...` });
      }
      ctxCount = 0;
    }

    for (const entry of diff) {
      if (entry.t === 'ctx') {
        ctxCount++;
        if (ctxCount <= 3) {
          const line = entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line;
          parts.push({ type: 'ctx', line });
        }
      } else {
        flushCtx();
        const line = entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line;
        parts.push({ type: entry.t, line });
      }
    }
    flushCtx();
    return parts;
  }

  function renderSideBySide() {
    let ctxCount = 0;
    const pendingDel: string[] = [];
    const pendingAdd: string[] = [];
    const pairs: Array<{ left: string; right: string; ltype: string; rtype: string }> = [];

    function flushCtx() {
      if (ctxCount > 3) {
        pairs.push({ left: `... ${ctxCount - 3} common lines hidden ...`, right: '', ltype: 'ctx-hidden', rtype: 'ctx-hidden' });
      }
      ctxCount = 0;
    }

    function flushPending() {
      if (pendingDel.length === 0 && pendingAdd.length === 0) return;
      const maxLen = Math.max(pendingDel.length, pendingAdd.length);
      for (let i = 0; i < maxLen; i++) {
        pairs.push({
          left: i < pendingDel.length ? (pendingDel[i] ?? '') : '',
          right: i < pendingAdd.length ? (pendingAdd[i] ?? '') : '',
          ltype: i < pendingDel.length ? 'del' : 'ctx',
          rtype: i < pendingAdd.length ? 'add' : 'ctx',
        });
      }
      pendingDel.length = 0;
      pendingAdd.length = 0;
    }

    for (const entry of diff) {
      if (entry.t === 'ctx') {
        flushPending();
        ctxCount++;
        if (ctxCount <= 3) {
          const line = entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line;
          pairs.push({ left: line, right: line, ltype: 'ctx', rtype: 'ctx' });
        }
      } else if (entry.t === 'del') {
        flushCtx();
        pendingDel.push(entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line);
      } else if (entry.t === 'add') {
        flushCtx();
        pendingAdd.push(entry.line.length > 200 ? entry.line.slice(0, 200) + '...' : entry.line);
      }
    }
    flushPending();
    flushCtx();
    return pairs;
  }

  let unifiedParts = $derived(renderUnified());
  let sideBySidePairs = $derived(renderSideBySide());
</script>

<div style="margin-bottom:8px;font-weight:700;display:flex;align-items:center;gap:8px;">
  <span>Diff: {escapeHtml(path)}</span>
  <button style="font-size:0.75rem;padding:2px 8px;cursor:pointer;background:var(--bg);color:var(--text);border:1px solid var(--border);border-radius:4px;" onclick={onToggleLayout}>
    {sideBySide ? 'Side by side' : 'Inline'}
  </button>
</div>

{#if sideBySide}
  <div style="display:grid;grid-template-columns:1fr 1fr;gap:0;font-size:0.85rem;font-family:monospace;">
    <div style="padding:2px 4px;font-weight:600;border-bottom:1px solid var(--border);background:rgb(255 255 255 / 3%);">Old</div>
    <div style="padding:2px 4px;font-weight:600;border-bottom:1px solid var(--border);background:rgb(255 255 255 / 3%);">New</div>
    {#each sideBySidePairs as pair (pair.left + pair.right)}
      <div style="padding:1px 4px;white-space:pre-wrap;background:{pair.ltype === 'del' ? 'rgba(248,113,113,0.1)' : pair.ltype === 'ctx-hidden' ? 'transparent' : 'rgba(255,255,255,0.02)'};color:{pair.ltype === 'ctx-hidden' ? 'var(--muted)' : 'inherit'};font-size:{pair.ltype === 'ctx-hidden' ? '0.75rem' : 'inherit'}">{escapeHtml(pair.left)}</div>
      <div style="padding:1px 4px;white-space:pre-wrap;background:{pair.rtype === 'add' ? 'rgba(52,211,153,0.1)' : pair.rtype === 'ctx-hidden' ? 'transparent' : 'rgba(255,255,255,0.02)'};color:{pair.rtype === 'ctx-hidden' ? 'var(--muted)' : 'inherit'};font-size:{pair.rtype === 'ctx-hidden' ? '0.75rem' : 'inherit'}">{escapeHtml(pair.right)}</div>
    {/each}
  </div>
{:else}
  {#each unifiedParts as part (part.line)}
    {#if part.type === 'ctx-hidden'}
      <div style="color:var(--muted);font-size:0.8rem;padding:2px 0;">{escapeHtml(part.line)}</div>
    {:else}
      <div style="padding:1px 4px;font-size:0.85rem;background:{part.type === 'add' ? 'rgba(52,211,153,0.1)' : part.type === 'del' ? 'rgba(248,113,113,0.1)' : 'rgba(255,255,255,0.02)'};border-radius:2px;white-space:pre-wrap;">
        {part.type === 'add' ? '+' : part.type === 'del' ? '-' : ' '} {escapeHtml(part.line)}
      </div>
    {/if}
  {/each}
{/if}
