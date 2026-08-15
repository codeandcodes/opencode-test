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

it("renders the real opencode schema: text, reasoning, rich tools", () => {
  const real: unknown[] = [
    { type: "step_start", timestamp: 1, part: { type: "step-start" } },
    {
      type: "reasoning",
      part: { type: "reasoning", text: "let me think about tetris" },
    },
    { type: "text", part: { type: "text", text: "I will fix the rotation." } },
    {
      type: "tool",
      part: {
        type: "tool",
        tool: "edit",
        state: {
          input: {
            filePath: "game.js",
            oldString: "rotate(cw)",
            newString: "rotateSRS(cw)",
          },
        },
      },
    },
    {
      type: "tool",
      part: {
        type: "tool",
        tool: "bash",
        state: { input: { command: "node test.js" }, output: "all ok" },
      },
    },
    { type: "step_finish", part: { type: "step-finish", tokens: {} } },
  ];
  render(Transcript, { props: { events: real } });
  expect(screen.getByText("I will fix the rotation.")).toBeInTheDocument();
  expect(screen.getByText(/thinking \(25 chars\)/)).toBeInTheDocument();
  expect(screen.getByText("rotate(cw)")).toBeInTheDocument();
  expect(screen.getByText("rotateSRS(cw)")).toBeInTheDocument();
  expect(screen.getByText("all ok")).toBeInTheDocument();
  const summaries = [...document.querySelectorAll("details.tool summary")];
  expect(summaries.some((s) => s.textContent?.includes("game.js"))).toBe(true);
  expect(summaries.some((s) => s.textContent?.includes("node test.js"))).toBe(
    true,
  );
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
