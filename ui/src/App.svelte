<script lang="ts">
  import Compare from "./lib/Compare.svelte";
  import Leaderboard from "./lib/Leaderboard.svelte";
  import Matrix from "./lib/Matrix.svelte";
  import ReviewMode from "./lib/ReviewMode.svelte";
  import NewTask from "./lib/NewTask.svelte";
  import Reports from "./lib/Reports.svelte";
  import RunDetail from "./lib/RunDetail.svelte";
  import { getReviewsPending } from "./lib/api";
  import { subscribe } from "./lib/sse";
  import type { RunnerEvent } from "./lib/types";

  type Route =
    | { page: "matrix" }
    | { page: "run"; task: string; model: string; ts: string }
    | { page: "compare" }
    | { page: "leaderboard" }
    | { page: "review" }
    | { page: "reports"; slug?: string }
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
    if (p === "/leaderboard") return { page: "leaderboard" };
    if (p === "/review") return { page: "review" };
    if (p.startsWith("/reports/")) {
      return {
        page: "reports",
        slug: decodeURIComponent(p.slice("/reports/".length)),
      };
    }
    if (p === "/reports") return { page: "reports" };
    if (p === "/new") return { page: "new" };
    return { page: "matrix" };
  }

  let hash = $state(typeof location !== "undefined" ? location.hash : "");
  const route = $derived(parseRoute(hash));

  // SSE: drive the nav progress chip and bump `refresh` so the matrix
  // refetches after every job.
  let refresh = $state(0);
  let live = $state<RunnerEvent | null>(null);
  let pendingReviews = $state(0);

  $effect(() => {
    void refresh;
    void hash; // recount when navigating (verdicts change the queue)
    getReviewsPending()
      .then((p) => (pendingReviews = p.length))
      .catch(() => {});
  });

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
  <span class="brand">
    <span class="power-lamp" class:on={!!live} aria-hidden="true"></span>
    opencode<span class="brand-dim">-bench</span>
  </span>
  <a href="#/" class:current={route.page === "matrix" || route.page === "run"}>
    Matrix
  </a>
  <a href="#/compare" class:current={route.page === "compare"}>Compare</a>
  <a href="#/review" class:current={route.page === "review"}>
    Review{#if pendingReviews > 0}<span class="nav-badge">{pendingReviews}</span>{/if}
  </a>
  <a href="#/leaderboard" class:current={route.page === "leaderboard"}>
    Leaderboard
  </a>
  <a href="#/reports" class:current={route.page === "reports"}>Reports</a>
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
  {:else if route.page === "leaderboard"}
    <Leaderboard />
  {:else if route.page === "review"}
    <ReviewMode />
  {:else if route.page === "reports"}
    {#key route.slug ?? ""}
      <Reports slug={route.slug} />
    {/key}
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
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-family: var(--mono);
    font-weight: 500;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    font-size: 0.8rem;
    color: var(--fg);
    margin-right: 0.8rem;
  }
  .brand-dim {
    color: var(--muted);
  }
  .power-lamp {
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 50%;
    background: var(--grey);
    transition: background 300ms ease;
  }
  .power-lamp.on {
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent);
  }
  .nav-badge {
    display: inline-block;
    margin-left: 0.35rem;
    padding: 0 0.4rem;
    border-radius: 999px;
    background: var(--well, #141210);
    border: 1px solid var(--border);
    color: var(--accent);
    font-family: var(--mono);
    font-size: 0.7rem;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
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
