<script lang="ts">
  import { itemMatches, parseItems } from "./transcript";

  let { events, searchable = false }: { events: unknown[]; searchable?: boolean } =
    $props();

  let query = $state("");
  const items = $derived(parseItems(events));
  const shown = $derived(items.filter((it) => itemMatches(it, query)));
</script>

<div class="transcript">
  {#if searchable && items.length > 3}
    <input
      class="search"
      type="search"
      placeholder="search transcript…"
      bind:value={query}
    />
  {/if}
  {#each shown as item, i (i)}
    {#if item.kind === "text"}
      <pre class="text">{item.text}</pre>
    {:else if item.kind === "reasoning"}
      <details class="reasoning">
        <summary>thinking ({item.text.length} chars)</summary>
        <pre>{item.text}</pre>
      </details>
    {:else}
      <details class="tool">
        <summary>
          <span class="tool-name">{item.name}</span>
          {#if item.title}<span class="tool-title">{item.title}</span>{/if}
        </summary>
        {#each item.sections as section (section.label)}
          <div class="section">
            <span class="label">{section.label}</span>
            <pre class={section.tone ?? ""}>{section.text}</pre>
          </div>
        {/each}
      </details>
    {/if}
  {:else}
    <p class="empty">
      {query ? "No events match the search." : "No events recorded."}
    </p>
  {/each}
</div>

<style>
  .transcript {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .search {
    font: inherit;
    font-size: 0.85rem;
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
    max-width: 24rem;
  }
  pre.text {
    white-space: pre-wrap;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.6rem 0.8rem;
    margin: 0;
  }
  details {
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--panel-2);
  }
  details.tool {
    border-left: 3px solid var(--accent);
  }
  details.reasoning {
    border-left: 3px solid var(--muted);
  }
  details summary {
    cursor: pointer;
    padding: 0.4rem 0.8rem;
    font-family: var(--mono);
    font-size: 0.85rem;
  }
  details.tool summary {
    color: var(--accent);
  }
  details.reasoning summary {
    color: var(--muted);
  }
  .tool-title {
    color: var(--muted);
    margin-left: 0.5rem;
    font-size: 0.8rem;
  }
  .section {
    border-top: 1px solid var(--border);
  }
  .section .label {
    display: block;
    padding: 0.25rem 0.8rem 0;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
  }
  .section pre,
  details.reasoning pre {
    margin: 0;
    padding: 0.4rem 0.8rem 0.6rem;
    overflow-x: auto;
    font-size: 0.8rem;
    white-space: pre-wrap;
  }
  .section pre.del {
    border-left: 3px solid var(--red);
    background: color-mix(in srgb, var(--red) 6%, transparent);
  }
  .section pre.ins {
    border-left: 3px solid var(--green, #3fb950);
    background: color-mix(in srgb, var(--green, #3fb950) 6%, transparent);
  }
  .empty {
    color: var(--muted);
  }
</style>
