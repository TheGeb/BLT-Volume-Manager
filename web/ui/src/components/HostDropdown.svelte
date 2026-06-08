<script lang="ts">
  import { allHosts, hostsLoading, loadHosts } from '$lib/stores/snapshots';
  import DropSelect from './DropSelect.svelte';

  let {
    value = '',
    onValueChange = (_v: string) => {},
    volume = '',
  }: {
    value: string;
    onValueChange: (v: string) => void;
    volume: string;
  } = $props();

  $effect(() => {
    if (volume && ($allHosts?.length ?? 0) === 0) {
      loadHosts(volume);
    }
  });

  function onOpenChange(open: boolean) {
    if (open && volume && ($allHosts?.length ?? 0) === 0) {
      loadHosts(volume);
    }
  }
</script>

<DropSelect
  options={[
    { value: '', label: 'Any host' },
    ...($hostsLoading ? [{ value: '__loading__', label: 'Loading...', disabled: true }] : []),
    ...($allHosts ?? []).map(h => ({ value: h, label: h })),
  ]}
  {value}
  {onValueChange}
  {onOpenChange}
/>
