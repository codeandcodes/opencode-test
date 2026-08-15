import { render, screen } from "@testing-library/svelte";
import { expect, it } from "vitest";
import FileTree from "./FileTree.svelte";
import type { FileEntry, RunRef } from "./types";

const ref: RunRef = { task: "t", model: "m", timestamp: "ts" };

it("nests files under their directories", () => {
  const files: FileEntry[] = [
    { path: "a", size: 0, dir: true },
    { path: "a/b.txt", size: 5, dir: false },
    { path: "index.html", size: 10, dir: false },
  ];
  render(FileTree, { props: { ref, files } });
  const file = screen.getByText("b.txt");
  const dir = screen.getByText("a");
  // b.txt must be rendered inside the li of directory a
  expect(dir.closest("li")).toContainElement(file);
  // index.html stays at the root
  const root = screen.getByText("index.html");
  expect(dir.closest("li")).not.toContainElement(root);
});
