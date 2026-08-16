<script lang="ts">
  import { getActiveTail, getFiles, previewUrl, type ActiveTail } from "./api";
  import { fmtTokens } from "./fmt";
  import Transcript from "./Transcript.svelte";

  let tail = $state<ActiveTail | null>(null);
  let showTranscript = $state(false);
  let showBuild = $state(false);
  let hasIndex = $state(false);
  let previewSrc = $state("");

  $effect(() => {
    let stop = false;
    async function poll() {
      while (!stop) {
        tail = await getActiveTail();
        await new Promise((r) => setTimeout(r, 5000));
      }
    }
    poll();
    return () => {
      stop = true;
    };
  });

  // Watch-it-build: while the job runs, check for index.html and refresh
  // the preview every 10s (cache-busted so the iframe reloads).
  $effect(() => {
    if (!showBuild) return;
    let stop = false;
    async function refreshPreview() {
      while (!stop) {
        const t = tail;
        if (t) {
          const ref = { task: t.task, model: t.model, timestamp: t.timestamp };
          try {
            const files = await getFiles(ref);
            hasIndex = files.some((f) => f.path === "index.html");
            if (hasIndex) {
              previewSrc = previewUrl(ref, "index.html") + "?t=" + Date.now();
            }
          } catch {
            hasIndex = false;
          }
        }
        await new Promise((r) => setTimeout(r, 10000));
      }
    }
    refreshPreview();
    return () => {
      stop = true;
    };
  });

  function ageClass(sec: number): string {
    if (!Number.isFinite(sec) || sec < 0) return "age-unknown";
    if (sec < 60) return "age-fresh";
    if (sec < 300) return "age-slow";
    return "age-stuck";
  }

  function ageLabel(sec: number): string {
    if (!Number.isFinite(sec) || sec < 0) return "no events yet";
    if (sec < 60) return `active ${Math.round(sec)}s ago`;
    return `quiet for ${Math.round(sec / 60)}m`;
  }
</script>

{#if tail}
  <div class="live">
    <div class="row">
      <span class="spinner" aria-hidden="true"></span>
      <strong>{tail.task}</strong>
      <span class="sep">×</span>
      <span>{tail.model}</span>
      <span class={"age " + ageClass(tail.last_event_age_sec)}>
        {ageLabel(tail.last_event_age_sec)}
      </span>
      <span class="stat well">{tail.steps ?? 0} steps</span>
      <span class="stat well">{tail.tool_calls ?? 0} tools</span>
      <span class="stat well">{fmtTokens(tail.tokens_out ?? 0)} tok out</span>
      <button
        type="button"
        class="toggle"
        onclick={() => (showBuild = !showBuild)}
      >
        {showBuild ? "hide" : "watch"} it build
      </button>
      <button
        type="button"
        class="toggle"
        onclick={() => (showTranscript = !showTranscript)}
      >
        {showTranscript ? "hide" : "show"} live transcript
      </button>
    </div>
    {#if showBuild}
      {#if hasIndex && previewSrc}
        <iframe
          class="build-preview"
          title="live build preview"
          sandbox="allow-scripts"
          src={previewSrc}
        ></iframe>
        <span class="preview-note">
          refreshes every 10s — the app may be half-built
        </span>
      {:else}
        <span class="preview-note">no index.html in the workspace yet…</span>
      {/if}
    {/if}
    {#if showTranscript}
      <div class="transcript">
        <Transcript events={tail.recent} />
      </div>
    {/if}
  </div>
{/if}

<style>
  .live {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.5rem 0.8rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    flex-wrap: wrap;
    font-size: 0.88rem;
  }
  .sep {
    color: var(--muted);
  }
  .stat {
    color: var(--muted);
    font-family: var(--mono);
    font-size: 0.8rem;
  }
  .age {
    font-size: 0.8rem;
    padding: 0.05rem 0.5rem;
    border-radius: 999px;
    border: 1px solid var(--border);
  }
  .age-fresh {
    color: var(--green, #3fb950);
    border-color: var(--green, #3fb950);
  }
  .age-slow {
    color: var(--orange, #d29922);
    border-color: var(--orange, #d29922);
  }
  .age-stuck {
    color: var(--red);
    border-color: var(--red);
  }
  .age-unknown {
    color: var(--muted);
  }
  .toggle {
    font: inherit;
    font-size: 0.8rem;
    background: none;
    border: none;
    color: var(--muted);
    text-decoration: underline;
    cursor: pointer;
  }
  .toggle:first-of-type {
    margin-left: auto;
  }
  .build-preview {
    width: 100%;
    aspect-ratio: 16 / 10;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: #fff;
  }
  .preview-note {
    color: var(--muted);
    font-size: 0.75rem;
    font-style: italic;
  }
  .transcript {
    max-height: 20rem;
    overflow: auto;
    border-top: 1px solid var(--border);
    padding-top: 0.5rem;
  }
  .spinner {
    width: 0.8rem;
    height: 0.8rem;
    border: 2px solid var(--border);
    border-top-color: var(--fg);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
