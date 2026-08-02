import { describe, it, expect } from 'vitest';
import { computeDateLabel, computeVersionLabel } from './format';

describe('computeDateLabel', () => {
  it('returns "Any date" when no filters set', () => {
    expect(computeDateLabel(undefined, undefined, undefined, undefined)).toBe('Any date');
  });

  it('returns time-of-day range when time-of-day filters are set', () => {
    const label = computeDateLabel(undefined, undefined, 0, 3600);
    expect(label).toContain('\u2013');
  });

  it('formats same-day range with time', () => {
    const tf = Date.UTC(2025, 0, 15, 9, 0, 0);
    const tt = Date.UTC(2025, 0, 15, 17, 0, 0);
    const label = computeDateLabel(tf, tt, undefined, undefined);
    expect(label).toContain('1/15');
    expect(label).toContain('9 AM');
    expect(label).toContain('5 PM');
  });

  it('formats different-day range', () => {
    const tf = Date.UTC(2025, 0, 15, 0, 0, 0);
    const tt = Date.UTC(2025, 0, 16, 0, 0, 0);
    const label = computeDateLabel(tf, tt, undefined, undefined);
    expect(label).toContain('1/15');
    expect(label).toContain('1/16');
    expect(label).toContain('\u2013');
  });

  it('formats single date falls back to range when only from set', () => {
    const tf = Date.UTC(2025, 5, 15, 0, 0, 0);
    const label = computeDateLabel(tf, undefined, undefined, undefined);
    expect(label).toContain('6/15');
    expect(label).toContain('\u2026');
  });

  it('returns time-of-day range with "--:--" for undefined bound', () => {
    const label = computeDateLabel(undefined, undefined, 3600, undefined);
    expect(label).toBe('1 AM\u2013--:--');
  });
});

describe('computeVersionLabel', () => {
  it('returns "Any version" when no bounds set', () => {
    expect(computeVersionLabel('', '', '', '')).toBe('Any version');
  });

  it('formats "from" only', () => {
    expect(computeVersionLabel('1', '5', '', '')).toBe('v1.5');
  });

  it('formats "to" only', () => {
    expect(computeVersionLabel('', '', '2', '0')).toBe('v2');
  });

  it('formats full range', () => {
    expect(computeVersionLabel('1', '0', '2', '0')).toBe('v1 - v2');
  });

  it('formats range with non-zero minor', () => {
    expect(computeVersionLabel('1', '5', '2', '3')).toBe('v1.5 - v2.3');
  });
});
