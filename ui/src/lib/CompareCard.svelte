<script lang="ts">
  import { getFiles, getRun } from "./api";
  import { fmtTokens, fmtTps, genTokens } from "./fmt";
  import Preview from "./Preview.svelte";
  import StatusChip from "./StatusChip.svelte";
  import { lastAssistantText } from "./transcript";
  import type { FileEntry, Result, RunRef } from "./types";

  let {
    modelName,
    result,
    review,
  }: { modelName: string; result: Result; review: boolean } = $props();

  const ref = $derived<RunRef>({
    task: result.task,
    model: result.model,
    timestamp: result.timestamp,
  });

  let files = $state<FileEntry[]>([]);
  let finalText = $state("");
  let error = $state("");

  $effect(() => {
    const current = ref;
    files = [];
    finalText = "";
    error = "";
    (async () => {
      try {
        const [d, f] = await Promise.all([
          getRun(current),
          getFiles(current).catch(() => [] as FileEntry[]),
        ]);
        files = f;
        finalText = lastAssistantText(d.events);
      } catch (e) {
        error = e instanceof Error ? e.message : String(e);
      }
    })();
  });
</script>

<article class="card">
  <header>
    <h3 title={result.model}>{modelName}</h3>
    <a
      class="detail-link"
      href={"#/run/" +
        encodeURIComponent(result.task) +
        "/" +
        encodeURIComponent(result.model) +
        "/" +
        encodeURIComponent(result.timestamp)}
    >
      detail
    </a>
  </header>
  <div class="stats">
    <StatusChip status={result.status} />
    <span>{Math.round(result.duration_sec)}s</span>
    <span>{result.messages} msg</span>
    <span>{result.tool_calls} tools</span>
    <span>{fmtTokens(result.tokens_in)}→{fmtTokens(genTokens(result))} tok</span>
    <span>{fmtTps(result)}</span>
  </div>
  {#if error}
    <p class="error">{error}</p>
  {/if}
  {#if review}
    <Preview {ref} {files} />
  {/if}
  {#if finalText}
    <pre class="final">{finalText}</pre>
  {/if}
</article>

<style>
  .card {
    flex: 0 0 26rem;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 0.5rem;
  }
  h3 {
    margin: 0;
    font-size: 0.95rem;
    font-family: var(--mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .detail-link {
    font-size: 0.8rem;
    color: var(--muted);
  }
  .detail-link:hover {
    color: var(--fg);
  }
  .stats {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    flex-wrap: wrap;
    font-size: 0.8rem;
    color: var(--muted);
    font-family: var(--mono);
  }
  .final {
    white-space: pre-wrap;
    background: var(--panel-2);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.5rem 0.7rem;
    margin: 0;
    max-height: 18rem;
    overflow: auto;
    font-size: 0.8rem;
  }
  .error {
    color: var(--red);
    margin: 0;
  }
</style>
