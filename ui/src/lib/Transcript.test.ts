import { render, screen } from "@testing-library/svelte";
import { expect, it } from "vitest";
import Transcript from "./Transcript.svelte";

const events: unknown[] = [
  {
    type: "message",
    role: "assistant",
    content: [{ type: "text", text: "hello world" }],
  },
  { type: "tool_use", name: "bash", input: { cmd: "ls" } },
  { type: "message", text: "second message text" },
];

it("renders message events as text blocks and tool events as details", () => {
  render(Transcript, { props: { events } });
  expect(screen.getByText("hello world")).toBeInTheDocument();
  expect(screen.getByText("second message text")).toBeInTheDocument();
  const details = document.querySelectorAll("details");
  expect(details).toHaveLength(1);
  expect(details[0].querySelector("summary")!.textContent).toContain("bash");
});

it("skips events that are neither message nor tool", () => {
  render(Transcript, {
    props: {
      events: [{ type: "step_start" }, { type: "message", text: "hi" }],
    },
  });
  expect(screen.getByText("hi")).toBeInTheDocument();
  expect(document.querySelectorAll(".transcript > *")).toHaveLength(1);
});
