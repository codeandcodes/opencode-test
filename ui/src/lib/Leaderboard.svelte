<script lang="ts">
  import { getLeaderboard, getModels, type LeaderboardRow } from "./api";
  import type { Model } from "./types";

  let rows = $state<LeaderboardRow[]>([]);
  let names = $state<Record<string, string>>({});
  let error = $state("");

  type SortKey =
    | "score"
    | "model"
    | "checks"
    | "rating"
    | "reviews_done"
    | "median_tps"
    | "errors"
    | "samples";
  let sortKey = $state<SortKey>("score");
  let sortDesc = $state(true);

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

  function keyValue(row: LeaderboardRow, key: SortKey): number | string {
    switch (key) {
      case "model":
        return (names[row.model] ?? row.model).toLowerCase();
      case "checks":
        return row.check_cells > 0
          ? row.check_cells_passed / row.check_cells
          : -1;
      case "rating":
        return row.rating_count > 0 ? row.rating_avg : -1;
      default:
        return row[key];
    }
  }

  const sorted = $derived.by(() => {
    const out = [...rows];
    out.sort((a, b) => {
      const av = keyValue(a, sortKey);
      const bv = keyValue(b, sortKey);
      const cmp =
        typeof av === "string"
          ? av.localeCompare(bv as string)
          : (av as number) - (bv as number);
      return sortDesc ? -cmp : cmp;
    });
    return out;
  });

  function setSort(key: SortKey) {
    if (sortKey === key) {
      sortDesc = !sortDesc;
    } else {
      sortKey = key;
      sortDesc = key !== "model";
    }
  }

  function arrow(key: SortKey): string {
    if (sortKey !== key) return "";
    return sortDesc ? " ▾" : " ▴";
  }

  function pct(row: LeaderboardRow): string {
    if (row.check_cells === 0) return "—";
    return `${Math.round((100 * row.check_cells_passed) / row.check_cells)}%`;
  }

  function basisLabel(row: LeaderboardRow): string {
    switch (row.score_basis) {
      case "checks+ratings":
        return "50% check pass rate + 50% rating";
      case "checks":
        return "check pass rate only (no ratings yet)";
      case "ratings":
        return "ratings only (no check results)";
      default:
        return "no data";
    }
  }
</script>

<div class="board">
  <h2>Leaderboard</h2>
  {#if error}
    <p class="error">{error}</p>
  {:else if rows.length === 0}
    <p class="muted">No completed runs yet — start a batch from the Matrix.</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th class="rank">#</th>
            <th><button onclick={() => setSort("model")}>Model{arrow("model")}</button></th>
            <th title="0-100: half objective check pass rate, half your 1-10 ratings rescaled">
              <button onclick={() => setSort("score")}>Score{arrow("score")}</button>
            </th>
            <th title="check cells passed / attempted">
              <button onclick={() => setSort("checks")}>Checks{arrow("checks")}</button>
            </th>
            <th title="average of your 1-10 ratings">
              <button onclick={() => setSort("rating")}>Rating{arrow("rating")}</button>
            </th>
            <th title="review cells with a completed build">
              <button onclick={() => setSort("reviews_done")}>
                Built{arrow("reviews_done")}
              </button>
            </th>
            <th title="median generation speed across cells">
              <button onclick={() => setSort("median_tps")}>t/s{arrow("median_tps")}</button>
            </th>
            <th title="cells whose latest run hit an infrastructure failure">
              <button onclick={() => setSort("errors")}>Errors{arrow("errors")}</button>
            </th>
            <th><button onclick={() => setSort("samples")}>Samples{arrow("samples")}</button></th>
          </tr>
        </thead>
        <tbody>
          {#each sorted as row, i (row.model)}
            <tr class:leader={i === 0 && sortKey === "score" && sortDesc}>
              <td class="rank">{i + 1}</td>
              <td class="model" title={row.model}>{names[row.model] ?? row.model}</td>
              <td class="scorecell" title={basisLabel(row)}>
                {#if row.score_basis}
                  <span class="score-num">{row.score.toFixed(1)}</span>
                  {#if row.score_basis !== "checks+ratings"}
                    <span class="partial">*</span>
                  {/if}
                  <span class="meter" aria-hidden="true">
                    <span class="meter-fill" style="width: {row.score}%"></span>
                  </span>
                {:else}
                  <span class="muted">—</span>
                {/if}
              </td>
              <td>
                {row.check_cells_passed}/{row.check_cells}
                <span class="muted">({pct(row)})</span>
              </td>
              <td>
                {#if row.rating_count > 0}
                  {row.rating_avg.toFixed(1)}
                  <span class="muted">n={row.rating_count}</span>
                {:else}
                  <span class="muted">—</span>
                {/if}
              </td>
              <td>{row.reviews_done}</td>
              <td>{row.median_tps > 0 ? row.median_tps.toFixed(1) : "—"}</td>
              <td class={row.errors > 0 ? "warn" : ""}>{row.errors}</td>
              <td>{row.samples}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <p class="muted note">
      Score = 50% check pass rate + 50% average rating rescaled to 0-100
      (* = only one axis has data). Click any column to sort.
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
    padding: 0.55rem 0.8rem;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
  }
  th {
    background: var(--panel);
    padding: 0.3rem 0.5rem;
  }
  th button {
    background: none;
    border: none;
    padding: 0.2rem 0.3rem;
    color: var(--muted);
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    cursor: pointer;
  }
  th button:hover {
    color: var(--fg);
  }
  tbody tr:last-child td {
    border-bottom: none;
  }
  tr.leader td {
    background: var(--accent-dim);
  }
  tr.leader td:first-child {
    border-left: 3px solid var(--accent);
  }
  .rank {
    color: var(--muted);
    font-family: var(--mono);
    width: 2rem;
  }
  .model {
    font-family: var(--mono);
  }
  .scorecell {
    min-width: 9rem;
  }
  .score-num {
    font-family: var(--mono);
    font-size: 1.05rem;
    color: var(--accent);
    background: var(--well, #141210);
    border-radius: 4px;
    padding: 0.1rem 0.45rem;
    box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.45);
  }
  .partial {
    color: var(--muted);
  }
  .meter {
    display: block;
    margin-top: 0.3rem;
    height: 3px;
    width: 100%;
    max-width: 8rem;
    background: var(--well, #141210);
    border-radius: 2px;
    overflow: hidden;
  }
  .meter-fill {
    display: block;
    height: 100%;
    background: var(--accent);
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
