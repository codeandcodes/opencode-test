import type {
  FileEntry,
  MatrixResponse,
  Model,
  Result,
  RunDetailResponse,
  RunRef,
  Task,
  TasksResponse,
  Verdict,
} from "./types";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body && typeof body.error === "string") msg = body.error;
    } catch {
      // non-JSON error body; keep the status text
    }
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

const enc = encodeURIComponent;

function runBase(ref: RunRef): string {
  return `/api/runs/${enc(ref.task)}/${enc(ref.model)}/${enc(ref.timestamp)}`;
}

export function getModels(): Promise<Model[]> {
  return request("/api/models");
}

export function getTasks(): Promise<TasksResponse> {
  return request("/api/tasks");
}

export function getTask(id: string): Promise<Task> {
  return request(`/api/tasks/${enc(id)}`);
}

export function createTask(t: Task): Promise<Task> {
  return request("/api/tasks", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(t),
  });
}

export function getMatrix(): Promise<MatrixResponse> {
  return request("/api/runs");
}

export function startRuns(
  models: string[],
  tasks: string[],
  force = false,
  samples = 1,
): Promise<{ jobs: number; skipped: number }> {
  return request("/api/runs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ models, tasks, force, samples }),
  });
}

export function cancelRuns(): Promise<void> {
  return request("/api/runs/active", { method: "DELETE" });
}

export function getHistory(task: string, model: string): Promise<Result[]> {
  return request(`/api/runs/${enc(task)}/${enc(model)}`);
}

export function getRun(ref: RunRef): Promise<RunDetailResponse> {
  return request(runBase(ref));
}

export function getFiles(ref: RunRef): Promise<FileEntry[]> {
  return request(`${runBase(ref)}/files`);
}

export function fileUrl(ref: RunRef, path: string): string {
  return `${runBase(ref)}/files/${path.split("/").map(enc).join("/")}`;
}

export function previewUrl(ref: RunRef, path: string): string {
  return `${runBase(ref)}/preview/${path.split("/").map(enc).join("/")}`;
}

export function getReviewsPending(): Promise<RunRef[]> {
  return request("/api/reviews/pending");
}

export interface LeaderboardRow {
  model: string;
  check_cells: number;
  check_cells_passed: number;
  reviews_done: number;
  verdict_good: number;
  verdict_bad: number;
  errors: number;
  median_tps: number;
  samples: number;
}

export function getLeaderboard(): Promise<LeaderboardRow[]> {
  return request("/api/leaderboard");
}

export interface ActiveTail {
  task: string;
  model: string;
  timestamp: string;
  steps: number;
  tool_calls: number;
  tokens_out: number;
  last_event_age_sec: number;
  recent: unknown[];
}

/** Returns null when no job is executing. */
export async function getActiveTail(): Promise<ActiveTail | null> {
  try {
    const r = await request<ActiveTail>("/api/runs/active/tail");
    // Shape guard: a mismatched server route can answer with other JSON.
    if (!r || typeof r.task !== "string" || !Array.isArray(r.recent)) {
      return null;
    }
    return r;
  } catch {
    return null;
  }
}

export interface ResumableBatch {
  jobs: { model: string; task: string }[];
  count: number;
}

/** Returns null when there is no interrupted batch to resume. */
export async function getResumable(): Promise<ResumableBatch | null> {
  try {
    const r = await request<ResumableBatch>("/api/runs/resumable");
    if (!r || typeof r.count !== "number" || !Array.isArray(r.jobs)) {
      return null;
    }
    return r;
  } catch {
    return null;
  }
}

export function resumeBatch(): Promise<{
  jobs: number;
  skipped: number;
  dropped: number;
}> {
  return request("/api/runs/resume", { method: "POST" });
}

export function dismissResumable(): Promise<void> {
  return request("/api/runs/resumable", { method: "DELETE" });
}

export function setVerdict(
  ref: RunRef,
  verdict: "good" | "bad",
  note: string,
): Promise<Verdict> {
  return request(`${runBase(ref)}/verdict`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ verdict, note }),
  });
}

export function clearVerdict(ref: RunRef): Promise<void> {
  return request(`${runBase(ref)}/verdict`, { method: "DELETE" });
}

export async function getFileText(ref: RunRef, path: string): Promise<string> {
  const res = await fetch(fileUrl(ref, path));
  if (!res.ok) throw new Error(`failed to load ${path}: ${res.status}`);
  return res.text();
}
