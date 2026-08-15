import type {
  FileEntry,
  MatrixResponse,
  Model,
  Result,
  RunDetailResponse,
  RunRef,
  Task,
  TasksResponse,
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
): Promise<{ jobs: number }> {
  return request("/api/runs", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ models, tasks }),
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

export async function getFileText(ref: RunRef, path: string): Promise<string> {
  const res = await fetch(fileUrl(ref, path));
  if (!res.ok) throw new Error(`failed to load ${path}: ${res.status}`);
  return res.text();
}
