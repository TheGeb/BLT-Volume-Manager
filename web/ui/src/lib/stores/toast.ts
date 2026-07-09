import { writable } from 'svelte/store';
import { toast as sonnerToast } from 'svelte-sonner';
import ToastCopyButton from '../components/ToastCopyButton.svelte';

export const toastCopyMsg = writable('');

export function showToast(msg: string, isError = false) {
  if (!msg) return;
  toastCopyMsg.set(msg);
  if (isError) {
    sonnerToast.error(msg, { action: ToastCopyButton });
  } else {
    sonnerToast(msg, { action: ToastCopyButton });
  }
}
