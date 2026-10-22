import type { Task, TaskPage, Stats, WordCount } from "./types.js";

const TOKEN = "dev-token"; // must match BUGHUNT_TOKEN if you start the server with one

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${TOKEN}`,
      ...(init.headers || {}),
    },
  });
  return res.json() as Promise<T>;
}

export const api = {
  listTasks: (params: URLSearchParams) => request<TaskPage>(`/api/tasks?${params}`),
  createTask: (t: Partial<Task>) =>
    request<Task>("/api/tasks", { method: "POST", body: JSON.stringify(t) }),
  updateTask: (t: Task) =>
    request<Task>(`/api/tasks/${t.id}`, { method: "PUT", body: JSON.stringify(t) }),
  deleteTask: (id: number) => request<void>(`/api/tasks/${id}`, { method: "DELETE" }),
  stats: () => request<Stats>("/api/stats"),
  wordstats: () => request<WordCount[]>("/api/wordstats"),
  runtime: () => request<Record<string, number>>("/api/debug/runtime"),
};
