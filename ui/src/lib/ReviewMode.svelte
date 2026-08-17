<script lang="ts">
  import {
    getMatrix,
    getModels,
    getReviewsPending,
    getTasks,
    previewUrl,
    setVerdict,
  } from "./api";
  import { getFiles, getRubric, type Rubric } from "./api";
  import RatingStrip from "./RatingStrip.svelte";
  import RubricPanel from "./RubricPanel.svelte";
  import type { Matrix, Model, RunRef, TaskSummary } from "./types";

  let pending = $state<RunRef[]>([]);
  let matrix = $state<Matrix>({});
  // task|model|ts -> whether the run's workspace has an index.html
  let hasApp = $state<Record<string, boolean>>({});
  let rubric = $state<Rubric | null>(null);

  $effect(() => {
    getRubric()
      .then((r) => (rubric = r))
      .catch(() => {});
  });
  let tasks = $state<TaskSummary[]>([]);
  let names = $state<Record<string, string>>({});
  let idx = $state(0);
  let gallery = $state(false);
  let note = $state("");
  let error = $state("");
  let loaded = $state(false);

  async function load() {
    try {
      const [p, mx, t, models] = await Promise.all([
        getReviewsPending(),
        getMatrix(),
        getTasks(),
        getModels(),
      ]);
      pending = p;
      matrix = mx.matrix ?? {};
      tasks = t.tasks;
      names = Object.fromEntries(models.map((m: Model) => [m.id, m.name]));
      idx = Math.min(idx, Math.max(0, p.length - 1));
      loaded = true;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
  $effect(() => {
    load();
  });

  const current = $derived(pending[idx] ?? null);

  function appKey(ref: RunRef): string {
    return `${ref.task}|${ref.model}|${ref.timestamp}`;
  }

  async function probeApp(ref: RunRef) {
    const key = appKey(ref);
    if (key in hasApp) return;
    try {
      const files = await getFiles(ref);
      hasApp[key] = files.some((f) => f.path === "index.html");
    } catch {
      hasApp[key] = false;
    }
  }

  $effect(() => {
    if (current) probeApp(current);
    for (const run of galleryRuns) {
      probeApp({ task: run.task, model: run.model, timestamp: run.timestamp });
    }
  });
  const taskTitle = $derived.by(() => {
    if (!current) return "";
    return tasks.find((t) => t.id === current.task)?.title || current.task;
  });
  // Every done run of the current task across models, judged or not, for
  // gallery comparison context.
  const galleryRuns = $derived.by(() => {
    if (!current) return [];
    const byModel = matrix[current.task] ?? {};
    return Object.values(byModel)
      .filter((r) => r.status === "done")
      .sort((a, b) => a.model.localeCompare(b.model));
  });

  async function judge(ref: RunRef, rating: number) {
    error = "";
    try {
      await setVerdict(ref, rating, note.trim());
      note = "";
      pending = pending.filter(
        (p) =>
          !(
            p.task === ref.task &&
            p.model === ref.model &&
            p.timestamp === ref.timestamp
          ),
      );
      if (idx >= pending.length) idx = Math.max(0, pending.length - 1);
      // refresh matrix so gallery badges update
      getMatrix().then((mx) => (matrix = mx.matrix ?? {}));
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  function skip() {
    if (pending.length > 0) idx = (idx + 1) % pending.length;
  }

  function onkey(e: KeyboardEvent) {
    const t = e.target as HTMLElement;
    if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA")) return;
    if (!current) return;
    if (e.key >= "1" && e.key <= "9") judge(current, Number(e.key));
    else if (e.key === "0") judge(current, 10);
    else if (e.key === "n") skip();
  }

  function verdictOf(taskID: string, model: string) {
    return matrix[taskID]?.[model]?.verdict;
  }
</script>

<svelte:window onkeydown={onkey} />

<div class="review">
  <header>
    <h2>Review</h2>
    {#if loaded}
      <span class="count">
        {pending.length} unjudged run{pending.length === 1 ? "" : "s"}
      </span>
    {/if}
    <label class="toggle">
      <input type="checkbox" bind:checked={gallery} />
      gallery (all models, current task)
    </label>
    <span class="keys">
      keys: <kbd>1</kbd>–<kbd>9</kbd> rate · <kbd>0</kbd> = 10 · <kbd>n</kbd> next
    </span>
  </header>

  {#if error}<p class="error">{error}</p>{/if}
  <RubricPanel {rubric} />

  {#if !loaded}
    <p class="muted">Loading…</p>
  {:else if !current}
    <p class="muted">
      Nothing to review — every completed review run has a verdict. 🎉
    </p>
  {:else if gallery}
    <h3>{taskTitle}</h3>
    <div class="grid">
      {#each galleryRuns as run (run.model)}
        {@const v = verdictOf(run.task, run.model)}
        <div class="card" class:judged={!!v}>
          <div class="card-head">
            <span class="model" title={run.model}>
              {names[run.model] ?? run.model}
            </span>
            {#if v}
              <span class="badge">
                {v.rating ? `${v.rating}/10` : v.verdict === "good" ? "👍" : "👎"}
              </span>
            {/if}
            <a
              class="link"
              href={"#/run/" +
                encodeURIComponent(run.task) +
                "/" +
                encodeURIComponent(run.model) +
                "/" +
                encodeURIComponent(run.timestamp)}
            >
              detail
            </a>
          </div>
          {#if hasApp[appKey({ task: run.task, model: run.model, timestamp: run.timestamp })]}
            <iframe
              title="{run.task} × {run.model}"
              sandbox="allow-scripts"
              src={previewUrl(
                { task: run.task, model: run.model, timestamp: run.timestamp },
                "index.html",
              )}
            ></iframe>
          {:else}
            <div class="no-app">
              No app produced — the agent wrote no index.html to its
              workspace.
            </div>
          {/if}
          {#if !v}
            <div class="card-actions">
              <RatingStrip
                compact
                {rubric}
                onrate={(rating) => judge(run, rating)}
              />
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {:else}
    <div class="queue">
      <div class="queue-head">
        <h3>
          {taskTitle}
          <span class="sep">×</span>
          {names[current.model] ?? current.model}
        </h3>
        <a
          class="link"
          href={"#/run/" +
            encodeURIComponent(current.task) +
            "/" +
            encodeURIComponent(current.model) +
            "/" +
            encodeURIComponent(current.timestamp)}
        >
          full detail
        </a>
      </div>
      {#if hasApp[appKey(current)]}
        <iframe
          class="main-preview"
          title="preview"
          sandbox="allow-scripts"
          src={previewUrl(current, "index.html")}
        ></iframe>
      {:else}
        <div class="no-app main-preview">
          No app produced — the agent wrote no index.html to its workspace.
          Check the transcript on the detail page to see what it did instead.
        </div>
      {/if}
      <div class="actions">
        <RatingStrip {rubric} onrate={(rating) => judge(current, rating)} />
        <button onclick={skip}>skip (n)</button>
        <input
          type="text"
          placeholder="note (optional, saved with rating)"
          bind:value={note}
        />
      </div>
    </div>
  {/if}
</div>

<style>
  .review {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  header {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }
  h2 {
    margin: 0;
    font-size: 1.05rem;
  }
  h3 {
    margin: 0;
    font-size: 0.95rem;
    font-family: var(--mono);
  }
  .sep {
    color: var(--muted);
  }
  .count {
    color: var(--muted);
    font-size: 0.85rem;
  }
  .toggle {
    font-size: 0.82rem;
    color: var(--muted);
    display: inline-flex;
    gap: 0.35rem;
    align-items: center;
  }
  .keys {
    margin-left: auto;
    color: var(--muted);
    font-size: 0.78rem;
  }
  kbd {
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 0 0.3rem;
    font-family: var(--mono);
  }
  .queue {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .queue-head {
    display: flex;
    align-items: baseline;
    gap: 1rem;
  }
  .main-preview {
    width: 100%;
    aspect-ratio: 16 / 10;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: #fff;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .actions button {
    font: inherit;
    padding: 0.3rem 0.9rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: var(--panel);
    color: var(--fg);
    cursor: pointer;
  }
  .actions button.good:hover {
    border-color: var(--green, #3fb950);
  }
  .actions button.bad:hover {
    border-color: var(--red);
  }
  .actions input {
    flex: 1;
    min-width: 16rem;
    font: inherit;
    font-size: 0.85rem;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(24rem, 1fr));
    gap: 0.8rem;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.6rem;
  }
  .card.judged {
    opacity: 0.75;
  }
  .card-head {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .model {
    font-family: var(--mono);
    font-size: 0.85rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badge {
    font-size: 0.9rem;
  }
  .link {
    margin-left: auto;
    font-size: 0.78rem;
    color: var(--muted);
  }
  .card iframe {
    width: 100%;
    aspect-ratio: 16 / 10;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: #fff;
  }
  .no-app {
    display: flex;
    align-items: center;
    justify-content: center;
    text-align: center;
    aspect-ratio: 16 / 10;
    border: 1px dashed var(--border);
    border-radius: 6px;
    color: var(--muted);
    font-size: 0.85rem;
    padding: 1rem;
    background: var(--well, #141210);
  }
  .card-actions {
    display: flex;
    gap: 0.5rem;
  }
  .card-actions button {
    font: inherit;
    font-size: 0.82rem;
    padding: 0.2rem 0.7rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
    cursor: pointer;
  }
  .error {
    color: var(--red);
  }
  .muted {
    color: var(--muted);
  }
</style>
