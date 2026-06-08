<script lang="ts">
  import { Dialog } from 'bits-ui';

  export let show = false;
  export let onClose: () => void = () => {};
  export let wide = false;
  export let xwide = false;

  function handleOpenChange(open: boolean) {
    if (!open) onClose();
  }
</script>

<Dialog.Root open={show} onOpenChange={handleOpenChange}>
  <Dialog.Portal>
    <Dialog.Overlay class="modal-overlay" />
    <Dialog.Content class="modal {wide ? 'modal-wide' : xwide ? 'modal-xwide' : ''}">
      <slot />
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  :global(.modal-overlay) {
    position: fixed; inset: 0; z-index: 1000;
    background: rgb(0 0 0 / 60%);
  }

  :global(.modal) {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 24px;
    padding: 28px;
    max-width: 440px;
    width: 90%;
    box-shadow: var(--shadow);
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 1001;
    transition: max-width 0.25s ease;
    outline: none;
  }

  :global(.modal-wide) {
    max-width: 600px;
  }

  :global(.modal-xwide) {
    max-width: 90vw;
    width: 90%;
  }
</style>
