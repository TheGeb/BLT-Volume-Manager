import { describe, it, expect } from 'vitest';
import { showToast } from './toast';

describe('toast', () => {
  it('empty string is a no-op', () => {
    expect(() => { showToast(''); }).not.toThrow();
    expect(() => { showToast('', true); }).not.toThrow();
  });

  it('calls through to sonner without throwing', () => {
    expect(() => { showToast('hello'); }).not.toThrow();
    expect(() => { showToast('error', true); }).not.toThrow();
  });
});
