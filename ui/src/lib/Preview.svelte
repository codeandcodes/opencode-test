<script lang="ts">
  import { previewUrl } from "./api";
  import type { FileEntry, RunRef } from "./types";

  let { ref, files }: { ref: RunRef; files: FileEntry[] } = $props();

  const hasIndex = $derived(
    files.some((f) => !f.dir && f.path === "index.html"),
  );
  const url = $derived(previewUrl(ref, "index.html"));
</script>

{#if hasIndex}
  <div class="preview">
    <div class="preview-bar">
      <span>Preview</span>
      <a href={url} target="_blank" rel="noopener noreferrer">
        open in new tab
      </a>
    </div>
    {#key url}
      <iframe sandbox="allow-scripts" src={url} title="workspace preview"
      ></iframe>
    {/key}
  </div>
{/if}

<style>
  .preview {
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
    background: var(--panel);
  }
  .preview-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.3rem 0.6rem;
    border-bottom: 1px solid var(--border);
    font-size: 0.8rem;
    color: var(--muted);
  }
  iframe {
    display: block;
    width: 100%;
    aspect-ratio: 16 / 10;
    border: 0;
    background: #fff;
  }
</style>
