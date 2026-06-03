<script lang="ts">
  import { Button } from 'bits-ui';
  import type { DiffHunk } from '../lib/diff';

  let {
    diffHunks = [],
    sideBySide = false,
    oldLabel = '',
    newLabel = '',
    onToggleLayout = () => {},
  }: {
    diffHunks: DiffHunk[];
    sideBySide: boolean;
    oldLabel: string;
    newLabel: string;
    onToggleLayout: () => void;
  } = $props();
</script>

<div style="display:flex;gap:8px;margin-bottom:8px;align-items:center;">
  <span style="font-size:0.85rem;color:var(--muted);">Diff: {oldLabel} vs {newLabel}</span>
  <Button.Root class="button button-secondary button-xs" style="margin-left:auto;" onclick={onToggleLayout}>
    {sideBySide ? 'Inline' : 'Side-by-side'}
  </Button.Root>
</div>

{#if sideBySide}
  <div style="display:flex;gap:0;border:1px solid var(--border);border-radius:8px;overflow:hidden;">
    <div style="flex:1;overflow-x:auto;border-right:1px solid var(--border);">
      <div style="padding:4px 8px;font-size:0.75rem;color:var(--muted);border-bottom:1px solid var(--border);background:rgb(255 255 255 / 3%);">Old ({oldLabel})</div>
      {#each diffHunks as hunk (hunk.oldStart + '-' + hunk.newStart)}
        <div style="padding:2px 8px;font-size:0.75rem;color:var(--muted);background:rgb(255 255 255 / 3%);border-bottom:1px solid var(--border);font-family:monospace;">
          @@ -{hunk.oldStart},{hunk.oldLen} +{hunk.newStart},{hunk.newLen} @@
        </div>
        {#each hunk.lines as entry (entry.oldLineNo + '-' + entry.newLineNo)}
          {#if entry.type === 'add'}
            <div style="padding:0 4px;font-size:0.85rem;background:var(--green-bg);">&nbsp;</div>
          {:else}
            <div style="display:flex;padding:0 4px;font-size:0.85rem;background:{entry.type === 'del' ? 'var(--red-bg)' : ''};">
              <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.oldLineNo || ''}</span>
              <span style="width:1ch;flex-shrink:0;color:{entry.type === 'del' ? 'var(--red)' : ''};">{entry.type === 'del' ? '-' : ' '}</span>
              <span style="flex:1;white-space:pre-wrap;">{entry.content}</span>
            </div>
          {/if}
        {/each}
      {/each}
    </div>
    <div style="flex:1;overflow-x:auto;">
      <div style="padding:4px 8px;font-size:0.75rem;color:var(--muted);border-bottom:1px solid var(--border);background:rgb(255 255 255 / 3%);">New ({newLabel})</div>
      {#each diffHunks as hunk (hunk.oldStart + '-' + hunk.newStart)}
        <div style="padding:2px 8px;font-size:0.75rem;color:var(--muted);background:rgb(255 255 255 / 3%);border-bottom:1px solid var(--border);font-family:monospace;">
          @@ -{hunk.oldStart},{hunk.oldLen} +{hunk.newStart},{hunk.newLen} @@
        </div>
        {#each hunk.lines as entry (entry.oldLineNo + '-' + entry.newLineNo)}
          {#if entry.type === 'del'}
            <div style="padding:0 4px;font-size:0.85rem;background:var(--red-bg);">&nbsp;</div>
          {:else}
            <div style="display:flex;padding:0 4px;font-size:0.85rem;background:{entry.type === 'add' ? 'var(--green-bg)' : ''};">
              <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.newLineNo || ''}</span>
              <span style="width:1ch;flex-shrink:0;color:{entry.type === 'add' ? 'var(--green)' : ''};">{entry.type === 'add' ? '+' : ' '}</span>
              <span style="flex:1;white-space:pre-wrap;">{entry.content}</span>
            </div>
          {/if}
        {/each}
      {/each}
    </div>
  </div>
{:else}
  {#each diffHunks as hunk (hunk.oldStart + '-' + hunk.newStart)}
    <div style="padding:2px 8px;margin:4px 0;font-size:0.8rem;color:var(--muted);background:rgb(255 255 255 / 3%);border-radius:4px;font-family:monospace;">
      @@ -{hunk.oldStart},{hunk.oldLen} +{hunk.newStart},{hunk.newLen} @@
    </div>
    {#each hunk.lines as entry (entry.oldLineNo + '-' + entry.newLineNo)}
      <div style="display:flex;padding:1px 4px;font-size:0.85rem;background:{entry.type === 'add' ? 'var(--green-bg)' : entry.type === 'del' ? 'var(--red-bg)' : ''};border-radius:2px;">
        <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.type === 'add' ? '' : entry.oldLineNo}</span>
        <span style="width:3ch;text-align:right;color:var(--muted);flex-shrink:0;user-select:none;">{entry.type === 'del' ? '' : entry.newLineNo}</span>
        <span style="width:1.2ch;flex-shrink:0;color:{entry.type === 'add' ? 'var(--green)' : entry.type === 'del' ? 'var(--red)' : ''};">{entry.type === 'add' ? '+' : entry.type === 'del' ? '-' : ' '}</span>
        <span style="flex:1;white-space:pre-wrap;">{entry.content}</span>
      </div>
    {/each}
  {/each}
{/if}
