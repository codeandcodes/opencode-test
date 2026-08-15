import { render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import RunControl from "./RunControl.svelte";
import { idleActive, jsonRes, models, tasksResp } from "./fixtures";

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn(async () => jsonRes({ jobs: 4 }, 202));
  vi.stubGlobal("fetch", fetchMock);
});

it("posts all selected model and task ids on Run", async () => {
  const user = userEvent.setup();
  render(RunControl, {
    props: { models, tasks: tasksResp.tasks, active: idleActive },
  });
  await user.click(screen.getByRole("button", { name: "Run" }));
  const call = fetchMock.mock.calls.find((c) => String(c[0]) === "/api/runs");
  expect(call).toBeTruthy();
  expect(call![1].method).toBe("POST");
  const body = JSON.parse(call![1].body as string);
  expect(body.models).toEqual(["m1", "m2"]);
  expect(body.tasks).toEqual(["tetris", "race-fix"]);
});

it("omits unchecked ids", async () => {
  const user = userEvent.setup();
  render(RunControl, {
    props: { models, tasks: tasksResp.tasks, active: idleActive },
  });
  await user.click(screen.getByLabelText("Model Two"));
  await user.click(screen.getByLabelText("Race fix"));
  await user.click(screen.getByRole("button", { name: "Run" }));
  const call = fetchMock.mock.calls.find((c) => String(c[0]) === "/api/runs");
  const body = JSON.parse(call![1].body as string);
  expect(body.models).toEqual(["m1"]);
  expect(body.tasks).toEqual(["tetris"]);
});

it("disables Run and shows progress while a batch is active", () => {
  render(RunControl, {
    props: {
      models,
      tasks: tasksResp.tasks,
      active: { running: true, task: "tetris", model: "m1", done: 1, total: 4 },
    },
  });
  expect(screen.getByRole("button", { name: "Run" })).toBeDisabled();
  expect(screen.getByText("1/4")).toBeInTheDocument();
});
