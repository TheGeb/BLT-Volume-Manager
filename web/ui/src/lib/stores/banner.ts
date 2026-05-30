import { writable } from 'svelte/store';

export const bannerText = writable('');
export const bannerError = writable(false);

export function setBanner(msg: string, isError = false) {
  bannerText.set(msg);
  bannerError.set(isError);
}
