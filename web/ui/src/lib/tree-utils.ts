import type { FileNode, DiffResult } from './types';

interface TreeNode {
  name: string;
  type: string;
  path: string;
  full_path?: string | undefined;
  size?: number | undefined;
  children?: Record<string, TreeNode> | undefined;
  dirDiffType?: string | undefined;
}

export function collectAllPaths(node: TreeNode): string[] {
  const paths: string[] = [];
  const p = node.full_path ?? node.path;
  if (p) paths.push(p);
  if (node.children) {
    const sorted = Object.values(node.children).sort((a, b) => {
      if (a.type !== b.type) return a.type === 'dir' ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    for (const child of sorted) {
      paths.push(...collectAllPaths(child));
    }
  }
  return paths;
}

export function buildDiffMap(diff: DiffResult): Map<string, string> {
  const m = new Map<string, string>();
  for (const cs of diff.change_sets) {
    for (const p of cs.paths) {
      m.set(p, cs.type);
      const norm = p.replace(/^\.\//, '').replace(/^\//, '');
      m.set(norm, cs.type);
      if (norm.includes('/')) {
        const parentRel = norm.split('/').slice(1).join('/');
        if (parentRel) m.set(parentRel, cs.type);
      }
    }
  }
  return m;
}

export function buildTree(allNodes: FileNode[], diff: DiffResult | null): TreeNode {
  let nodes: TreeNode[] = allNodes;
  if (diff) {
    const existingPaths = new Set<string>();
    for (const n of nodes) {
      if (n.path) existingPaths.add(n.path.replace(/^\//, ''));
    }
    for (const cs of diff.change_sets) {
      if (cs.type !== 'added') continue;
      for (const p of cs.paths) {
        const norm = p.replace(/^\.\//, '').replace(/^\//, '');
        if (!norm || existingPaths.has(norm)) continue;
        nodes = [...nodes, {
          name: norm.split('/').pop() ?? norm,
          type: 'file',
          path: '/' + norm,
          full_path: p,
        }];
        existingPaths.add(norm);
      }
    }
  }
  const root: TreeNode = { name: '/', type: 'dir', path: '/', children: {} };
  for (const n of nodes) {
    if (!n.path || n.path === '/') continue;
    const parts = n.path.replace(/^\//, '').split('/');
    let cur = root;
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      if (!part || part === '') continue;
      if (i === parts.length - 1) {
        n.children = undefined;
        if (cur.children) cur.children[part] = n;
      } else {
        cur.children ??= {};
        let child = cur.children[part];
        if (!child) {
          child = { name: part, type: 'dir', path: '/' + parts.slice(0, i + 1).join('/'), children: {} };
          cur.children[part] = child;
        } else {
          child.children ??= {};
        }
        cur = child;
      }
    }
  }
  computeDirDiffTypes(root, diff ? buildDiffMap(diff) : null);
  return root;
}

function computeDirDiffTypes(node: TreeNode, dm: Map<string, string> | null): void {
  if (!node.children) return;
  for (const child of Object.values(node.children)) {
    if (child.children) computeDirDiffTypes(child, dm);
  }
  let result: string | null = null;
  let hasChildWithType = false;
  for (const child of Object.values(node.children)) {
    let childType: string | null = null;
    if (child.children) {
      childType = child.dirDiffType ?? null;
    } else {
      // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
      const t = dm?.get(child.full_path ?? '') ?? dm?.get((child.path ?? '').replace(/^\//, '')) ?? '';
      childType = t !== '' ? t : null;
    }
    if (childType === null) continue;
    hasChildWithType = true;
    if (result === null) {
      result = childType;
    } else if (result !== childType) {
      result = null;
      break;
    }
  }
  node.dirDiffType = hasChildWithType && result !== null ? result : undefined;
}
