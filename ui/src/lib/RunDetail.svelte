<script lang="ts">
  import { clearVerdict, getFiles, getHistory, getRun, setVerdict } from "./api";
  import { fmtCheckScore, fmtTokens, fmtTps, ratingBand } from "./fmt";
  import RatingStrip from "./RatingStrip.svelte";
  import FileTree from "./FileTree.svelte";
  import Preview from "./Preview.svelte";
  import StatusChip from "./StatusChip.svelte";
  import Transcript from "./Transcript.svelte";
  import type { FileEntry, Result, RunDetailResponse, RunRef } from "./types";

  let { task, model, ts }: { task: string; model: string; ts: string } =
    $props();

  const ref = $derived<RunRef>({ task, model, timestamp: ts });

  let detail = $state<RunDetailResponse | null>(null);
  let files = $state<FileEntry[]>([]);
  let history = $state<Result[]>([]);
  let error = $state("");

  $effect(() => {
    const current = ref;
    detail = null;
    files = [];
    error = "";
    (async () => {
      try {
        const [d, f, h] = await Promise.all([
          getRun(current),
          getFiles(current).catch(() => [] as FileEntry[]),
          getHistory(current.task, current.model).catch(() => [] as Result[]),
        ]);
        detail = d;
        files = f;
        history = h;
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      }
    })();
  });

  function switchRun(e: Event) {
    const sel = (e.currentTarget as HTMLSelectElement).value;
    location.hash = `#/run/${encodeURIComponent(task)}/${encodeURIComponent(model)}/${encodeURIComponent(sel)}`;
  }

  const result = $derived(detail?.result ?? null);

  let verdictNote = $state("");
  let verdictError = $state("");

  async function rate(rating: number) {
    verdictError = "";
    try {
      const saved = await setVerdict(ref, rating, verdictNote.trim());
      if (detail) detail = { ...detail, result: { ...detail.result, verdict: saved } };
    } catch (e) {
      verdictError = e instanceof Error ? e.message : String(e);
    }
  }

  async function unjudge() {
    verdictError = "";
    try {
      await clearVerdict(ref);
      if (detail) {
        const { verdict: _, ...rest } = detail.result;
        detail = { ...detail, result: rest };
      }
    } catch (e) {
      verdictError = e instanceof Error ? e.message : String(e);
    }
  }
</script>

<div class="rundetail">
  <header>
    <a class="back" href="#/">← matrix</a>
    <h2>{task} <span class="sep">×</span> {model}</h2>
    {#if history.length > 0}
      <label class="history">
        run
        <select value={ts} onchange={switchRun}>
          {#each history as h (h.timestamp)}
            <option value={h.timestamp}>
              {h.timestamp} ({h.status})
            </option>
          {/each}
        </select>
      </label>
    {/if}
  </header>

  {#if error}
    <p class="error">{error}</p>
  {:else if !detail}
    <p class="muted">Loading…</p>
  {:else if result}
    <div class="stats">
      <StatusChip status={result.status} />
      <span class="stat">
        <span class="k">duration</span>
        {Math.round(result.duration_sec)}s
      </span>
      <span class="stat"><span class="k">messages</span> {result.messages}</span>
      <span class="stat">
        <span class="k">tool calls</span>
        {result.tool_calls}
      </span>
      <span class="stat">
        <span class="k">tokens</span>
        {fmtTokens(result.tokens_in)} in
        {#if result.cache_read > 0}({fmtTokens(result.cache_read)} cached){/if}
        / {fmtTokens(result.tokens_out)} out
        {#if result.tokens_reasoning > 0}
          (+{fmtTokens(result.tokens_reasoning)} think)
        {/if}
      </span>
      {#if fmtCheckScore(result)}
        <span class="stat">
          <span class="k">assertions</span>
          {fmtCheckScore(result)}
        </span>
      {/if}
      <span class="stat">
        <span class="k">gen speed</span>
        {fmtTps(result)}
        {#if result.gen_seconds > 0}
          <span class="k">over {Math.round(result.gen_seconds)}s</span>
        {/if}
      </span>
      {#if (result.load_seconds ?? 0) > 0.5}
        <span class="stat">
          <span class="k">first token</span>
          {Math.round(result.load_seconds ?? 0)}s
        </span>
      {/if}
      {#if result.error}
        <span class="stat error">{result.error}</span>
      {/if}
    </div>

    <div class="verdict-bar">
      {#if result.verdict}
        {#if result.verdict.rating}
          <span class="verdict rated rated-{ratingBand(result.verdict.rating)}">
            {result.verdict.rating}/10
          </span>
        {:else}
          <span class="verdict">
            {result.verdict.verdict === "good" ? "👍 good" : "👎 bad"}
            <span class="k">(legacy — re-rate below)</span>
          </span>
        {/if}
        {#if result.verdict.note}<span class="note">{result.verdict.note}</span>{/if}
        <button class="linkish" onclick={unjudge}>clear</button>
      {:else}
        <span class="k">rate this result — 1 non-functional · 10 perfect:</span>
        <RatingStrip onrate={rate} />
        <input
          type="text"
          placeholder="note (optional)"
          bind:value={verdictNote}
        />
      {/if}
      {#if verdictError}<span class="error">{verdictError}</span>{/if}
    </div>

    {#if detail.check_log}
      <section>
        <h3>check log</h3>
        <pre class="checklog">{detail.check_log}</pre>
      </section>
    {/if}

    <Preview {ref} {files} />

    <section>
      <h3>transcript</h3>
      <Transcript events={detail.events} searchable />
    </section>

    <section>
      <h3>workspace</h3>
      <FileTree {ref} {files} />
    </section>

    {#if detail.provenance}
      <section>
        <h3>provenance</h3>
        <div class="prov">
          <div>
            <span class="k">prompt sha</span>
            <code>{detail.provenance.prompt_sha.slice(0, 12)}</code>
            <span class="k">captured</span>
            {detail.provenance.captured_at}
          </div>
          {#if detail.provenance.llama_swap_entry}
            <pre class="checklog">{JSON.stringify(
                detail.provenance.llama_swap_entry,
                null,
                2,
              )}</pre>
          {/if}
        </div>
      </section>
    {/if}
  {/if}
</div>

<style>
  .rundetail {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  header {
    display: flex;
    align-items: baseline;
    gap: 1rem;
    flex-wrap: wrap;
  }
  h2 {
    margin: 0;
    font-size: 1.05rem;
    font-family: var(--mono);
  }
  .sep {
    color: var(--muted);
  }
  .back {
    color: var(--muted);
    text-decoration: none;
    font-size: 0.85rem;
  }
  .back:hover {
    color: var(--fg);
  }
  .history {
    margin-left: auto;
    color: var(--muted);
    font-size: 0.85rem;
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
  }
  .stats {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.5rem 0.8rem;
    font-size: 0.88rem;
  }
  .stat .k {
    color: var(--muted);
    font-size: 0.75rem;
    margin-right: 0.25rem;
  }
  h3 {
    margin: 0 0 0.4rem;
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }
  .verdict-bar {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.45rem 0.8rem;
    font-size: 0.85rem;
  }
  .verdict-bar .k {
    color: var(--muted);
    font-size: 0.78rem;
  }
  .verdict-bar button {
    font: inherit;
    padding: 0.15rem 0.6rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: var(--panel-2, transparent);
    color: var(--fg);
    cursor: pointer;
  }
  .verdict-bar button:hover {
    border-color: var(--fg);
  }
  .verdict-bar button.linkish {
    border: none;
    background: none;
    color: var(--muted);
    text-decoration: underline;
    padding: 0;
  }
  .verdict-bar input {
    font: inherit;
    font-size: 0.82rem;
    padding: 0.15rem 0.5rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
    min-width: 14rem;
  }
  .verdict {
    font-weight: 600;
  }
  .rated {
    font-family: var(--mono);
    background: var(--well, #141210);
    border-radius: 5px;
    padding: 0.15rem 0.55rem;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
  }
  .rated-high {
    color: var(--green, #5cb8a0);
  }
  .rated-mid {
    color: var(--accent);
  }
  .rated-low {
    color: var(--red);
  }
  .verdict-bar .note {
    color: var(--muted);
    font-style: italic;
  }
  .checklog {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.8rem;
    margin: 0;
    max-height: 20rem;
    overflow: auto;
    font-size: 0.8rem;
  }
  .error {
    color: var(--red);
  }
  .muted {
    color: var(--muted);
  }
</style>
