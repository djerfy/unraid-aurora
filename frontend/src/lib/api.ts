import type {
  ActionKind,
  Container,
  ContainerConnections,
  LogLine,
  SessionInfo,
  Snapshot,
  StorageLayout,
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

async function readNDJSON<T>(body: ReadableStream<Uint8Array>, onEvent: (event: T) => void) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let index = buffer.indexOf("\n");
    while (index >= 0) {
      const line = buffer.slice(0, index).trim();
      buffer = buffer.slice(index + 1);
      if (line) {
        try {
          onEvent(JSON.parse(line) as T);
        } catch {
          // Ignore a truncated line; the next chunk completes it.
        }
      }
      index = buffer.indexOf("\n");
    }
  }
  if (buffer.trim()) {
    onEvent(JSON.parse(buffer) as T);
  }
}

export const api = {
  bootstrap: () => request<Snapshot>("/api/v1/bootstrap"),
  serverHealth: () => request<import("./types").ServerHealth>("/api/v1/health"),
  auditLog: (limit = 200) => request<import("./types").AuditEntry[]>(`/api/v1/audit?limit=${limit}`),
  apiTokens: () => request<import("./types").APIToken[]>("/api/v1/tokens"),
  createAPIToken: (name: string, role: "viewer" | "operator" | "admin", expiresInDays: number) =>
    request<{ token: import("./types").APIToken; secret: string }>("/api/v1/tokens", {
      method: "POST",
      body: JSON.stringify({ name, role, expiresInDays }),
    }),
  revokeAPIToken: (id: string) =>
    request<{ ok: boolean }>(`/api/v1/tokens/${encodeURIComponent(id)}`, { method: "DELETE" }),
  metricsHistory: (range: string) =>
    request<import("./types").MetricPoint[]>(`/api/v1/metrics/history?range=${encodeURIComponent(range)}`),
  flashBackup: () => fetch("/api/v1/tools/flash-backup", { credentials: "same-origin" }),
  configExport: (redacted = false) => fetch(`/api/v1/config/export${redacted ? "?redact=1" : ""}`, { credentials: "same-origin" }),
  configImport: async (file: File) => {
    const body = new FormData();
    body.append("file", file);
    const res = await fetch("/api/v1/config/import", {
      method: "POST",
      headers: { "X-Aurora-Request": "aurora" },
      body,
      credentials: "same-origin",
    });
    if (!res.ok) {
      const payload = await res.json().catch(() => null);
      throw new ApiError(res.status, "import_failed", payload?.error?.message ?? res.statusText);
    }
    return (await res.json()) as { ok: boolean; files: string[]; restartRequired: boolean };
  },
  upsSettings: () => request<import("./types").UPSSettings>("/api/v1/ups/settings"),
  saveUPSSettings: (settings: import("./types").UPSSettings) =>
    request<{ ok: boolean }>("/api/v1/ups/settings", { method: "PUT", body: JSON.stringify(settings) }),
  zfsPools: () => request<import("./types").ZFSPool[]>("/api/v1/zfs/pools"),
  scrubZFSPool: (name: string) =>
    request<{ ok: boolean }>(`/api/v1/zfs/pools/${encodeURIComponent(name)}/scrub`, { method: "POST", body: "{}" }),
  zfsSnapshots: (dataset: string) =>
    request<import("./types").ZFSSnapshot[]>(`/api/v1/zfs/snapshots?dataset=${encodeURIComponent(dataset)}`),
  createZFSSnapshot: (dataset: string, name: string) =>
    request<{ ok: boolean }>("/api/v1/zfs/snapshots", { method: "POST", body: JSON.stringify({ dataset, name }) }),
  deleteZFSSnapshot: (dataset: string, name: string) =>
    request<{ ok: boolean }>(`/api/v1/zfs/snapshots?dataset=${encodeURIComponent(dataset)}&name=${encodeURIComponent(name)}`, { method: "DELETE" }),
  userScripts: () => request<import("./types").UserScript[]>("/api/v1/user-scripts"),
  userScript: (id: string) =>
    request<{ content: string }>(`/api/v1/user-scripts/${encodeURIComponent(id)}`),
  runUserScript: (id: string) =>
    request<{ output: string; error?: string }>(`/api/v1/user-scripts/${encodeURIComponent(id)}/run`, {
      method: "POST",
      body: "{}",
    }),
  updateUserScript: (id: string, content: string, frequency: string, cron: string) =>
    request<{ ok: boolean }>(`/api/v1/user-scripts/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify({ content, frequency, cron }),
    }),
  testNotification: (settings: import("./types").Settings) =>
    request<{ ok: boolean }>("/api/v1/notifications/test", {
      method: "POST",
      body: JSON.stringify(settings),
    }),
  session: () => request<SessionInfo>("/api/v1/auth/session"),
  login: (username: string, password: string) =>
    request<{ authenticated: boolean; user: string }>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  logout: () => request<{ ok: boolean }>("/api/v1/auth/logout", { method: "POST", body: "{}" }),
  revokeAllSessions: () =>
    request<{ ok: boolean }>("/api/v1/auth/revoke-all", { method: "POST", body: "{}" }),
  action: (kind: ActionKind, target = "", value = "") =>
    request<{ ok: boolean }>("/api/v1/actions", {
      method: "POST",
      body: JSON.stringify({ kind, target, value }),
    }),
  containerLogs: (id: string, lines = 160) =>
    request<LogLine[]>(`/api/v1/docker/containers/${encodeURIComponent(id)}/logs?lines=${lines}`),
  containerConnections: (id: string) =>
    request<ContainerConnections>(`/api/v1/docker/containers/${encodeURIComponent(id)}/connections`),
  createUser: (options: import("./types").UserCreateOptions) =>
    request<{ ok: boolean }>("/api/v1/users", { method: "POST", body: JSON.stringify(options) }),
  updateUser: (name: string, description: string) =>
    request<{ ok: boolean }>(`/api/v1/users/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ description }),
    }),
  changeUserPassword: (name: string, password: string) =>
    request<{ ok: boolean }>(`/api/v1/users/${encodeURIComponent(name)}/password`, {
      method: "PUT",
      body: JSON.stringify({ password }),
    }),
  deleteUser: (name: string) =>
    request<{ ok: boolean }>(`/api/v1/users/${encodeURIComponent(name)}`, { method: "DELETE" }),
  installPlugin: async (
    url: string,
    onEvent: (event: import("./types").PluginEvent) => void,
  ) => {
    const res = await fetch("/api/v1/plugins/install", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Aurora-Request": "aurora" },
      body: JSON.stringify({ url }),
      credentials: "same-origin",
    });
    if (!res.ok || !res.body) {
      const payload = await res.json().catch(() => null);
      const body = payload as { error?: { code?: string; message?: string } } | null;
      throw new ApiError(res.status, body?.error?.code ?? "http_error", body?.error?.message ?? res.statusText);
    }
    await readNDJSON(res.body, onEvent);
  },
  removePlugin: async (
    name: string,
    onEvent: (event: import("./types").PluginEvent) => void,
  ) => {
    const res = await fetch(`/api/v1/plugins/${encodeURIComponent(name)}`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json", "X-Aurora-Request": "aurora" },
      credentials: "same-origin",
    });
    if (!res.ok || !res.body) {
      const payload = await res.json().catch(() => null);
      const body = payload as { error?: { code?: string; message?: string } } | null;
      throw new ApiError(res.status, body?.error?.code ?? "http_error", body?.error?.message ?? res.statusText);
    }
    await readNDJSON(res.body, onEvent);
  },
  osStatus: () => request<import("./types").OSStatus>("/api/v1/system/os"),
  checkOSUpdate: () =>
    request<import("./types").OSStatus>("/api/v1/system/os/check", { method: "POST", body: "{}" }),
  installOSUpdate: async (onEvent: (event: import("./types").PluginEvent) => void) => {
    const res = await fetch("/api/v1/system/os/update", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Aurora-Request": "aurora" },
      body: "{}",
      credentials: "same-origin",
    });
    if (!res.ok || !res.body) {
      const payload = await res.json().catch(() => null);
      const body = payload as { error?: { code?: string; message?: string } } | null;
      throw new ApiError(res.status, body?.error?.code ?? "http_error", body?.error?.message ?? res.statusText);
    }
    await readNDJSON(res.body, onEvent);
  },
  restorePreviousOS: () =>
    request<{ ok: boolean; version: string; rebootRequired: boolean }>("/api/v1/system/os/restore", {
      method: "POST",
      body: "{}",
    }),
  unraidAPIStatus: () => request<import("./types").UnraidAPIStatus>("/api/v1/unraid-api"),
  unraidAPIAction: (action: "start" | "stop" | "restart") =>
    request<{ ok: boolean; output: string }>(`/api/v1/unraid-api/${action}`, { method: "POST", body: "{}" }),
  unraidAPILogs: (lines = 200) =>
    request<import("./types").LogLine[]>(`/api/v1/unraid-api/logs?lines=${lines}`),
  networkConfig: () => request<import("./types").NetworkConfiguration>("/api/v1/network/config"),
  saveNetworkConfig: (config: import("./types").NetworkConfiguration, acknowledge: boolean) =>
    request<import("./types").NetworkConfiguration>("/api/v1/network/config", {
      method: "PUT",
      body: JSON.stringify({ ...config, acknowledge }),
    }),
  applyNetworkConfig: () =>
    request<{ ok: boolean; applying: boolean }>("/api/v1/network/apply", {
      method: "POST",
      body: JSON.stringify({ acknowledge: true }),
    }),
  revertNetworkConfig: () =>
    request<{ ok: boolean; backup: string }>("/api/v1/network/revert", { method: "POST", body: "{}" }),
  setNetworkInterfaceState: (name: string, action: "up" | "down") =>
    request<{ ok: boolean; name: string; state: string }>(
      `/api/v1/network/interfaces/${encodeURIComponent(name)}/${action}`,
      { method: "POST", body: "{}" },
    ),
  createContainer: (options: import("./types").ContainerCreateOptions) =>
    request<{ id: string }>("/api/v1/docker/containers", {
      method: "POST",
      body: JSON.stringify(options),
    }),
  bulkContainerAction: (action: "start" | "stop" | "restart", targets: string[]) =>
    request<{ ok: boolean; succeeded: string[]; failed: { target: string; error: string }[] }>(
      "/api/v1/docker/containers/bulk",
      { method: "POST", body: JSON.stringify({ action, targets }) },
    ),
  removeContainer: (id: string) =>
    request<{ ok: boolean }>(`/api/v1/docker/containers/${encodeURIComponent(id)}`, { method: "DELETE" }),
  containerNetworks: () => request<string[]>("/api/v1/docker/networks"),
  containerSecurity: (id: string) =>
    request<import("./types").ContainerSecurityDetail>(`/api/v1/docker/containers/${encodeURIComponent(id)}/security`),
  securityOverview: () => request<import("./types").SecurityOverview>("/api/v1/docker/security"),
  scanContainer: (id: string) =>
    request<{ ok: boolean; started: boolean }>(`/api/v1/docker/containers/${encodeURIComponent(id)}/scan`, {
      method: "POST",
      body: "{}",
    }),
  scanAllContainers: () =>
    request<{ ok: boolean; started: number }>("/api/v1/docker/scan-all", {
      method: "POST",
      body: "{}",
    }),
  trivyStatus: () => request<import("./types").TrivyStatus>("/api/v1/trivy"),
  trivyVersions: () => request<import("./types").TrivyVersion[]>("/api/v1/trivy/versions"),
  installTrivy: (version: string) =>
    request<{ ok: boolean; started: boolean }>("/api/v1/trivy/install", {
      method: "POST",
      body: JSON.stringify({ version }),
    }),
  updateTrivyDB: () =>
    request<{ ok: boolean; started: boolean }>("/api/v1/trivy/db/update", {
      method: "POST",
      body: "{}",
    }),
  containerTemplates: () => request<import("./types").ContainerTemplate[]>("/api/v1/docker/templates"),
  appsCatalog: (params: {
    query?: string;
    kind?: string;
    category?: string;
    order?: string;
    limit?: number;
    offset?: number;
  }) => {
    const search = new URLSearchParams();
    if (params.query) search.set("query", params.query);
    if (params.kind) search.set("kind", params.kind);
    if (params.category) search.set("category", params.category);
    if (params.order) search.set("order", params.order);
    if (params.limit) search.set("limit", String(params.limit));
    if (params.offset) search.set("offset", String(params.offset));
    return request<import("./types").AppCatalogPage>(`/api/v1/apps?${search.toString()}`);
  },
  appTemplate: (url: string) =>
    request<import("./types").ContainerTemplate>(`/api/v1/apps/template?url=${encodeURIComponent(url)}`),
  schedulerState: () => request<import("./types").SchedulerState>("/api/v1/system/scheduler"),
  previewScheduler: (settings: import("./types").SchedulerSettings) =>
    request<import("./types").SchedulerPreview>("/api/v1/system/scheduler/preview", {
      method: "POST",
      body: JSON.stringify(settings),
    }),
  applyScheduler: (settings: import("./types").SchedulerSettings, acknowledge: boolean) =>
    request<{ ok: boolean; state?: import("./types").SchedulerState }>("/api/v1/system/scheduler", {
      method: "PUT",
      body: JSON.stringify({ ...settings, acknowledge }),
    }),
  updateDisplaySettings: (display: import("./types").SystemDisplaySettings, acknowledge: boolean) =>
    request<{ ok: boolean }>("/api/v1/system/display", {
      method: "PUT",
      body: JSON.stringify({ ...display, acknowledge }),
    }),
  diskSmart: (device: string) =>
    request<import("./types").SmartReport>(`/api/v1/disks/${encodeURIComponent(device)}/smart`),
  runSmartTest: (device: string) =>
    request<{ ok: boolean }>(`/api/v1/disks/${encodeURIComponent(device)}/smart-test`, {
      method: "POST",
      body: "{}",
    }),
  setContainerAutostart: (id: string, enabled: boolean) =>
    request<{ ok: boolean }>(`/api/v1/docker/containers/${encodeURIComponent(id)}/autostart`, {
      method: "PUT",
      body: JSON.stringify({ enabled }),
    }),
  setVMAutostart: (id: string, enabled: boolean) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/autostart`, {
      method: "PUT",
      body: JSON.stringify({ enabled }),
    }),
  vmXML: (id: string) =>
    request<{ xml: string }>(`/api/v1/vms/${encodeURIComponent(id)}/xml`),
  createVM: (options: import("./types").VMCreateOptions) =>
    request<import("./types").VM>("/api/v1/vms", { method: "POST", body: JSON.stringify(options) }),
  vmHostDevices: () => request<import("./types").VMHostDevice[]>("/api/v1/vms/host-devices"),
  vmAdvanced: (id: string) =>
    request<import("./types").VMAdvancedConfig>(`/api/v1/vms/${encodeURIComponent(id)}/advanced`),
  updateVMAdvanced: (id: string, config: import("./types").VMAdvancedConfig) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/advanced`, {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  cloneVM: (id: string, name: string, start: boolean) =>
    request<import("./types").VM>(`/api/v1/vms/${encodeURIComponent(id)}/clone`, {
      method: "POST",
      body: JSON.stringify({ name, start }),
    }),
  resizeVMDisk: (id: string, sizeGB: number) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/resize`, {
      method: "POST",
      body: JSON.stringify({ sizeGB }),
    }),
  updateVMXML: (id: string, xml: string) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/xml`, {
      method: "PUT",
      body: JSON.stringify({ xml }),
    }),
  vmSnapshots: (id: string) =>
    request<import("./types").VMSnapshot[]>(`/api/v1/vms/${encodeURIComponent(id)}/snapshots`),
  createVMSnapshot: (id: string, name: string, description: string) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/snapshots`, {
      method: "POST",
      body: JSON.stringify({ name, description }),
    }),
  restoreVMSnapshot: (id: string, name: string) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/snapshots/${encodeURIComponent(name)}/revert`, {
      method: "POST",
      body: "{}",
    }),
  deleteVMSnapshot: (id: string, name: string) =>
    request<{ ok: boolean }>(`/api/v1/vms/${encodeURIComponent(id)}/snapshots/${encodeURIComponent(name)}`, {
      method: "DELETE",
    }),
  createShare: (config: import("./types").ShareConfig) =>
    request<{ ok: boolean }>("/api/v1/shares", { method: "POST", body: JSON.stringify(config) }),
  shareConfig: (name: string) =>
    request<import("./types").ShareConfig>(`/api/v1/shares/${encodeURIComponent(name)}/config`),
  updateShare: (name: string, config: import("./types").ShareConfig) =>
    request<{ ok: boolean }>(`/api/v1/shares/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  updateShareSMB: (name: string, config: import("./types").ShareConfig) =>
    request<{ ok: boolean }>(`/api/v1/shares/${encodeURIComponent(name)}/smb`, {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  updateShareNFS: (name: string, config: import("./types").ShareConfig) =>
    request<{ ok: boolean }>(`/api/v1/shares/${encodeURIComponent(name)}/nfs`, {
      method: "PUT",
      body: JSON.stringify(config),
    }),
  balanceDisks: () => request<import("./types").BalanceDisk[]>("/api/v1/balance/disks"),
  balanceBrowse: (disk: string, path: string) =>
    request<import("./types").BalanceEntry[]>(
      `/api/v1/balance/browse?disk=${encodeURIComponent(disk)}&path=${encodeURIComponent(path)}`,
    ),
  balanceStatus: () => request<import("./types").BalanceJob>("/api/v1/balance/status"),
  balanceHistory: () => request<import("./types").BalanceJob[]>("/api/v1/balance/history"),
  startBalance: (options: import("./types").BalanceOptions) =>
    request<import("./types").BalanceJob>("/api/v1/balance/start", {
      method: "POST",
      body: JSON.stringify(options),
    }),
  cancelBalance: () =>
    request<{ ok: boolean }>("/api/v1/balance/cancel", { method: "POST", body: "{}" }),
  configHistory: () => request<import("./types").ConfigHistory[]>("/api/v1/config-history"),
  configHistoryDetail: (id: string) =>
    request<import("./types").ConfigHistory>(`/api/v1/config-history/${encodeURIComponent(id)}`),
  rollbackConfig: (id: string) =>
    request<{ ok: boolean }>(`/api/v1/config-history/${encodeURIComponent(id)}/rollback`, {
      method: "POST",
      body: "{}",
    }),
  checkPluginUpdates: () =>
    request<import("./types").Plugin[]>("/api/v1/plugins/check-updates", { method: "POST", body: "{}" }),
  updatePlugin: async (
    name: string,
    onEvent: (event: import("./types").PluginEvent) => void,
  ) => {
    const res = await fetch(`/api/v1/plugins/${encodeURIComponent(name)}/update`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Aurora-Request": "aurora" },
      body: "{}",
      credentials: "same-origin",
    });
    if (!res.ok || !res.body) {
      const payload = await res.json().catch(() => null);
      const body = payload as { error?: { code?: string; message?: string } } | null;
      throw new ApiError(
        res.status,
        body?.error?.code ?? "http_error",
        body?.error?.message ?? res.statusText,
      );
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let index = buffer.indexOf("\n");
      while (index >= 0) {
        const line = buffer.slice(0, index).trim();
        buffer = buffer.slice(index + 1);
        if (line) {
          try {
            onEvent(JSON.parse(line) as import("./types").PluginEvent);
          } catch {
            // Ignore a truncated line; the next chunk completes it.
          }
        }
        index = buffer.indexOf("\n");
      }
    }
    if (buffer.trim()) {
      onEvent(JSON.parse(buffer) as import("./types").PluginEvent);
    }
  },
  diagnostics: async () => {
    const response = await fetch("/api/v1/tools/diagnostics", { credentials: "same-origin" });
    if (!response.ok) {
      throw new ApiError(response.status, "diagnostics_failed", response.statusText || "diagnostics failed");
    }
    return response.blob();
  },
  checkContainerUpdates: () => request<import("./types").ImageCheck[]>("/api/v1/docker/check-updates", {
    method: "POST",
    body: "{}",
  }),
  updateContainer: async (
    id: string,
    onEvent: (event: import("./types").ContainerUpdateEvent) => void,
  ) => {
    const res = await fetch(`/api/v1/docker/containers/${encodeURIComponent(id)}/update`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Aurora-Request": "aurora" },
      body: "{}",
      credentials: "same-origin",
    });
    if (!res.ok || !res.body) {
      const payload = await res.json().catch(() => null);
      const body = payload as { error?: { code?: string; message?: string } } | null;
      throw new ApiError(
        res.status,
        body?.error?.code ?? "http_error",
        body?.error?.message ?? res.statusText,
      );
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let index = buffer.indexOf("\n");
      while (index >= 0) {
        const line = buffer.slice(0, index).trim();
        buffer = buffer.slice(index + 1);
        if (line) {
          try {
            onEvent(JSON.parse(line) as import("./types").ContainerUpdateEvent);
          } catch {
            // Ignore a truncated line; the next chunk completes it.
          }
        }
        index = buffer.indexOf("\n");
      }
    }
    if (buffer.trim()) {
      onEvent(JSON.parse(buffer) as import("./types").ContainerUpdateEvent);
    }
  },
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
  storageLayout: () => request<StorageLayout>("/api/v1/storage-layout"),
  saveStorageLayout: (layout: StorageLayout) => {
    const { updated: _updated, ...body } = layout;
    return request<StorageLayout>("/api/v1/storage-layout", {
      method: "PUT",
      body: JSON.stringify(body),
    });
  },
  saveSettings: (settings: Snapshot["settings"]) =>
    request<Snapshot["settings"]>("/api/v1/settings", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),
};

export function isContainerRunning(container: Container) {
  return container.state === "running";
}
