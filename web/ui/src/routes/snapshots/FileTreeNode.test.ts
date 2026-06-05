import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import FileTreeNode from './FileTreeNode.svelte';
import type { FileNode } from '$lib/types';

afterEach(cleanup);

function makeDir(name: string, path: string, children?: Record<string, FileNode>): FileNode {
  return { name, type: 'dir', path, ...(children ? { children } : {}), full_path: path };
}

function makeFile(name: string, path: string, size?: number): FileNode {
  return { name, type: 'file', path, full_path: path, ...(size != null ? { size } : {}) };
}

describe('FileTreeNode search', () => {
  it('highlights matched files with subtle background', () => {
    const node = makeFile('config.ts', '/src/config.ts', 1024);

    const { container } = render(FileTreeNode, {
      node,
      depth: 1,
      searchResults: ['/src/config.ts'],
      searchActivePath: '',
      searchAncestorPaths: new Set<string>(),
    });

    const dirBg = container.querySelector('[style*="color-mix(in srgb, var(--accent) 5%, transparent)"]');
    expect(dirBg).toBeTruthy();
  });

  it('highlights active search result with stronger highlight and border', () => {
    const node = makeFile('config.ts', '/src/config.ts', 1024);

    const { container } = render(FileTreeNode, {
      node,
      depth: 1,
      searchResults: ['/src/config.ts'],
      searchActivePath: '/src/config.ts',
      searchAncestorPaths: new Set<string>(),
    });

    const activeBg = container.querySelector('[style*="color-mix(in srgb, var(--accent) 14%, transparent)"]');
    expect(activeBg).toBeTruthy();
    const activeBorder = container.querySelector('[style*="2px solid var(--accent)"]');
    expect(activeBorder).toBeTruthy();
  });

  it('does not highlight files not in search results', () => {
    const node = makeFile('other.ts', '/src/other.ts');

    const { container } = render(FileTreeNode, {
      node,
      depth: 1,
      searchResults: ['/src/config.ts'],
      searchActivePath: '',
      searchAncestorPaths: new Set<string>(),
    });

    const hl = container.querySelector('[style*="color-mix"]');
    expect(hl).toBeFalsy();
  });

  it('auto-expands directory that is an ancestor of a search match', () => {
    const node = makeDir('src', '/src', {
      config: makeFile('config.ts', '/src/config.ts'),
    });

    const { container } = render(FileTreeNode, {
      node,
      depth: 0,
      expanded: false,
      searchResults: [],
      searchActivePath: '',
      searchAncestorPaths: new Set(['/src']),
    });

    const slideGrid = container.querySelector('.slide-grid');
    expect(slideGrid?.classList.contains('expanded')).toBe(true);
  });

  it('auto-expands ancestor of active search result', () => {
    const node = makeDir('src', '/src', {
      config: makeFile('config.ts', '/src/config.ts'),
    });

    const { container } = render(FileTreeNode, {
      node,
      depth: 0,
      expanded: false,
      searchResults: ['/src/config.ts'],
      searchActivePath: '/src/config.ts',
      searchAncestorPaths: new Set<string>(),
    });

    const slideGrid = container.querySelector('.slide-grid');
    expect(slideGrid?.classList.contains('expanded')).toBe(true);
  });

  it('highlights directory with match inside it', () => {
    const node = makeDir('src', '/src', {
      config: makeFile('config.ts', '/src/config.ts'),
    });

    const { container } = render(FileTreeNode, {
      node,
      depth: 0,
      searchResults: ['/src'],
      searchActivePath: '',
      searchAncestorPaths: new Set<string>(),
    });

    const dirBg = container.querySelector('[style*="color-mix(in srgb, var(--accent) 5%, transparent)"]');
    expect(dirBg).toBeTruthy();
  });

  it('highlights directory containing the active search result', () => {
    const node = makeDir('src', '/src', {
      config: makeFile('config.ts', '/src/config.ts'),
    });

    const { container } = render(FileTreeNode, {
      node,
      depth: 0,
      searchResults: ['/src/config.ts'],
      searchActivePath: '/src/config.ts',
      searchAncestorPaths: new Set(['/src']),
    });

    const activeBg = container.querySelector('[style*="color-mix(in srgb, var(--accent) 14%, transparent)"]');
    expect(activeBg).toBeTruthy();
  });

  it('does not auto-expand non-ancestor directory', () => {
    const node = makeDir('tests', '/tests', {
      spec: makeFile('spec.ts', '/tests/spec.ts'),
    });

    const { container } = render(FileTreeNode, {
      node,
      depth: 0,
      expanded: false,
      searchResults: [],
      searchActivePath: '',
      searchAncestorPaths: new Set(['/src']),
    });

    expect(container.querySelector('.slide-grid.expanded')).toBeFalsy();
  });

  it('sets data-tree-path on file nodes for scroll-to-search', () => {
    const node = makeFile('config.ts', '/src/config.ts');

    const { container } = render(FileTreeNode, {
      node,
      depth: 1,
      searchResults: [],
      searchActivePath: '',
      searchAncestorPaths: new Set<string>(),
    });

    const el = container.querySelector('[data-tree-path="/src/config.ts"]');
    expect(el).toBeTruthy();
  });

  it('uses full_path for data-tree-path when available', () => {
    const node: FileNode = {
      name: 'config.ts',
      type: 'file',
      path: '/alt/config.ts',
      full_path: '/src/config.ts',
    };

    const { container } = render(FileTreeNode, {
      node,
      depth: 1,
      searchResults: [],
      searchActivePath: '',
      searchAncestorPaths: new Set<string>(),
    });

    const el = container.querySelector('[data-tree-path="/src/config.ts"]');
    expect(el).toBeTruthy();
  });
});
