<script lang="ts">
  import { cancelRuns, startRuns } from "./api";
  import type { Active, Model, TaskSummary } from "./types";

  let {
    models,
    tasks,
    active,
    onchanged,
  }: {
    models: Model[];
    tasks: TaskSummary[];
    active: Active;
    onchanged?: () => void;
  } = $props();

  // Selections default to "everything checked": absent key = checked, so
  // newly discovered ids appear checked automatically.
  let selModels = $state<Record<string, boolean>>({});
  let selTasks = $state<Record<string, boolean>>({});

  const isOn = (sel: Record<string, boolean>, id: string) => sel[id] ?? true;
  function toggle(sel: Record<string, boolean>, id: string, e: Event) {
    sel[id] = (e.currentTarget as HTMLInputElement).checked;
  }

  let busy = $state(false);
  let error = $state("");
  let force = $state(false);
  let lastQueued = $state("");

  async function run() {
    const ms = models.filter((m) => isOn(selModels, m.id)).map((m) => m.id);
    const ts = tasks.filter((t) => isOn(selTasks, t.id)).map((t) => t.id);
    busy = true;
    error = "";
    lastQueued = "";
    try {
      const res = await startRuns(ms, ts, force);
      lastQueued =
        res.jobs === 0
          ? `nothing to do — ${res.skipped} cell${res.skipped === 1 ? "" : "s"} already complete (check "re-run completed" for more samples)`
          : `queued ${res.jobs}${res.skipped ? ` · skipped ${res.skipped} already complete` : ""}`;
      onchanged?.();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      busy = false;
    }
  }

  async function cancel() {
    error = "";
    try {
      await cancelRuns();
      onchanged?.();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    }
  }
</script>

<section class="runcontrol">
  <div class="pickers">
    <fieldset>
      <legend>Models</legend>
      {#each models as m (m.id)}
        <label>
          <input
            type="checkbox"
            checked={isOn(selModels, m.id)}
            onchange={(e) => toggle(selModels, m.id, e)}
          />
          {m.name || m.id}
        </label>
      {/each}
    </fieldset>
    <fieldset>
      <legend>Tasks</legend>
      {#each tasks as t (t.id)}
        <label>
          <input
            type="checkbox"
            checked={isOn(selTasks, t.id)}
            onchange={(e) => toggle(selTasks, t.id, e)}
          />
          {t.title || t.id}
        </label>
      {/each}
    </fieldset>
  </div>
  <div class="actions">
    <button
      type="button"
      class="primary"
      onclick={run}
      disabled={active.running || busy}
    >
      Run
    </button>
    <button type="button" onclick={cancel} disabled={!active.running}>
      Cancel
    </button>
    <label class="force">
      <input type="checkbox" bind:checked={force} />
      re-run completed (extra samples)
    </label>
    {#if active.running}
      <span class="progress">
        <span class="spinner" aria-hidden="true"></span>
        {active.done}/{active.total}
        {#if active.task}
          <span class="current">{active.task} × {active.model}</span>
        {/if}
      </span>
    {/if}
    {#if lastQueued}
      <span class="queued">{lastQueued}</span>
    {/if}
    {#if error}
      <span class="error">{error}</span>
    {/if}
  </div>
</section>

<style>
  .runcontrol {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .pickers {
    display: flex;
    gap: 1rem;
    flex-wrap: wrap;
  }
  fieldset {
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.4rem 0.6rem;
    min-width: 12rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 1rem;
  }
  legend {
    color: var(--muted);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 0 0.3rem;
  }
  label {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.85rem;
    white-space: nowrap;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .progress {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-family: var(--mono);
    font-size: 0.85rem;
  }
  .current {
    color: var(--muted);
  }
  .error {
    color: var(--red);
    font-size: 0.85rem;
  }
  .force {
    color: var(--muted);
    font-size: 0.82rem;
  }
  .queued {
    color: var(--muted);
    font-size: 0.82rem;
    font-style: italic;
  }
</style>
