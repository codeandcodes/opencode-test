<script lang="ts">
  import { getMatrix, getModels, getTasks } from "./api";
  import CompareCard from "./CompareCard.svelte";
  import type { Matrix, Model, TaskSummary } from "./types";

  let models = $state<Model[]>([]);
  let tasks = $state<TaskSummary[]>([]);
  let matrix = $state<Matrix>({});
  let error = $state("");

  let taskID = $state("");
  // absent key = checked
  let selModels = $state<Record<string, boolean>>({});
  const isOn = (id: string) => selModels[id] ?? true;

  $effect(() => {
    (async () => {
      try {
        const [m, t, mx] = await Promise.all([
          getModels(),
          getTasks(),
          getMatrix(),
        ]);
        models = m;
        tasks = t.tasks;
        matrix = mx.matrix ?? {};
        if (!taskID && t.tasks.length > 0) taskID = t.tasks[0].id;
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      }
    })();
  });

  const task = $derived(tasks.find((t) => t.id === taskID));
  const columns = $derived(
    models
      .filter((m) => isOn(m.id))
      .map((m) => ({ model: m, result: matrix[taskID]?.[m.id] })),
  );
</script>

<div class="compare">
  <div class="controls">
    <label>
      Task
      <select bind:value={taskID}>
        {#each tasks as t (t.id)}
          <option value={t.id}>{t.title || t.id} ({t.type})</option>
        {/each}
      </select>
    </label>
    <fieldset>
      <legend>Models</legend>
      {#each models as m (m.id)}
        <label class="model">
          <input
            type="checkbox"
            checked={isOn(m.id)}
            onchange={(e) =>
              (selModels[m.id] = (e.currentTarget as HTMLInputElement).checked)}
          />
          {m.name || m.id}
        </label>
      {/each}
    </fieldset>
  </div>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  {#if task}
    <div class="columns">
      {#each columns as col (col.model.id)}
        {#if col.result}
          <CompareCard
            modelName={col.model.name || col.model.id}
            result={col.result}
            review={task.type === "review"}
          />
        {:else}
          <article class="card empty-card">
            <h3>{col.model.name || col.model.id}</h3>
            <p>no run yet</p>
          </article>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .compare {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .controls {
    display: flex;
    gap: 1rem;
    align-items: flex-start;
    flex-wrap: wrap;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
    font-size: 0.85rem;
  }
  .controls > label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--muted);
  }
  fieldset {
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.3rem 0.6rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.2rem 1rem;
    margin: 0;
  }
  legend {
    color: var(--muted);
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  label.model {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--fg);
    white-space: nowrap;
  }
  .columns {
    display: flex;
    gap: 0.75rem;
    overflow-x: auto;
    align-items: flex-start;
    padding-bottom: 0.5rem;
  }
  .empty-card {
    flex: 0 0 26rem;
    background: var(--panel);
    border: 1px dashed var(--border);
    border-radius: 8px;
    padding: 0.75rem;
    color: var(--muted);
  }
  .empty-card h3 {
    margin: 0 0 0.4rem;
    font-size: 0.95rem;
    font-family: var(--mono);
  }
  .empty-card p {
    margin: 0;
  }
  .error {
    color: var(--red);
  }
</style>
