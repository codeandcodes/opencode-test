import type { RunnerEvent } from "./types";

/**
 * Subscribe to the runner event stream. Reconnects automatically 2s after
 * an error closes the connection. Returns a function that closes the
 * subscription for good.
 */
export function subscribe(onEvent: (e: RunnerEvent) => void): () => void {
  let es: EventSource | null = null;
  let retry: ReturnType<typeof setTimeout> | null = null;
  let closed = false;

  function connect() {
    if (closed) return;
    es = new EventSource("/api/events");
    es.onmessage = (msg) => {
      try {
        onEvent(JSON.parse(msg.data) as RunnerEvent);
      } catch {
        // ignore malformed events
      }
    };
    es.onerror = () => {
      es?.close();
      es = null;
      if (!closed) retry = setTimeout(connect, 2000);
    };
  }

  connect();

  return () => {
    closed = true;
    if (retry !== null) clearTimeout(retry);
    es?.close();
    es = null;
  };
}
