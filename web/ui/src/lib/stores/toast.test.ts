import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { toasts, showToast, dismissToast } from './toast';

describe('toast', () => {
  beforeEach(() => {
    toasts.set([]);
  });

  it('starts empty', () => {
    expect(get(toasts)).toEqual([]);
  });

  it('showToast creates a toast with default error=false', () => {
    showToast('hello');
    const list = get(toasts);
    expect(list).toHaveLength(1);
    expect(list[0]?.message).toBe('hello');
    expect(list[0]?.error).toBe(false);
  });

  it('showToast with isError=true', () => {
    showToast('oh no', true);
    const list = get(toasts);
    expect(list).toHaveLength(1);
    expect(list[0]?.message).toBe('oh no');
    expect(list[0]?.error).toBe(true);
  });

  it('showToast appends toasts', () => {
    showToast('first', true);
    showToast('second');
    const list = get(toasts);
    expect(list).toHaveLength(2);
    expect(list[0]?.message).toBe('first');
    expect(list[1]?.message).toBe('second');
  });

  it('empty string is a no-op', () => {
    showToast('hello');
    showToast('');
    expect(get(toasts)).toHaveLength(1);
  });

  it('dismissToast removes specific toast', () => {
    showToast('keep');
    showToast('remove');
    const list = get(toasts);
    const toRemove = list[1];
    expect(toRemove).toBeDefined();
    if (toRemove) dismissToast(toRemove.id);
    const after = get(toasts);
    expect(after).toHaveLength(1);
    expect(after[0]?.message).toBe('keep');
  });
});
