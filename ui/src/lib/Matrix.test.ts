import { render, screen } from "@testing-library/svelte";
import { beforeEach, expect, it, vi } from "vitest";
import Matrix from "./Matrix.svelte";
import { jsonRes, matrixResp, models, tasksResp } from "./fixtures";

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/models")) return jsonRes(models);
      if (url.startsWith("/api/tasks")) return jsonRes(tasksResp);
      if (url.startsWith("/api/runs")) return jsonRes(matrixResp);
      return jsonRes({ error: "not found" }, 404);
    }),
  );
});

it("renders status chips from the matrix", async () => {
  render(Matrix);
  await screen.findByText("pass");
  // done on a review task renders as a "review" chip
  expect(document.querySelector(".chip-done")!.textContent).toContain("review");
  expect(document.querySelector(".chip-pass")!.textContent).toContain("pass");
  expect(document.querySelector(".chip-fail")!.textContent).toContain("fail");
});

it("links cells to the run detail route with the result timestamp", async () => {
  render(Matrix);
  const chip = await screen.findByText("pass");
  const link = chip.closest("a");
  expect(link).not.toBeNull();
  expect(link!.getAttribute("href")).toBe(
    "#/run/race-fix/m1/2026-08-15T10-00-00Z",
  );
});

it("shows rounded durations next to chips", async () => {
  render(Matrix);
  expect(await screen.findByText("62s")).toBeInTheDocument();
});
