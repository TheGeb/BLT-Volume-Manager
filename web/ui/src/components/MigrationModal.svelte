<script lang="ts">
  import { Button } from 'bits-ui';
  import Modal from './Modal.svelte';
  import type { MigrationBackendSpec } from '$lib/types';
  import { fetchVersion } from '$lib/api';
  import { migrateModal, migrateRunning, migrateResult, migrateError, emptyBackendSpec, runMigrate } from '$lib/stores/migration';

  let from = $state<MigrationBackendSpec>(emptyBackendSpec());
  let to = $state<MigrationBackendSpec>(emptyBackendSpec());
  let fromEtcdText = $state<{ value: string }>({ value: from.etcd_endpoints.join(', ') });
  let toEtcdText = $state<{ value: string }>({ value: to.etcd_endpoints.join(', ') });

  $effect(() => {
    from.etcd_endpoints = fromEtcdText.value.split(',').map((s: string) => s.trim()).filter(Boolean);
  });
  $effect(() => {
    to.etcd_endpoints = toEtcdText.value.split(',').map((s: string) => s.trim()).filter(Boolean);
  });

  async function useCurrent(spec: MigrationBackendSpec) {
    try {
      const v = await fetchVersion();
      spec.type = (v.metadata_backend === 'etcd' ? 'etcd' : 's3') as MigrationBackendSpec['type'];
      spec.bucket = v.s3_bucket ?? '';
      spec.endpoint = v.s3_endpoint ?? '';
      spec.region = '';
      spec.force_path_style = true;
      spec.etcd_endpoints = v.etcd_endpoints ?? [];
    } catch {
      migrateError.set('Could not load current backend');
    }
  }
</script>

<Modal show={$migrateModal} onClose={() => $migrateModal = false} wide={true}>
  <h3 style="margin:0 0 12px;">Migrate metadata</h3>
  <p style="margin:0 0 12px;color:var(--muted);font-size:0.85rem;">
    This migrates the <strong>entire metadata database</strong> (owner locks,
    registered volumes, version counters, restore points) from the source
    backend to the destination. Owner locks are migrated with a refreshed
    expiry. S3 credentials come from this server's environment.
  </p>
  <p style="margin:0 0 12px;color:var(--yellow);font-size:0.85rem;">
    Stop other hosts and this server's writes before running a real migration,
    and preview with "Dry run" first.
  </p>
  <p style="margin:0 0 4px;color:var(--red);font-size:0.85rem;font-weight:600;">
    No volumes may be mounted during the migration.
  </p>
  <p style="margin:0 0 12px;color:var(--muted);font-size:0.85rem;">
    Active owner locks are refreshed with their remaining lifetime, already
    expired ones are skipped, and a fresh lock is (re)created by the owning host
    when it starts up against the new backend. Mounting mid-cutover can split or
    drop ownership.
  </p>

  <div class="migrate-columns">
    <div>
      <h4 style="margin:0 0 8px;font-size:0.9rem;">Source</h4>
      {@render backendFields(from, fromEtcdText, () => useCurrent(from))}
    </div>
    <div>
      <h4 style="margin:0 0 8px;font-size:0.9rem;">Destination</h4>
      {@render backendFields(to, toEtcdText, () => useCurrent(to))}
    </div>
  </div>

  {#if $migrateError}
    <p style="margin:12px 0 0;color:var(--red);font-size:0.85rem;">{$migrateError}</p>
  {/if}
  {#if $migrateResult}
    <div style="margin:12px 0 0;padding:10px 12px;border:1px solid var(--border);border-radius:8px;font-size:0.85rem;color:var(--muted);">
      {#if $migrateResult.dry_run}
        <strong style="color:var(--text);">Dry run:</strong> {$migrateResult.owner_locks} owner lock{$migrateResult.owner_locks === 1 ? '' : 's'} and {$migrateResult.keys} other entr{$migrateResult.keys === 1 ? 'y' : 'ies'} would be migrated.
      {:else}
        {$migrateResult.owner_locks} owner lock{$migrateResult.owner_locks === 1 ? '' : 's'} and {$migrateResult.keys} other entr{$migrateResult.keys === 1 ? 'y' : 'ies'} migrated.
      {/if}
    </div>
  {/if}

  <div class="modal-footer" style="margin-top:16px;">
    <Button.Root class="button button-secondary" onclick={() => $migrateModal = false}>Cancel</Button.Root>
    <Button.Root class="button button-secondary" onclick={() => runMigrate(from, to, true)} disabled={$migrateRunning}>
      {$migrateRunning ? 'Running...' : 'Dry run'}
    </Button.Root>
    <Button.Root class="button button-destructive" onclick={() => runMigrate(from, to, false)} disabled={$migrateRunning}>
      {$migrateRunning ? 'Running...' : 'Run migration'}
    </Button.Root>
  </div>
</Modal>

{#snippet backendFields(spec: MigrationBackendSpec, etcdText: { value: string }, onUseCurrent: () => void)}
  <div class="backend-fields">
    <label class="field-row">
      <span class="field-label">Type</span>
      <select class="input migrate-input" bind:value={spec.type}>
        <option value="s3">S3</option>
        <option value="etcd">etcd</option>
      </select>
    </label>
    {#if spec.type === 's3'}
      <label class="field-row">
        <span class="field-label">Bucket</span>
        <input class="input migrate-input" type="text" bind:value={spec.bucket} placeholder="my-meta" />
      </label>
      <label class="field-row">
        <span class="field-label">Endpoint</span>
        <input class="input migrate-input" type="text" bind:value={spec.endpoint} placeholder="https://s3.example.com" />
      </label>
      <label class="field-row">
        <span class="field-label">Region</span>
        <input class="input migrate-input" type="text" bind:value={spec.region} placeholder="us-east-1" />
      </label>
      <label class="checkbox-row">
        <input type="checkbox" bind:checked={spec.force_path_style} />
        Force path-style (MinIO / Garage)
      </label>
    {:else}
      <label class="field-row">
        <span class="field-label">Endpoints</span>
        <input class="input migrate-input" type="text" bind:value={etcdText.value} placeholder="http://127.0.0.1:2379" />
      </label>
    {/if}
    <div class="field-row">
      <button class="button button-secondary button-xs" type="button" onclick={onUseCurrent}>Use current backend</button>
    </div>
  </div>
{/snippet}

<style>
  .migrate-columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .backend-fields { display: flex; flex-direction: column; gap: 8px; }
  .field-row { display: flex; flex-direction: column; gap: 4px; }
  .field-label { font-size: 0.78rem; color: var(--muted); }
  .checkbox-row { display: flex; align-items: center; gap: 8px; font-size: 0.85rem; color: var(--text); }

  .migrate-input {
    flex: none;
    width: 100%;
    box-sizing: border-box;
    padding: 8px 10px;
    font-size: 0.85rem;
    border-radius: 8px;
  }

  input.migrate-input,
  select.migrate-input {
    min-width: 0;
    margin: 0;
  }

  @media (width <= 640px) {
    .migrate-columns { grid-template-columns: 1fr; }
  }
</style>
