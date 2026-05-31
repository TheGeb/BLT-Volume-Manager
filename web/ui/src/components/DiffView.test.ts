import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';
import DiffView from './DiffView.svelte';

interface DiffEntry { t: string; line: string }

afterEach(cleanup);

describe('DiffView', () => {
  it('renders path and toggle button', () => {
    render(DiffView, { diff: [], path: 'src/main.go', sideBySide: false });
    expect(screen.getByText(/Diff:/)).toBeTruthy();
    expect(screen.getByRole('button', { name: /Inline/ })).toBeTruthy();
  });

  it('renders unified diff lines with +/- prefixes', () => {
    const diff: DiffEntry[] = [
      { t: 'ctx', line: 'package main' },
      { t: 'del', line: 'old code' },
      { t: 'add', line: 'new code' },
      { t: 'ctx', line: '}' },
    ];
    const { container } = render(DiffView, { diff, path: 'test.go', sideBySide: false });

    const lines = container.querySelectorAll('[style*="white-space: pre-wrap"]');
    const texts = Array.from(lines).map(el => el.textContent || '');
    expect(texts.some(t => t.includes('package main'))).toBe(true);
    expect(texts.some(t => t.includes('- old code'))).toBe(true);
    expect(texts.some(t => t.includes('+ new code'))).toBe(true);
  });

  it('collapses context lines beyond 3', () => {
    const diff: DiffEntry[] = [
      { t: 'ctx', line: 'c1' },
      { t: 'ctx', line: 'c2' },
      { t: 'ctx', line: 'c3' },
      { t: 'ctx', line: 'c4' },
      { t: 'ctx', line: 'c5' },
      { t: 'add', line: 'changed' },
    ];
    render(DiffView, { diff, path: 'f', sideBySide: false });
    expect(screen.getByText(/common lines hidden/)).toBeTruthy();
  });

  it('shows inline button label when in sideBySide mode', () => {
    render(DiffView, { diff: [], path: 'x', sideBySide: true });
    expect(screen.getByRole('button', { name: /Side by side/ })).toBeTruthy();
  });

  it('renders side-by-side grid headers', () => {
    const diff: DiffEntry[] = [
      { t: 'del', line: 'removed line' },
      { t: 'add', line: 'added line' },
    ];
    const { container } = render(DiffView, { diff, path: 'script.js', sideBySide: true });

    const headers = container.querySelectorAll('[style*="font-weight:600"]');
    const headerTexts = Array.from(headers).map(el => el.textContent.trim() || '');
    expect(headerTexts).toContain('Old');
    expect(headerTexts).toContain('New');
  });

  it('calls onToggleLayout when toggle button clicked', async () => {
    const onToggleLayout = vi.fn();
    render(DiffView, { diff: [], path: 'x', sideBySide: false, onToggleLayout });

    const btn = screen.getByRole('button', { name: /Inline/ });
    await fireEvent.click(btn);
    expect(onToggleLayout).toHaveBeenCalledOnce();
  });

  it('renders empty state without diff lines', () => {
    const { container } = render(DiffView, { diff: [], path: 'empty', sideBySide: false });

    const lines = container.querySelectorAll('[style*="white-space: pre-wrap"]');
    expect(lines.length).toBe(0);
  });

  it('truncates lines longer than 200 characters', () => {
    const longLine = 'x'.repeat(250);
    const diff: DiffEntry[] = [{ t: 'add', line: longLine }];
    const { container } = render(DiffView, { diff, path: 'big', sideBySide: false });

    const lineElements = container.querySelectorAll('[style*="white-space: pre-wrap"]');
    expect(lineElements.length).toBeGreaterThan(0);
    const text = (lineElements[0]?.textContent ?? '').trim();
    expect(text).toContain('...');
    expect(text.length).toBeLessThan(longLine.length + 5);
  });

  it('shows correct hidden count', () => {
    const diff: DiffEntry[] = [
      { t: 'ctx', line: 'a' },
      { t: 'ctx', line: 'b' },
      { t: 'ctx', line: 'c' },
      { t: 'ctx', line: 'd' },
      { t: 'ctx', line: 'e' },
      { t: 'ctx', line: 'f' },
    ];
    render(DiffView, { diff, path: 'f', sideBySide: false });
    expect(screen.getByText(/3 common lines hidden/)).toBeTruthy();
  });

  it('shows add lines with green background', () => {
    const diff: DiffEntry[] = [{ t: 'add', line: 'fresh line' }];
    const { container } = render(DiffView, { diff, path: 'f', sideBySide: false });

    const addLine = container.querySelector('[style*="rgba(52, 211, 153, 0.1)"]');
    expect(addLine).toBeTruthy();
  });

  it('shows del lines with red background', () => {
    const diff: DiffEntry[] = [{ t: 'del', line: 'stale line' }];
    const { container } = render(DiffView, { diff, path: 'f', sideBySide: false });

    const delLine = container.querySelector('[style*="rgba(248, 113, 113, 0.1)"]');
    expect(delLine).toBeTruthy();
  });

  it('escapes HTML in rendered diff lines', () => {
    const diff: DiffEntry[] = [{ t: 'ctx', line: '<script>alert("xss")</script>' }];
    const { container } = render(DiffView, { diff, path: 'f', sideBySide: false });

    const html = container.querySelector('[style*="white-space: pre-wrap"]')?.innerHTML ?? '';
    expect(html).toContain('amp;lt;script');
    expect(html).not.toContain('<script>');
  });
});
