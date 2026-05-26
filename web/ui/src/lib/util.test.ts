import { describe, it, expect } from 'vitest';
import { formatBytes, formatDuration, escapeHtml } from './util';

describe('formatBytes', () => {
  it('formats zero bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('formats negative bytes', () => {
    expect(formatBytes(-100)).toBe('0 B');
  });

  it('formats bytes', () => {
    expect(formatBytes(500)).toBe('500 B');
  });

  it('formats KiB', () => {
    const result = formatBytes(1024);
    expect(result).toContain('KiB');
  });

  it('formats MiB', () => {
    const result = formatBytes(1024 * 1024);
    expect(result).toContain('MiB');
  });

  it('formats GiB', () => {
    const result = formatBytes(1024 * 1024 * 1024);
    expect(result).toContain('GiB');
  });

  it('formats TiB', () => {
    const result = formatBytes(1024 * 1024 * 1024 * 1024);
    expect(result).toContain('TiB');
  });

  it('formats TiB for very large values', () => {
    const result = formatBytes(5 * 1024 * 1024 * 1024 * 1024);
    expect(result).toContain('TiB');
  });

  it('removes trailing zeros', () => {
    const result = formatBytes(1024);
    expect(result).not.toContain('.00');
  });

  it('uses correct units', () => {
    expect(formatBytes(1)).toContain('B');
    expect(formatBytes(1024)).toContain('KiB');
    expect(formatBytes(1024 ** 2)).toContain('MiB');
    expect(formatBytes(1024 ** 3)).toContain('GiB');
    expect(formatBytes(1024 ** 4)).toContain('TiB');
  });
});

describe('formatDuration', () => {
  it('formats expired', () => {
    expect(formatDuration(0)).toBe('expired');
    expect(formatDuration(-1)).toBe('expired');
  });

  it('formats seconds only', () => {
    expect(formatDuration(45)).toBe('45s remaining');
  });

  it('formats minutes and seconds', () => {
    const result = formatDuration(125);
    expect(result).toContain('2m');
    expect(result).toContain('5s');
  });

  it('formats hours', () => {
    const result = formatDuration(3661);
    expect(result).toContain('1h');
    expect(result).toContain('1m');
    expect(result).toContain('1s');
  });

  it('formats days', () => {
    const result = formatDuration(90061);
    expect(result).toContain('1d');
    expect(result).toContain('1h');
    expect(result).toContain('1m');
    expect(result).toContain('1s');
  });

  it('includes all parts from days down to seconds', () => {
    const result = formatDuration(86400 + 3600 + 60 + 1);
    expect(result).toContain('1d');
    expect(result).toContain('1h');
    expect(result).toContain('1m');
    expect(result).toContain('1s');
  });

  it('formats exactly one day', () => {
    const result = formatDuration(86400);
    expect(result).toContain('1d');
    expect(result).toContain('0h');
  });

  it('includes remaining suffix', () => {
    expect(formatDuration(60)).toContain('remaining');
  });
});

describe('escapeHtml', () => {
  it('escapes < and >', () => {
    expect(escapeHtml('<script>alert("xss")</script>')).toBe(
      '&lt;script&gt;alert("xss")&lt;/script&gt;'
    );
  });

  it('escapes ampersands', () => {
    expect(escapeHtml('a & b')).toBe('a &amp; b');
  });

  it('passes through plain text', () => {
    expect(escapeHtml('hello world')).toBe('hello world');
  });

  it('handles empty string', () => {
    expect(escapeHtml('')).toBe('');
  });

  it('does not escape quotes in text content', () => {
    const result = escapeHtml('text with "quotes"');
    expect(result).toContain('"quotes"');
    expect(result).not.toContain('&quot;');
  });

  it('escapes HTML tags', () => {
    expect(escapeHtml('<a href="test">click</a>')).toBe(
      '&lt;a href="test"&gt;click&lt;/a&gt;'
    );
  });
});
