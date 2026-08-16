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
 * Generation speed in tokens/sec over model-active time (step windows
 * minus tool execution). Includes prompt processing; the first step also
 * includes model load. Null when the event stream carried no timing info.
 */
export function genTps(r: Result): number | null {
  const out = genTokens(r);
  if (!r.gen_seconds || r.gen_seconds <= 0 || out === 0) return null;
  return out / r.gen_seconds;
}

/** Color band for a 1-10 rating: high ≥7, low ≤3, mid between. */
export function ratingBand(rating: number): "high" | "mid" | "low" {
  if (rating >= 7) return "high";
  if (rating <= 3) return "low";
  return "mid";
}

/** "9✓" | "2✗" | "8✓ 1✗" — empty string when no counts were parsed. */
export function fmtCheckScore(r: Result): string {
  if (!r.check_parsed) return "";
  const parts: string[] = [];
  if ((r.check_passed ?? 0) > 0) parts.push(`${r.check_passed}✓`);
  if ((r.check_failed ?? 0) > 0) parts.push(`${r.check_failed}✗`);
  if (parts.length === 0) return r.status === "pass" ? "all✓" : "";
  return parts.join(" ");
}

export function fmtTps(r: Result): string {
  const tps = genTps(r);
  return tps === null
    ? "—"
    : `${tps >= 100 ? Math.round(tps) : tps.toFixed(1)} t/s`;
}
