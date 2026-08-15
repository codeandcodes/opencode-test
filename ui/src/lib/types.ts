// Shared API types mirroring the Go backend structs.

export type Status =
  "done" | "pass" | "fail" | "timeout" | "error" | "interrupted" | "running";

export interface Model {
  id: string;
  name: string;
}

export interface TaskSummary {
  id: string;
  title: string;
  category: string;
  type: "review" | "check";
  timeout_minutes: number;
}

export interface Task extends TaskSummary {
  prompt: string;
  check?: string;
}

export interface Result {
  task: string;
  model: string;
  status: Status;
  started_at: string;
  finished_at: string;
  duration_sec: number;
  messages: number;
  tool_calls: number;
  tokens_in: number;
  tokens_out: number;
  error?: string;
  timestamp: string;
}

/** matrix[taskID][modelID] = latest Result */
export type Matrix = Record<string, Record<string, Result>>;

export interface Active {
  running: boolean;
  task: string;
  model: string;
  done: number;
  total: number;
}

export interface MatrixResponse {
  matrix: Matrix;
  active: Active;
}

export interface TasksResponse {
  tasks: TaskSummary[];
  warnings: string[];
}

export interface RunDetailResponse {
  result: Result;
  events: unknown[];
  check_log: string;
}

export interface FileEntry {
  path: string;
  size: number;
  dir: boolean;
}

export interface RunnerEvent {
  type: "job_start" | "job_end" | "batch_end";
  task?: string;
  model?: string;
  status?: Status;
  done: number;
  total: number;
}

export interface RunRef {
  task: string;
  model: string;
  timestamp: string;
}
