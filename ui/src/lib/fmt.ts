// Shared formatting helpers for run statistics.
import type { Result } from "./types";

export function fmtTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 10_000) return Math.round(n / 1000) + "k";
  if (n >= 1_000) return (n / 1000).toFixed(1) + "k";
  return String(n);
}

/** Generated tokens: visible output plus reasoning/thinking tokens. */
export function genTokens(r: Result): number {
  return (r.tokens_out ?? 0) + (r.tokens_reasoning ?? 0);
}

/**
 * Generation speed in tokens/sec over active generation time (text and
 * reasoning part windows), excluding tool execution and model load.
 * Null when the event stream carried no timing info.
 */
export function genTps(r: Result): number | null {
  const out = genTokens(r);
  if (!r.gen_seconds || r.gen_seconds <= 0 || out === 0) return null;
  return out / r.gen_seconds;
}

export function fmtTps(r: Result): string {
  const tps = genTps(r);
  return tps === null
    ? "—"
    : `${tps >= 100 ? Math.round(tps) : tps.toFixed(1)} t/s`;
}
