<script lang="ts">
  import { extractText, isMessage, isTool, pretty, toolName } from "./transcript";

  let { events }: { events: unknown[] } = $props();

  type Item =
    | { kind: "text"; text: string }
    | { kind: "tool"; name: string; body: string };

  const items = $derived(
    events.flatMap((e): Item[] => {
      if (isTool(e)) {
        return [{ kind: "tool", name: toolName(e), body: pretty(e) }];
      }
      if (isMessage(e)) {
        const text = extractText(e);
        if (text !== "") return [{ kind: "text", text }];
      }
      return [];
    }),
  );
</script>

<div class="transcript">
  {#each items as item, i (i)}
    {#if item.kind === "text"}
      <pre class="text">{item.text}</pre>
    {:else}
      <details class="tool">
        <summary>{item.name}</summary>
        <pre>{item.body}</pre>
      </details>
    {/if}
  {:else}
    <p class="empty">No events recorded.</p>
  {/each}
</div>

<style>
  .transcript {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  pre.text {
    white-space: pre-wrap;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.8rem;
    margin: 0;
  }
  details.tool {
    border: 1px solid var(--border);
    border-left: 3px solid var(--accent);
    border-radius: 6px;
    background: var(--panel-2);
  }
  details.tool summary {
    cursor: pointer;
    padding: 0.4rem 0.8rem;
    font-family: var(--mono);
    font-size: 0.85rem;
    color: var(--accent);
  }
  details.tool pre {
    margin: 0;
    padding: 0.6rem 0.8rem;
    border-top: 1px solid var(--border);
    overflow-x: auto;
    font-size: 0.8rem;
  }
  .empty {
    color: var(--muted);
  }
</style>
