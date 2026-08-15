<script lang="ts">
  import {
    dismissResumable,
    getMatrix,
    getModels,
    getResumable,
    getTasks,
    resumeBatch,
    type ResumableBatch,
  } from "./api";
  import { fmtTokens, fmtTps, genTokens } from "./fmt";
  import RunControl from "./RunControl.svelte";
  import StatusChip from "./StatusChip.svelte";
  import type {
    Active,
    Matrix as MatrixData,
    Model,
    TaskSummary,
  } from "./types";

  let { refresh = 0 }: { refresh?: number } = $props();

  let models = $state<Model[]>([]);
  let tasks = $state<TaskSummary[]>([]);
  let warnings = $state<string[]>([]);
  let matrix = $state<MatrixData>({});
  let active = $state<Active>({
    running: false,
    task: "",
    model: "",
    done: 0,
    total: 0,
  });
  let error = $state("");
  let resumable = $state<ResumableBatch | null>(null);
  let resumeMsg = $state("");

  async function load() {
    try {
      const [m, t, mx, res] = await Promise.all([
        getModels(),
        getTasks(),
        getMatrix(),
        getResumable(),
      ]);
      models = m;
      tasks = t.tasks;
      warnings = t.warnings ?? [];
      matrix = mx.matrix ?? {};
      active = mx.active;
      resumable = res;
      error = "";
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }

  async function resume() {
    resumeMsg = "";
    try {
      const r = await resumeBatch();
      resumeMsg = `resumed: queued ${r.jobs}${r.skipped ? `, skipped ${r.skipped}` : ""}${r.dropped ? `, dropped ${r.dropped}` : ""}`;
      resumable = null;
      load();
    } catch (e) {
      resumeMsg = e instanceof Error ? e.message : String(e);
    }
  }

  async function dismiss() {
    try {
      await dismissResumable();
      resumable = null;
    } catch (e) {
      resumeMsg = e instanceof Error ? e.message : String(e);
    }
  }

  $effect(() => {
    void refresh; // refetch whenever the SSE-driven counter bumps
    load();
  });

  // Group tasks by category, preserving backend order.
  const groups = $derived.by(() => {
    const out: { category: string; tasks: TaskSummary[] }[] = [];
    const idx = new Map<string, number>();
    for (const t of tasks) {
      const key = t.category || "uncategorized";
      let i = idx.get(key);
      if (i === undefined) {
        i = out.length;
        idx.set(key, i);
        out.push({ category: key, tasks: [] });
      }
      out[i].tasks.push(t);
    }
    return out;
  });

  function isRunning(taskID: string, modelID: string): boolean {
    return (
      active.running && active.task === taskID && active.model === modelID
    );
  }

  function fmtDuration(sec: number): string {
    return `${Math.round(sec)}s`;
  }
</script>

<div class="matrix-page">
  {#if resumable}
    <div class="resume-banner">
      <span>
        Interrupted batch: <strong>{resumable.count}</strong> pending job{resumable.count ===
        1
          ? ""
          : "s"}
      </span>
      <button type="button" class="primary" onclick={resume}>Resume</button>
      <button type="button" onclick={dismiss}>Dismiss</button>
    </div>
  {/if}
  {#if resumeMsg}
    <p class="resume-msg">{resumeMsg}</p>
  {/if}
  <RunControl {models} {tasks} {active} onchanged={load} />

  {#if error}
    <p class="error">{error}</p>
  {/if}
  {#each warnings as w (w)}
    <p class="warning">task warning: {w}</p>
  {/each}

  {#if tasks.length > 0 && models.length > 0}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th class="taskcol">Task</th>
            {#each models as m (m.id)}
              <th title={m.id}>{m.name || m.id}</th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each groups as group (group.category)}
            <tr class="category-row">
              <td colspan={models.length + 1}>{group.category}</td>
            </tr>
            {#each group.tasks as t (t.id)}
              <tr>
                <td class="taskcol">
                  <span class="task-title">{t.title || t.id}</span>
                  <span class="badge badge-{t.type}">{t.type}</span>
                </td>
                {#each models as m (m.id)}
                  {@const r = matrix[t.id]?.[m.id]}
                  <td class="cell">
                    {#if isRunning(t.id, m.id)}
                      <StatusChip status="running" running />
                    {:else if r}
                      <a
                        href={"#/run/" +
                          encodeURIComponent(t.id) +
                          "/" +
                          encodeURIComponent(m.id) +
                          "/" +
                          encodeURIComponent(r.timestamp)}
                      >
                        <StatusChip status={r.status} />
                        {#if r.stale}
                          <span
                            class="stale"
                            title="task prompt changed since this run"
                          >
                            Δ
                          </span>
                        {/if}
                        {#if r.verdict}
                          <span
                            class="verdict-icon"
                            title={r.verdict.note || r.verdict.verdict}
                          >
                            {r.verdict.verdict === "good" ? "👍" : "👎"}
                          </span>
                        {/if}
                        <span class="dur">{fmtDuration(r.duration_sec)}</span>
                        {#if genTokens(r) > 0}
                          <span
                            class="substats"
                            title={`${r.tokens_in} in / ${r.tokens_out} out` +
                              (r.tokens_reasoning
                                ? ` / ${r.tokens_reasoning} think`
                                : "")}
                          >
                            {fmtTokens(r.tokens_in)}→{fmtTokens(genTokens(r))}
                            · {fmtTps(r)}
                          </span>
                        {/if}
                      </a>
                    {:else}
                      <span class="nodata">—</span>
                    {/if}
                  </td>
                {/each}
              </tr>
            {/each}
          {/each}
        </tbody>
      </table>
    </div>
  {:else if !error}
    <p class="empty">No models or tasks discovered yet.</p>
  {/if}
</div>

<style>
  .matrix-page {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel);
  }
  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.85rem;
  }
  th,
  td {
    padding: 0.4rem 0.7rem;
    text-align: left;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  th {
    color: var(--muted);
    font-weight: 600;
    background: var(--panel-2);
  }
  .category-row td {
    background: var(--panel-2);
    color: var(--muted);
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    padding: 0.25rem 0.7rem;
  }
  .taskcol {
    min-width: 12rem;
  }
  .task-title {
    margin-right: 0.5rem;
  }
  .cell a {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    text-decoration: none;
    color: inherit;
  }
  .cell a:hover .dur {
    text-decoration: underline;
  }
  .dur {
    color: var(--muted);
    font-family: var(--mono);
    font-size: 0.78rem;
  }
  .verdict-icon {
    font-size: 0.8rem;
  }
  .resume-banner {
    display: flex;
    align-items: center;
    gap: 0.8rem;
    background: var(--panel);
    border: 1px solid var(--orange, #d29922);
    border-radius: 8px;
    padding: 0.5rem 0.8rem;
    font-size: 0.88rem;
  }
  .resume-banner button {
    font: inherit;
    padding: 0.15rem 0.7rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
    cursor: pointer;
  }
  .resume-banner button.primary {
    border-color: var(--orange, #d29922);
  }
  .resume-msg {
    color: var(--muted);
    font-size: 0.85rem;
    font-style: italic;
    margin: 0;
  }
  .stale {
    color: var(--orange, #d29922);
    font-weight: 700;
    cursor: help;
  }
  .substats {
    display: block;
    color: var(--muted);
    font-family: var(--mono);
    font-size: 0.68rem;
    margin-top: 0.15rem;
    white-space: nowrap;
  }
  .nodata {
    color: var(--border);
  }
  .badge {
    font-size: 0.65rem;
    padding: 0.1rem 0.35rem;
    border-radius: 999px;
    border: 1px solid var(--border);
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .badge-review {
    border-color: var(--blue);
    color: var(--blue);
  }
  .badge-check {
    border-color: var(--green);
    color: var(--green);
  }
  .error {
    color: var(--red);
  }
  .warning {
    color: var(--orange);
    font-size: 0.8rem;
    margin: 0;
  }
  .empty {
    color: var(--muted);
  }
</style>
