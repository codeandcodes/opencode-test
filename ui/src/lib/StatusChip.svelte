<script lang="ts">
  let { status, running = false }: { status: string; running?: boolean } =
    $props();

  const label = $derived(
    running ? "running" : status === "done" ? "review" : status,
  );
  const cls = $derived(running ? "running" : status);
</script>

<span class="chip chip-{cls}">
  {#if running}<span class="spinner" aria-hidden="true"></span>{/if}
  {label}
</span>
