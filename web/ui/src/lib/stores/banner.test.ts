import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { bannerText, bannerError, setBanner } from './banner';

describe('banner', () => {
  beforeEach(() => {
    bannerText.set('');
    bannerError.set(false);
  });

  it('starts empty', () => {
    expect(get(bannerText)).toBe('');
    expect(get(bannerError)).toBe(false);
  });

  it('setBanner updates text and defaults error to false', () => {
    setBanner('hello');
    expect(get(bannerText)).toBe('hello');
    expect(get(bannerError)).toBe(false);
  });

  it('setBanner with isError=true', () => {
    setBanner('oh no', true);
    expect(get(bannerText)).toBe('oh no');
    expect(get(bannerError)).toBe(true);
  });

  it('setBanner overwrites previous values', () => {
    setBanner('first', true);
    setBanner('second');
    expect(get(bannerText)).toBe('second');
    expect(get(bannerError)).toBe(false);
  });
});
