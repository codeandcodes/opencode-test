<script lang="ts">
  import type { Rubric } from "./api";
  import { ratingBand } from "./fmt";

  let { rubric }: { rubric: Rubric | null } = $props();
</script>

{#if rubric}
  <details class="rubric">
    <summary>rating rubric</summary>
    <table>
      <tbody>
        {#each rubric.levels as level (level.rating)}
          <tr>
            <td class="num num-{ratingBand(level.rating)}">{level.rating}</td>
            <td class="band">{level.band}</td>
            <td>{level.description}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </details>
{/if}

<style>
  .rubric {
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--panel);
    font-size: 0.82rem;
  }
  .rubric summary {
    cursor: pointer;
    padding: 0.35rem 0.8rem;
    color: var(--muted);
    font-family: var(--mono);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  table {
    border-collapse: collapse;
    width: 100%;
  }
  td {
    padding: 0.3rem 0.8rem;
    border-top: 1px solid var(--border);
    vertical-align: top;
  }
  .num {
    font-family: var(--mono);
    width: 2rem;
    text-align: right;
  }
  .num-high {
    color: var(--green, #5cb8a0);
  }
  .num-mid {
    color: var(--accent);
  }
  .num-low {
    color: var(--red);
  }
  .band {
    color: var(--muted);
    white-space: nowrap;
  }
</style>
