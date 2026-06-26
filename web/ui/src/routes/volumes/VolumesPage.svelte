<script lang="ts">
  import type { VolumeOwnerInfo } from '$lib/types';
  import { landingShown } from '$lib/stores/volumes';
  import VolumeTree from '../../components/VolumeTree.svelte';
  import LandingPanel from './LandingPanel.svelte';

  let {
    volumes = [] as string[],
    loading = false,
    onSelect = (_vol: string) => {},
    volumeOwnerInfo = {} as Record<string, VolumeOwnerInfo>,
    onCreateTestVolume = (_name: string) => {},
    creatingTest = false,
    testStatus = '',
  }: {
    volumes?: string[];
    loading?: boolean;
    onSelect?: (vol: string) => void;
    volumeOwnerInfo?: Record<string, VolumeOwnerInfo>;
    onCreateTestVolume?: (name: string) => void;
    creatingTest?: boolean;
    testStatus?: string;
  } = $props();
</script>

{#if $landingShown}
  {#if volumes.length === 0 && !loading}
    <LandingPanel {onCreateTestVolume} {creatingTest} {testStatus} />
  {:else}
    <VolumeTree {volumes} {loading} {onSelect} {volumeOwnerInfo} />
  {/if}
{/if}
