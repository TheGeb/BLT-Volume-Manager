<script lang="ts">
  import { Button } from 'bits-ui';

  export let onCreateTestVolume: (name: string) => void = () => {};
  export let creatingTest = false;
  export let testStatus = '';

  let testVolName = '';

  function handleCreate() {
    const name = testVolName.trim();
    if (!name) return;
    testVolName = '';
    onCreateTestVolume(name);
  }
</script>

<section class="panel" style="display:block;">
  <div style="text-align:center;padding:20px 0;">
    <h2 style="margin:0 0 8px;font-size:1.6rem;">BLT Volume Manager</h2>
    <p style="margin:0;color:var(--muted);font-size:0.95rem;">
      Select a volume from the list above to view its snapshots, owners, and stats.
    </p>
  </div>
  <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:16px;">
    <div style="background:rgb(255 255 255 / 3%);border:1px solid var(--border);border-radius:16px;padding:20px;">
      <h3 class="stats-heading" style="margin:0 0 12px;">Quick start</h3>
      <div style="display:flex;flex-direction:column;gap:8px;font-size:0.85rem;font-family:monospace;">
        <code style="padding:10px;background:rgb(0 0 0 / 20%);border-radius:8px;color:var(--accent);">docker volume create --driver blt-volume-manager --name my-volume</code>
        <code style="padding:10px;background:rgb(0 0 0 / 20%);border-radius:8px;color:var(--accent);">docker run -v my-volume:/data alpine echo hello</code>
      </div>
    </div>
    <div style="background:rgb(255 255 255 / 3%);border:1px solid var(--border);border-radius:16px;padding:20px;">
      <h3 class="stats-heading" style="margin:0 0 12px;">Create test data</h3>
      <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;">
        <input class="input" type="text" placeholder="name or group/name" style="width:200px;flex:none;" bind:value={testVolName} on:keydown={e => e.key === 'Enter' && handleCreate()} />
        <Button.Root class="button button-secondary" onclick={handleCreate} disabled={creatingTest}>
          {creatingTest ? 'Creating...' : 'Create sample data'}
        </Button.Root>
      </div>
      {#if testStatus}
        <div class="status" style="margin-top:12px;color:{testStatus.includes('Error') ? 'var(--red)' : 'inherit'}">{testStatus}</div>
      {/if}
    </div>
  </div>
</section>

<style>
  .stats-heading {
    margin: 0;
    font-size: 1.25rem;
  }

  .status {
    margin-top: 16px;
    color: var(--muted);
  }
</style>
