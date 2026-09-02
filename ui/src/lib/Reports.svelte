<script lang="ts">
  import { getReport, getReports } from "./api";
  import { renderReport } from "./reports";
  import type { ReportDetail, ReportMeta } from "./types";

  let { slug = "" }: { slug?: string } = $props();

  let list = $state<ReportMeta[]>([]);
  let detail = $state<ReportDetail | null>(null);
  let error = $state("");
  let loaded = $state(false);

  $effect(() => {
    const current = slug;
    error = "";
    detail = null;
    loaded = false;
    (async () => {
      try {
        if (current) {
          detail = await getReport(current);
        } else {
          list = await getReports();
        }
        loaded = true;
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      }
    })();
  });

  const rendered = $derived(detail ? renderReport(detail.markdown) : "");

  function fmtDate(iso: string): string {
    const d = new Date(iso);
    return isNaN(d.getTime()) ? "" : d.toISOString().slice(0, 10);
  }
</script>

<div class="reports">
  {#if error}
    <p class="error">{error}</p>
  {:else if slug}
    <header>
      <a class="back" href="#/reports">← reports</a>
    </header>
    {#if !loaded}
      <p class="muted">Loading…</p>
    {:else if detail}
      <!-- eslint-disable-next-line svelte/no-at-html-tags — reports are authored in-repo -->
      <article class="prose">{@html rendered}</article>
    {/if}
  {:else}
    <header>
      <h2>Research reports</h2>
      {#if loaded}
        <span class="count">{list.length} report{list.length === 1 ? "" : "s"}</span>
      {/if}
    </header>
    {#if !loaded}
      <p class="muted">Loading…</p>
    {:else if list.length === 0}
      <p class="muted">
        No reports yet. Each benched model gets one — see PIPELINE.md for the
        cycle that produces them.
      </p>
    {:else}
      <ul class="cards">
        {#each list as r (r.slug)}
          <li>
            <a class="card" href={"#/reports/" + encodeURIComponent(r.slug)}>
              <span class="title">{r.title}</span>
              <span class="meta">
                <code>{r.slug}</code>
                {#if fmtDate(r.updated_at)}· {fmtDate(r.updated_at)}{/if}
              </span>
            </a>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</div>

<style>
  .reports {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  header {
    display: flex;
    align-items: baseline;
    gap: 1rem;
  }
  h2 {
    margin: 0;
    font-size: 1.05rem;
  }
  .count {
    color: var(--muted);
    font-size: 0.85rem;
  }
  .back {
    color: var(--muted);
    text-decoration: none;
    font-size: 0.85rem;
  }
  .back:hover {
    color: var(--fg);
  }
  .cards {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(22rem, 1fr));
    gap: 0.8rem;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.8rem 1rem;
    text-decoration: none;
    color: var(--fg);
  }
  .card:hover {
    border-color: var(--accent);
  }
  .title {
    font-weight: 600;
  }
  .meta {
    color: var(--muted);
    font-size: 0.78rem;
  }
  .prose {
    max-width: 72rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1.2rem 1.6rem;
    line-height: 1.6;
    font-size: 0.92rem;
  }
  .prose :global(h1) {
    font-size: 1.3rem;
    margin: 0.2rem 0 0.8rem;
  }
  .prose :global(h2) {
    font-size: 1.05rem;
    margin: 1.4rem 0 0.5rem;
  }
  .prose :global(h3) {
    font-size: 0.95rem;
    margin: 1.1rem 0 0.4rem;
  }
  .prose :global(img) {
    max-width: 100%;
    border: 1px solid var(--border);
    border-radius: 6px;
  }
  .prose :global(table) {
    border-collapse: collapse;
    font-size: 0.85rem;
    margin: 0.6rem 0;
  }
  .prose :global(th),
  .prose :global(td) {
    border: 1px solid var(--border);
    padding: 0.3rem 0.6rem;
    text-align: left;
  }
  .prose :global(th) {
    color: var(--muted);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .prose :global(code) {
    font-family: var(--mono);
    font-size: 0.85em;
    background: var(--well, #141210);
    border-radius: 4px;
    padding: 0.08rem 0.3rem;
  }
  .prose :global(pre) {
    background: var(--well, #141210);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.8rem;
    overflow-x: auto;
  }
  .prose :global(pre code) {
    background: none;
    padding: 0;
  }
  .prose :global(blockquote) {
    margin: 0.6rem 0;
    padding: 0.2rem 0.9rem;
    border-left: 3px solid var(--accent);
    color: var(--muted);
  }
  .prose :global(a) {
    color: var(--accent);
  }
  .error {
    color: var(--red);
  }
  .muted {
    color: var(--muted);
  }
</style>
