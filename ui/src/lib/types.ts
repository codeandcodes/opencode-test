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
  tokens_reasoning: number;
  cache_read: number;
  /** model-active generation seconds (excludes load and tool time) */
  gen_seconds: number;
  /** first-token wait: model swap/load + initial prefill */
  load_seconds?: number;
  error?: string;
  timestamp: string;
  verdict?: Verdict;
  prompt_sha?: string;
  /** true when the task prompt changed after this run (matrix responses only) */
  stale?: boolean;
  /** assertion counts parsed from check.log; valid when check_parsed */
  check_passed?: number;
  check_failed?: number;
  check_parsed?: boolean;
}

export interface Provenance {
  model: string;
  prompt_sha: string;
  task: Task;
  llama_swap_entry?: unknown;
  captured_at: string;
}

export interface Verdict {
  /** 1-10 (10 = perfect, 1 = completely non-functional); absent on legacy verdicts */
  rating?: number;
  /** legacy binary judgment */
  verdict?: "good" | "bad";
  note?: string;
  at: string;
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

export interface CellAgg {
  samples: number;
  passes: number;
  fails: number;
  dones: number;
  verdict_good: number;
  verdict_bad: number;
  rating_count: number;
  rating_avg: number;
  median_tps: number;
}

export interface MatrixResponse {
  matrix: Matrix;
  agg?: Record<string, Record<string, CellAgg>>;
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
  provenance?: Provenance | null;
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
