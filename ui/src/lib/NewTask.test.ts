import { render, screen } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import NewTask from "./NewTask.svelte";
import { jsonRes } from "./fixtures";

beforeEach(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => jsonRes({ error: "id must match ^[a-z0-9-]+$" }, 400)),
  );
});

it("shows the server error on 400", async () => {
  const user = userEvent.setup();
  render(NewTask);
  await user.type(screen.getByLabelText("ID"), "Bad ID");
  await user.type(screen.getByLabelText("Title"), "Bad");
  await user.type(screen.getByLabelText("Prompt"), "do it");
  await user.click(screen.getByRole("button", { name: "Create task" }));
  expect(await screen.findByText(/id must match/)).toBeInTheDocument();
});

it("shows the check field only for check tasks", async () => {
  const user = userEvent.setup();
  render(NewTask);
  expect(screen.queryByLabelText("Check script")).not.toBeInTheDocument();
  await user.selectOptions(screen.getByLabelText("Type"), "check");
  expect(screen.getByLabelText("Check script")).toBeInTheDocument();
});
