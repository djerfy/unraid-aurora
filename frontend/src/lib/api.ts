import type {
  ActionKind,
  Container,
  LogLine,
  SessionInfo,
  Snapshot,
  LogLine as LogLineType,
} from "./types";

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = init.method ?? "GET";
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (method !== "GET" && method !== "HEAD") headers.set("X-Aurora-Request", "aurora");

  const res = await fetch(path, { ...init, method, headers, credentials: "same-origin" });
  const contentType = res.headers.get("content-type") ?? "";
  const isJSON = contentType.includes("application/json");
  const payload = isJSON ? await res.json().catch(() => null) : await res.text();

  if (!res.ok) {
    const body = payload as { error?: { code?: string; message?: string } } | null;
    throw new ApiError(
      res.status,
      body?.error?.code ?? "http_error",
      body?.error?.message ?? (typeof payload === "string" ? payload : res.statusText),
    );
  }
  return payload as T;
}

export const api = {
  bootstrap: () => request<Snapshot>("/api/v1/bootstrap"),
  session: () => request<SessionInfo>("/api/v1/auth/session"),
  login: (username: string, password: string) =>
    request<{ authenticated: boolean; user: string }>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST", body: "{}" }),
  action: (kind: ActionKind, target = "", value = "") =>
    request<{ ok: boolean }>("/api/v1/actions", {
      method: "POST",
      body: JSON.stringify({ kind, target, value }),
    }),
  containerLogs: (id: string, lines = 160) =>
    request<LogLine[]>(`/api/v1/docker/containers/${encodeURIComponent(id)}/logs?lines=${lines}`),
  createContainer: (options: import("./types").ContainerCreateOptions) =>
    request<{ id: string }>("/api/v1/docker/containers", {
      method: "POST",
      body: JSON.stringify(options),
    }),
  removeContainer: (id: string) =>
    request<{ ok: boolean }>(`/api/v1/docker/containers/${encodeURIComponent(id)}`, { method: "DELETE" }),
  containerNetworks: () => request<string[]>("/api/v1/docker/networks"),
  checkContainerUpdates: () => request<import("./types").ImageCheck[]>("/api/v1/docker/check-updates", {
    method: "POST",
    body: "{}",
  }),
  containerConfig: (id: string) =>
    request<import("./types").ContainerConfig>(`/api/v1/docker/containers/${encodeURIComponent(id)}/config`),
  updateContainerConfig: (id: string, config: import("./types").ContainerConfig) =>
    request<{ ok: boolean }>(`/api/v1/docker/containers/${encodeURIComponent(id)}/config`, {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  startArray: (options: import("./types").ArrayStartOptions) =>
    request<{ ok: boolean }>("/api/v1/array/start", {
      method: "POST",
      body: JSON.stringify(options),
    }),
  stopArray: () =>
    request<{ ok: boolean }>("/api/v1/array/stop", { method: "POST", body: "{}" }),
  mover: () => request<import("./types").MoverStatus>("/api/v1/mover"),
  startMover: () => request<{ ok: boolean }>("/api/v1/mover/start", { method: "POST", body: "{}" }),
  stopMover: () => request<{ ok: boolean }>("/api/v1/mover/stop", { method: "POST", body: "{}" }),
  logs: (source: string, lines = 240) =>
    request<LogLineType[]>(`/api/v1/logs?source=${encodeURIComponent(source)}&lines=${lines}`),
  saveSettings: (settings: Snapshot["settings"]) =>
    request<Snapshot["settings"]>("/api/v1/settings", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),
};

export function isContainerRunning(container: Container) {
  return container.state === "running";
}
