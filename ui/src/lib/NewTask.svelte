<script lang="ts">
  import { createTask } from "./api";
  import type { Task } from "./types";

  let id = $state("");
  let title = $state("");
  let category = $state("");
  let type = $state<"review" | "check">("review");
  let timeoutMinutes = $state(30);
  let prompt = $state("");
  let check = $state("");

  let error = $state("");
  let saving = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    error = "";
    saving = true;
    const t: Task = {
      id,
      title,
      category,
      type,
      timeout_minutes: Number(timeoutMinutes),
      prompt,
    };
    if (type === "check") t.check = check;
    try {
      await createTask(t);
      location.hash = "#/";
    } catch (err) {
      error = err instanceof Error ? err.message : String(err);
    } finally {
      saving = false;
    }
  }
</script>

<form class="newtask" onsubmit={submit}>
  <h2>New task</h2>

  <div class="row">
    <label for="nt-id">ID</label>
    <input
      id="nt-id"
      bind:value={id}
      placeholder="lowercase-with-dashes"
      autocomplete="off"
    />
  </div>

  <div class="row">
    <label for="nt-title">Title</label>
    <input id="nt-title" bind:value={title} autocomplete="off" />
  </div>

  <div class="row">
    <label for="nt-category">Category</label>
    <input
      id="nt-category"
      bind:value={category}
      placeholder="games, algorithms, …"
      autocomplete="off"
    />
  </div>

  <div class="row">
    <label for="nt-type">Type</label>
    <select id="nt-type" bind:value={type}>
      <option value="review">review</option>
      <option value="check">check</option>
    </select>
  </div>

  <div class="row">
    <label for="nt-timeout">Timeout (minutes)</label>
    <input id="nt-timeout" type="number" min="1" bind:value={timeoutMinutes} />
  </div>

  <div class="row">
    <label for="nt-prompt">Prompt</label>
    <textarea id="nt-prompt" rows="12" bind:value={prompt}></textarea>
  </div>

  {#if type === "check"}
    <div class="row">
      <label for="nt-check">Check script</label>
      <textarea
        id="nt-check"
        rows="8"
        bind:value={check}
        placeholder="bash; exit 0 = pass"
      ></textarea>
    </div>
  {/if}

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <div class="actions">
    <button type="submit" class="primary" disabled={saving}>
      Create task
    </button>
    <a href="#/">cancel</a>
  </div>
</form>

<style>
  .newtask {
    max-width: 46rem;
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 1rem;
  }
  h2 {
    margin: 0 0 0.3rem;
    font-size: 1.05rem;
  }
  .row {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  label {
    font-size: 0.78rem;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  textarea {
    font-family: var(--mono);
    font-size: 0.82rem;
    resize: vertical;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 1rem;
    margin-top: 0.3rem;
  }
  .actions a {
    color: var(--muted);
    font-size: 0.85rem;
  }
  .error {
    color: var(--red);
    margin: 0;
    font-size: 0.88rem;
  }
</style>
