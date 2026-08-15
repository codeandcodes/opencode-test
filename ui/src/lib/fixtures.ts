// Shared test fixtures.
import type {
  Active,
  MatrixResponse,
  Model,
  Result,
  TasksResponse,
} from "./types";

export const models: Model[] = [
  { id: "m1", name: "Model One" },
  { id: "m2", name: "Model Two" },
];

export const tasksResp: TasksResponse = {
  tasks: [
    {
      id: "tetris",
      title: "Tetris",
      category: "games",
      type: "review",
      timeout_minutes: 30,
    },
    {
      id: "race-fix",
      title: "Race fix",
      category: "concurrency",
      type: "check",
      timeout_minutes: 45,
    },
  ],
  warnings: [],
};

export function result(
  task: string,
  model: string,
  status: Result["status"],
  dur: number,
): Result {
  return {
    task,
    model,
    status,
    started_at: "2026-08-15T10:00:00Z",
    finished_at: "2026-08-15T10:01:00Z",
    duration_sec: dur,
    messages: 3,
    tool_calls: 1,
    tokens_in: 10,
    tokens_out: 20,
    timestamp: "2026-08-15T10-00-00Z",
  };
}

export const idleActive: Active = {
  running: false,
  task: "",
  model: "",
  done: 0,
  total: 0,
};

export const matrixResp: MatrixResponse = {
  matrix: {
    tetris: {
      m1: result("tetris", "m1", "done", 62.4),
      m2: result("tetris", "m2", "fail", 10.2),
    },
    "race-fix": {
      m1: result("race-fix", "m1", "pass", 33.7),
    },
  },
  active: idleActive,
};

export function jsonRes(v: unknown, status = 200): Response {
  return new Response(JSON.stringify(v), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
