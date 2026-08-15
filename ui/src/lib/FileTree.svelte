<script lang="ts">
  import { getFileText } from "./api";
  import type { FileEntry, RunRef } from "./types";

  let { ref, files }: { ref: RunRef; files: FileEntry[] } = $props();

  interface Node {
    name: string;
    path: string;
    dir: boolean;
    size: number;
    children: Node[];
  }

  function buildTree(entries: FileEntry[]): Node[] {
    const root: Node[] = [];
    const byPath = new Map<string, Node>();
    const sorted = [...entries].sort((a, b) => a.path.localeCompare(b.path));
    const attach = (node: Node) => {
      const idx = node.path.lastIndexOf("/");
      if (idx === -1) {
        root.push(node);
        return;
      }
      const parentPath = node.path.slice(0, idx);
      let parent = byPath.get(parentPath);
      if (!parent) {
        parent = {
          name: parentPath.slice(parentPath.lastIndexOf("/") + 1),
          path: parentPath,
          dir: true,
          size: 0,
          children: [],
        };
        byPath.set(parentPath, parent);
        attach(parent);
      }
      parent.children.push(node);
    };
    for (const e of sorted) {
      if (byPath.has(e.path)) continue;
      const node: Node = {
        name: e.path.slice(e.path.lastIndexOf("/") + 1),
        path: e.path,
        dir: e.dir,
        size: e.size,
        children: [],
      };
      byPath.set(e.path, node);
      attach(node);
    }
    return root;
  }

  const nodes = $derived(buildTree(files));

  let selected = $state("");
  let content = $state("");
  let loadError = $state("");

  async function open(path: string) {
    selected = path;
    content = "";
    loadError = "";
    try {
      content = await getFileText(ref, path);
    } catch (e) {
      loadError = e instanceof Error ? e.message : String(e);
    }
  }

  function fmtSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
  }
</script>

{#snippet tree(list: Node[])}
  <ul class="tree">
    {#each list as node (node.path)}
      <li>
        {#if node.dir}
          <span class="dirname">{node.name}</span>
          {@render tree(node.children)}
        {:else}
          <button
            type="button"
            class="file"
            class:selected={selected === node.path}
            onclick={() => open(node.path)}
          >
            {node.name}
            <span class="size">{fmtSize(node.size)}</span>
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{/snippet}

<div class="filetree">
  <div class="pane tree-pane">
    {#if files.length === 0}
      <p class="empty">Workspace is empty.</p>
    {:else}
      {@render tree(nodes)}
    {/if}
  </div>
  {#if selected}
    <div class="pane viewer">
      <div class="viewer-head">{selected}</div>
      {#if loadError}
        <p class="error">{loadError}</p>
      {:else}
        <pre><code>{content}</code></pre>
      {/if}
    </div>
  {/if}
</div>

<style>
  .filetree {
    display: grid;
    grid-template-columns: minmax(180px, 240px) 1fr;
    gap: 0.75rem;
    align-items: start;
  }
  .filetree:not(:has(.viewer)) {
    grid-template-columns: 1fr;
  }
  .pane {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.5rem;
    min-width: 0;
  }
  ul.tree {
    list-style: none;
    margin: 0;
    padding-left: 0;
  }
  ul.tree ul.tree {
    padding-left: 1rem;
    border-left: 1px solid var(--border);
    margin-left: 0.3rem;
  }
  .dirname {
    display: block;
    padding: 0.15rem 0.3rem;
    color: var(--muted);
    font-weight: 600;
  }
  .dirname::before {
    content: "▸ ";
  }
  button.file {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    width: 100%;
    text-align: left;
    background: none;
    border: 0;
    color: var(--fg);
    font: inherit;
    padding: 0.15rem 0.3rem;
    border-radius: 4px;
    cursor: pointer;
  }
  button.file:hover {
    background: var(--panel-2);
  }
  button.file.selected {
    background: var(--accent-dim);
    color: var(--accent);
  }
  .size {
    color: var(--muted);
    font-size: 0.75rem;
    white-space: nowrap;
  }
  .viewer pre {
    margin: 0;
    max-height: 32rem;
    overflow: auto;
    font-size: 0.8rem;
  }
  .viewer-head {
    font-family: var(--mono);
    font-size: 0.8rem;
    color: var(--muted);
    border-bottom: 1px solid var(--border);
    padding-bottom: 0.3rem;
    margin-bottom: 0.4rem;
  }
  .empty {
    color: var(--muted);
    margin: 0.3rem;
  }
  .error {
    color: var(--red);
  }
</style>
