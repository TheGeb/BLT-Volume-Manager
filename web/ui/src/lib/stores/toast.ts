import { writable } from 'svelte/store';

export interface Toast {
  id: number;
  message: string;
  error: boolean;
}

export const toasts = writable<Toast[]>([]);

let nextId = 0;

export function showToast(msg: string, isError = false) {
  if (!msg) return;
  const id = nextId++;
  const toast: Toast = { id, message: msg, error: isError };
  toasts.update(t => [...t, toast]);
  setTimeout(() => { dismissToast(id); }, 10000);
}

export function dismissToast(id: number) {
  toasts.update(t => t.filter(to => to.id !== id));
}
