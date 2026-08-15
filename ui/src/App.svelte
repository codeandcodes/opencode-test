<script lang="ts">
  import Compare from "./lib/Compare.svelte";
  import Matrix from "./lib/Matrix.svelte";
  import NewTask from "./lib/NewTask.svelte";
  import RunDetail from "./lib/RunDetail.svelte";
  import { subscribe } from "./lib/sse";
  import type { RunnerEvent } from "./lib/types";

  type Route =
    | { page: "matrix" }
    | { page: "run"; task: string; model: string; ts: string }
    | { page: "compare" }
    | { page: "new" };

  function parseRoute(h: string): Route {
    const p = h.replace(/^#/, "");
    if (p.startsWith("/run/")) {
      const parts = p.slice("/run/".length).split("/");
      if (parts.length >= 3) {
        return {
          page: "run",
          task: decodeURIComponent(parts[0]),
          model: decodeURIComponent(parts[1]),
          ts: decodeURIComponent(parts[2]),
        };
      }
    }
    if (p === "/compare") return { page: "compare" };
    if (p === "/new") return { page: "new" };
    return { page: "matrix" };
  }

  let hash = $state(typeof location !== "undefined" ? location.hash : "");
  const route = $derived(parseRoute(hash));

  // SSE: drive the nav progress chip and bump `refresh` so the matrix
  // refetches after every job.
  let refresh = $state(0);
  let live = $state<RunnerEvent | null>(null);

  $effect(() => {
    const close = subscribe((e) => {
      if (e.type === "batch_end") {
        live = null;
        refresh++;
      } else {
        live = e;
        if (e.type === "job_end") refresh++;
        else refresh++; // job_start: refetch to light up the running cell
      }
    });
    return close;
  });
</script>

<svelte:window onhashchange={() => (hash = location.hash)} />

<nav>
  <span class="brand">opencode-bench</span>
  <a href="#/" class:current={route.page === "matrix" || route.page === "run"}>
    Matrix
  </a>
  <a href="#/compare" class:current={route.page === "compare"}>Compare</a>
  <a href="#/new" class:current={route.page === "new"}>New Task</a>
  {#if live}
    <span class="live" title="{live.task} × {live.model}">
      <span class="spinner" aria-hidden="true"></span>
      {live.done}/{live.total}
      <span class="live-what">{live.task} × {live.model}</span>
    </span>
  {/if}
</nav>

<main>
  {#if route.page === "run"}
    {#key `${route.task}/${route.model}/${route.ts}`}
      <RunDetail task={route.task} model={route.model} ts={route.ts} />
    {/key}
  {:else if route.page === "compare"}
    <Compare />
  {:else if route.page === "new"}
    <NewTask />
  {:else}
    <Matrix {refresh} />
  {/if}
</main>

<style>
  nav {
    display: flex;
    align-items: center;
    gap: 1.1rem;
    padding: 0.55rem 1rem;
    border-bottom: 1px solid var(--border);
    background: var(--panel);
    position: sticky;
    top: 0;
    z-index: 10;
  }
  .brand {
    font-family: var(--mono);
    font-weight: 700;
    color: var(--accent);
    margin-right: 0.5rem;
  }
  nav a {
    color: var(--muted);
    text-decoration: none;
    font-size: 0.9rem;
    padding: 0.15rem 0;
    border-bottom: 2px solid transparent;
  }
  nav a:hover {
    color: var(--fg);
  }
  nav a.current {
    color: var(--fg);
    border-bottom-color: var(--accent);
  }
  .live {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-family: var(--mono);
    font-size: 0.82rem;
    background: var(--accent-dim);
    color: var(--accent);
    border: 1px solid var(--accent);
    border-radius: 999px;
    padding: 0.15rem 0.7rem;
    max-width: 40%;
  }
  .live-what {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
  }
  main {
    padding: 1rem;
    max-width: 100rem;
    margin: 0 auto;
  }
</style>
