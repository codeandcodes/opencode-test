<script lang="ts">
  import type { Rubric } from "./api";
  import { ratingBand } from "./fmt";

  let {
    onrate,
    compact = false,
    rubric = null,
  }: {
    onrate: (rating: number) => void;
    compact?: boolean;
    rubric?: Rubric | null;
  } = $props();

  const ratings = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

  function hint(n: number): string {
    const level = rubric?.levels.find((l) => l.rating === n);
    if (level) return `${n} — ${level.band}: ${level.description}`;
    if (n === 1) return "1 — completely non-functional";
    if (n === 10) return "10 — perfect result";
    return String(n);
  }
</script>

<div class="strip" class:compact>
  {#each ratings as n (n)}
    <button
      type="button"
      class="r r-{ratingBand(n)}"
      title={hint(n)}
      onclick={() => onrate(n)}
    >
      {n}
    </button>
  {/each}
</div>

<style>
  .strip {
    display: inline-flex;
    gap: 0.2rem;
  }
  .r {
    font-family: var(--mono);
    font-size: 0.82rem;
    min-width: 1.9rem;
    padding: 0.25rem 0;
    text-align: center;
    background: var(--well, #141210);
    border: 1px solid var(--border);
    border-radius: 5px;
    color: var(--muted);
    cursor: pointer;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
  }
  .compact .r {
    min-width: 1.6rem;
    font-size: 0.75rem;
    padding: 0.15rem 0;
  }
  .r:hover {
    color: var(--fg);
  }
  .r-high:hover {
    border-color: var(--green, #5cb8a0);
    color: var(--green, #5cb8a0);
  }
  .r-mid:hover {
    border-color: var(--accent);
    color: var(--accent);
  }
  .r-low:hover {
    border-color: var(--red);
    color: var(--red);
  }
</style>
