import { writable } from 'svelte/store';
import type { MigrationBackendSpec, MigrationResult } from '../types';
import * as api from '../api';
import { showToast } from './toast';

export const migrateModal = writable(false);
export const migrateRunning = writable(false);
export const migrateResult = writable<MigrationResult | null>(null);
export const migrateError = writable('');

export function emptyBackendSpec(): MigrationBackendSpec {
  return { type: 's3', bucket: '', endpoint: '', region: '', force_path_style: true, etcd_endpoints: [] };
}

export function openMigrateModal() {
  migrateResult.set(null);
  migrateError.set('');
  migrateModal.set(true);
}

export async function runMigrate(from: MigrationBackendSpec, to: MigrationBackendSpec, dryRun: boolean) {
  migrateRunning.set(true);
  migrateError.set('');
  try {
    const result = await api.runMigration(dryRun, from, to);
    migrateResult.set(result);
    if (!dryRun) showToast('Metadata migration complete');
  } catch (e: unknown) {
    migrateError.set(e instanceof Error ? e.message : 'Migration failed');
  } finally {
    migrateRunning.set(false);
  }
}
