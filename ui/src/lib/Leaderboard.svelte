<script lang="ts">
  import { getLeaderboard, getModels, type LeaderboardRow } from "./api";
  import type { Model } from "./types";

  let rows = $state<LeaderboardRow[]>([]);
  let names = $state<Record<string, string>>({});
  let error = $state("");

  $effect(() => {
    (async () => {
      try {
        const [r, models] = await Promise.all([getLeaderboard(), getModels()]);
        rows = r;
        names = Object.fromEntries(models.map((m: Model) => [m.id, m.name]));
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      }
    })();
  });

  function pct(row: LeaderboardRow): string {
    if (row.check_cells === 0) return "—";
    return `${Math.round((100 * row.check_cells_passed) / row.check_cells)}%`;
  }
</script>

<div class="board">
  <h2>Leaderboard</h2>
  {#if error}
    <p class="error">{error}</p>
  {:else if rows.length === 0}
    <p class="muted">No completed runs yet.</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Model</th>
            <th title="check cells passed / attempted">Checks</th>
            <th>Pass rate</th>
            <th title="review cells with a completed build">Reviews built</th>
            <th title="your verdicts on review runs">Verdicts</th>
            <th title="median generation speed across cells">t/s (median)</th>
            <th title="cells whose latest run hit an infrastructure failure">Errors</th>
            <th>Samples</th>
          </tr>
        </thead>
        <tbody>
          {#each rows as row (row.model)}
            <tr>
              <td class="model" title={row.model}>{names[row.model] ?? row.model}</td>
              <td>{row.check_cells_passed}/{row.check_cells}</td>
              <td class="pct">{pct(row)}</td>
              <td>{row.reviews_done}</td>
              <td>
                {#if row.verdict_good}👍{row.verdict_good}{/if}
                {#if row.verdict_bad}👎{row.verdict_bad}{/if}
                {#if !row.verdict_good && !row.verdict_bad}<span class="muted">—</span>{/if}
              </td>
              <td>{row.median_tps > 0 ? row.median_tps.toFixed(1) : "—"}</td>
              <td class={row.errors > 0 ? "warn" : ""}>{row.errors}</td>
              <td>{row.samples}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <p class="muted note">
      Ranked by check cells passed. Review columns need your verdicts (👍/👎
      on run pages) to become comparable.
    </p>
  {/if}
</div>

<style>
  .board {
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  h2 {
    margin: 0;
    font-size: 1.05rem;
  }
  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--border);
    border-radius: 8px;
  }
  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.88rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.5rem 0.8rem;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  th {
    color: var(--muted);
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    background: var(--panel);
  }
  tbody tr:last-child td {
    border-bottom: none;
  }
  .model {
    font-family: var(--mono);
  }
  .pct {
    font-weight: 600;
  }
  .warn {
    color: var(--orange, #d29922);
  }
  .muted {
    color: var(--muted);
  }
  .note {
    font-size: 0.8rem;
    margin: 0;
  }
  .error {
    color: var(--red);
  }
</style>
