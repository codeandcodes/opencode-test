import { describe, expect, it } from "vitest";
import { fmtTokens, fmtTps, genTps } from "./fmt";
import type { Result } from "./types";

function result(partial: Partial<Result>): Result {
  return {
    task: "t",
    model: "m",
    status: "done",
    started_at: "",
    finished_at: "",
    duration_sec: 100,
    messages: 1,
    tool_calls: 0,
    tokens_in: 0,
    tokens_out: 0,
    tokens_reasoning: 0,
    cache_read: 0,
    gen_seconds: 0,
    timestamp: "ts",
    ...partial,
  };
}

describe("fmtTokens", () => {
  it("formats magnitudes", () => {
    expect(fmtTokens(950)).toBe("950");
    expect(fmtTokens(1500)).toBe("1.5k");
    expect(fmtTokens(12345)).toBe("12k");
    expect(fmtTokens(2_500_000)).toBe("2.5M");
  });
});

describe("genTps", () => {
  it("includes reasoning tokens over generation seconds", () => {
    const r = result({
      tokens_out: 100,
      tokens_reasoning: 100,
      gen_seconds: 4,
    });
    expect(genTps(r)).toBe(50);
    expect(fmtTps(r)).toBe("50.0 t/s");
  });
  it("is null without timing or output", () => {
    expect(genTps(result({ tokens_out: 100, gen_seconds: 0 }))).toBeNull();
    expect(genTps(result({ tokens_out: 0, gen_seconds: 5 }))).toBeNull();
    expect(fmtTps(result({}))).toBe("—");
  });
  it("rounds three-digit speeds", () => {
    const r = result({ tokens_out: 1000, gen_seconds: 8 });
    expect(fmtTps(r)).toBe("125 t/s");
  });
});
