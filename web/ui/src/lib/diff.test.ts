import { describe, it, expect } from 'vitest';
import { computeDiff } from './diff';
import type { DiffHunk } from './diff';

describe('computeDiff', () => {
  it('returns empty for identical content', () => {
    expect(computeDiff(['line1', 'line2', 'line3'], ['line1', 'line2', 'line3'])).toEqual([]);
    expect(computeDiff(['a'], ['a'])).toEqual([]);
    expect(computeDiff([], [])).toEqual([]);
  });

  it('detects added lines', () => {
    const a = ['line1', 'line2'];
    const b = ['line1', 'line2', 'line3'];
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
    const added = hunks[0].lines.filter(l => l.type === 'add');
    expect(added.length).toBe(1);
    expect(added[0].content).toBe('line3');
  });

  it('detects deleted lines', () => {
    const a = ['line1', 'line2', 'line3'];
    const b = ['line1', 'line3'];
    const hunks = computeDiff(a, b);
    const deleted = hunks.flatMap(h => h.lines).filter(l => l.type === 'del');
    expect(deleted.length).toBeGreaterThan(0);
  });

  it('detects modified lines', () => {
    const a = ['line1', 'line2', 'line3'];
    const b = ['line1', 'modified', 'line3'];
    const hunks = computeDiff(a, b);
    const changes = hunks.flatMap(h => h.lines).filter(l => l.type !== 'ctx');
    expect(changes.length).toBeGreaterThan(0);
  });

  it('handles completely different content', () => {
    const a = ['old1', 'old2', 'old3'];
    const b = ['new1', 'new2', 'new3'];
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
  });

  it('handles empty old content (all additions)', () => {
    const a: string[] = [];
    const b = ['new1', 'new2'];
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
    const allAdd = hunks.flatMap(h => h.lines).every(l => l.type === 'add');
    expect(allAdd).toBe(true);
  });

  it('handles empty new content (all deletions)', () => {
    const a = ['old1', 'old2'];
    const b: string[] = [];
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
    const allDel = hunks.flatMap(h => h.lines).every(l => l.type === 'del');
    expect(allDel).toBe(true);
  });

  it('groups hunks with context', () => {
    const a = Array.from({ length: 20 }, (_, i) => `line${i + 1}`);
    const b = [
      ...Array.from({ length: 10 }, (_, i) => `line${i + 1}`),
      'changed',
      ...Array.from({ length: 9 }, (_, i) => `line${i + 12}`),
    ];
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
  });

  it('handles single line identical content', () => {
    expect(computeDiff(['a'], ['a'])).toEqual([]);
  });

  it('handles single line changed content', () => {
    const hunks = computeDiff(['a'], ['b']);
    expect(hunks.length).toBeGreaterThan(0);
    const changes = hunks.flatMap(h => h.lines).filter(l => l.type !== 'ctx');
    expect(changes.length).toBeGreaterThan(0);
  });

  it('provides correct line numbers', () => {
    const a = ['keep1', 'remove', 'keep2'];
    const b = ['keep1', 'keep2'];
    const hunks = computeDiff(a, b);
    const allLines = hunks.flatMap(h => h.lines);
    const ctxLines = allLines.filter(l => l.type === 'ctx');
    for (const line of ctxLines) {
      expect(line.oldLineNo).toBeGreaterThan(0);
      expect(line.newLineNo).toBeGreaterThan(0);
    }
  });

  it('handles large diff', () => {
    const a = Array.from({ length: 100 }, (_, i) => `line${i}`);
    const b = Array.from({ length: 100 }, (_, i) =>
      i % 10 === 0 ? `modified${i}` : `line${i}`
    );
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
  });

  it('handles Unicode content', () => {
    const a = ['hello', '世界', 'foo'];
    const b = ['hello', '世界', 'bar', '👋'];
    const hunks = computeDiff(a, b);
    expect(hunks.length).toBeGreaterThan(0);
  });

  it('returns hunks with correct structure', () => {
    const a = ['a', 'b'];
    const b = ['a', 'c', 'd'];
    const hunks = computeDiff(a, b);
    for (const hunk of hunks) {
      expect(hunk).toHaveProperty('oldStart');
      expect(hunk).toHaveProperty('oldLen');
      expect(hunk).toHaveProperty('newStart');
      expect(hunk).toHaveProperty('newLen');
      expect(Array.isArray(hunk.lines)).toBe(true);
    }
  });
});
