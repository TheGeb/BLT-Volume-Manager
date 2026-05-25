<script lang="ts">
  export let volumes: string[] = [];
  export let loading = false;
  export let onSelect: (vol: string) => void = () => {};
  export let filter = '';
  export let onFilterChange: (f: string) => void;

  let filterVal = '';
  $: filterVal = filter;

  function handleInput() {
    onFilterChange(filterVal);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') { filterVal = ''; onFilterChange(''); }
  }

  interface TreeNode {
    name: string;
    path: string;
    children?: TreeNode[];
  }

  $: tree = buildTree(volumes, filter);

  function buildTree(names: string[], filterText: string): TreeNode[] {
    const filtered = filterText
      ? names.filter(n => n.toLowerCase().includes(filterText.toLowerCase()))
      : names;

    const root: Record<string, TreeNode & { _leaf: boolean; _children: Record<string, any> }> = {};
    for (const fullPath of filtered) {
      const parts = fullPath.split('/');
      let level = root;
      for (let i = 0; i < parts.length; i++) {
        const part = parts[i];
        const isLast = i === parts.length - 1;
        if (!level[part]) {
          level[part] = { _leaf: false, _children: {}, name: part, path: fullPath };
        }
        if (isLast) level[part]._leaf = true;
        level = level[part]._children;
      }
    }

    function toNodes(
      obj: Record<string, TreeNode & { _leaf: boolean; _children: Record<string, any> }>,
      prefix: string
    ): TreeNode[] {
      return Object.keys(obj).sort().map(k => {
        const full = prefix ? `${prefix}/${k}` : k;
        const n = obj[k];
        const childNodes = toNodes(n._children, full);
        return { name: k, path: full, children: childNodes.length > 0 ? childNodes : undefined };
      });
    }

    return toNodes(root, '');
  }

  let expanded: Record<string, boolean> = {};

  $: if (filter) {
    const all: Record<string, boolean> = {};
    function expandAll(nodes: TreeNode[]) {
      for (const n of nodes) {
        if (n.children) {
          all[n.path] = true;
          expandAll(n.children);
        }
      }
    }
    expandAll(tree);
    expanded = all;
  } else {
    expanded = {};
  }

  function toggle(path: string) {
    if (filter) return;
    expanded = { ...expanded, [path]: !expanded[path] };
  }

  interface FlatItem {
    name: string;
    path: string;
    depth: number;
    isGroup: boolean;
  }

  $: flatItems = flatten(tree, expanded, 0);

  function flatten(nodes: TreeNode[], exp: Record<string, boolean>, depth: number): FlatItem[] {
    const items: FlatItem[] = [];
    for (const node of nodes) {
      const isGroup = !!node.children;
      items.push({ name: node.name, path: node.path, depth, isGroup });
      if (isGroup && exp[node.path]) {
        items.push(...flatten(node.children!, exp, depth + 1));
      }
    }
    return items;
  }
</script>

<section class="panel" style="display:block;">
  {#if !loading || volumes.length > 0}
    <div class="filter-row">
      <input class="volume-filter-input" type="search" placeholder="Filter volumes..."
        bind:value={filterVal} on:input={handleInput} on:keydown={handleKeydown} />
    </div>
  {/if}

  {#if loading && volumes.length === 0}
    <div class="grid">
      {#each { length: 6 } as _}
        <div class="skeleton" style="height:80px;border-radius:16px;"></div>
      {/each}
    </div>
  {:else if volumes.length === 0 && !filter}
    <p class="empty">No volumes found</p>
  {:else if flatItems.length === 0 && filter}
    <p class="empty">No volumes match "{filter}"</p>
  {:else}
    <div class="tree">
      {#each flatItems as item (item.path)}
        <div class="tree-row" style="padding-left:{20 + item.depth * 20}px;">
          {#if item.isGroup}
            <button class="tree-group" on:click={() => toggle(item.path)} title={item.path}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="folder-icon">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
              <span class="tree-name">{item.name}</span>
              <svg width="10" height="10" viewBox="0 0 24 24" fill="currentColor" class="chevron"
                style="transform:rotate({expanded[item.path] ? 0 : -90}deg);opacity:0.5;">
                <polygon points="6 3 20 12 6 21 6 3"/>
              </svg>
            </button>
          {:else}
            <button class="tree-volume" on:click={() => onSelect(item.path)} title={item.path}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="volume-icon">
                <ellipse cx="12" cy="5" rx="9" ry="3"/>
                <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/>
                <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/>
              </svg>
              <span class="tree-name">{item.name}</span>
            </button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .filter-row {
    padding: 0 0 16px;
  }

  .volume-filter-input {
    width: 100%;
    box-sizing: border-box;
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.04);
    color: var(--text);
    padding: 10px 16px;
    border-radius: 999px;
    font-size: 0.9rem;
    font-weight: 500;
    outline: none;
    transition: border-color 0.15s;
  }

  .volume-filter-input:focus {
    border-color: var(--accent);
  }

  .volume-filter-input::placeholder {
    color: var(--muted);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 12px;
  }

  .empty {
    color: var(--muted);
    text-align: center;
    padding: 40px;
    margin: 0;
  }

  .tree {
    display: flex;
    flex-direction: column;
  }

  .tree-row {
    display: flex;
    align-items: center;
    min-height: 36px;
  }

  .tree-group, .tree-volume {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    border: none;
    border-radius: 8px;
    background: none;
    color: var(--text);
    font-size: 0.9rem;
    font-family: inherit;
    cursor: pointer;
    text-align: left;
    width: 100%;
    transition: background 0.1s;
  }

  .tree-group:hover, .tree-volume:hover {
    background: rgba(255, 255, 255, 0.06);
  }

  .tree-name {
    font-weight: 500;
  }

  .folder-icon, .volume-icon {
    flex-shrink: 0;
    opacity: 0.7;
  }

  .chevron {
    flex-shrink: 0;
    margin-left: auto;
    transition: transform 0.15s;
  }
</style>
