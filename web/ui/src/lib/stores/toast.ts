import { toast as sonnerToast } from 'svelte-sonner';

export function showToast(msg: string, isError = false) {
  if (!msg) return;
  if (isError) {
    sonnerToast.error(msg);
  } else {
    sonnerToast(msg);
  }
}
