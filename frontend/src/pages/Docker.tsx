import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowDownToLine,
  Boxes,
  Check,
  CloudDownload,
  Copy,
  Cpu,
  ExternalLink,
  Info as InfoIcon,
  List,
  MemoryStick,
  Network,
  Play,
  Plus,
  RefreshCw,
  RotateCw,
  Search,
  Settings2,
  Square,
  Terminal,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { ContainerCreateModal } from "../components/containers/ContainerCreate";
import { ContainerUpdateModal, type ContainerUpdateRun } from "../components/containers/ContainerUpdateModal";
import { ContainerSettingsModal } from "../components/containers/ContainerSettings";
import { EntityIcon, containerIconURL } from "../components/ui/entity-icon";
import { Drawer, Menu, Modal } from "../components/ui/overlays";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  InputBase,
  Progress,
  Switch,
  Tabs,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { useAction } from "../lib/useAction";
import { api } from "../lib/api";
import {
  formatMemory,
  formatPercent,
  formatUptime,
  hexToRgba,
  relativeTime,
} from "../lib/format";
import type { Container } from "../lib/types";

type Filter = "all" | "running" | "stopped" | "updates";

export function DockerPage() {
  const { snapshot, refresh } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const action = useAction();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [filter, setFilter] = useState<Filter>("all");
  const [view, setView] = useState<"grid" | "list">(() => {
    try {
      return localStorage.getItem("aurora.docker.view") === "list" ? "list" : "grid";
    } catch {
      return "grid";
    }
  });
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<Container | null>(null);
  const [updateRun, setUpdateRun] = useState<ContainerUpdateRun | null>(null);
  const [creating, setCreating] = useState(false);
  const [removing, setRemoving] = useState<Container | null>(null);
  const [removeError, setRemoveError] = useState("");

  useEffect(() => {
    try {
      localStorage.setItem("aurora.docker.view", view);
    } catch {
      // Storage can be unavailable; the view still works for this session.
    }
  }, [view]);

  const selectedName = params.get("container");
  const selected = snapshot?.containers.find((container) => container.name === selectedName) ?? null;

  const containers = snapshot?.containers ?? [];
  const running = containers.filter((container) => container.state === "running");
  const stopped = containers.filter((container) => container.state !== "running");
  const updates = containers.filter((container) => container.updateAvailable);

  const filtered = useMemo(() => {
    return containers
      .filter((container) => {
        if (filter === "running") return container.state === "running";
        if (filter === "stopped") return container.state !== "running";
        if (filter === "updates") return container.updateAvailable;
        return true;
      })
      .filter((container) => {
        if (!query.trim()) return true;
        const haystack = `${container.name} ${container.image} ${container.category} ${container.network} ${container.ip}`.toLowerCase();
        return haystack.includes(query.toLowerCase());
      })
      .sort((a, b) => {
        if (a.state !== b.state) return a.state === "running" ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
  }, [containers, filter, query]);

  const openContainer = (container: Container) => {
    const next = new URLSearchParams(params);
    next.set("container", container.name);
    setParams(next, { replace: true });
  };

  const closeContainer = () => {
    const next = new URLSearchParams(params);
    next.delete("container");
    setParams(next, { replace: true });
  };

  const runUpdates = async (targets: Container[]) => {
    if (targets.length === 0 || updateRun?.running) return;
    setUpdateRun({
      names: targets.map((container) => container.name),
      index: 0,
      events: [],
      running: true,
      error: "",
      done: false,
    });
    for (let index = 0; index < targets.length; index += 1) {
      setUpdateRun((current) => (current ? { ...current, index, events: [] } : current));
      let streamError = "";
      try {
        await api.updateContainer(targets[index].id, (event) => {
          if (event.error) streamError = event.message;
          setUpdateRun((current) =>
            current ? { ...current, events: [...current.events.slice(-40), event] } : current,
          );
        });
        if (streamError) throw new Error(streamError);
      } catch (error) {
        setUpdateRun((current) =>
          current ? { ...current, running: false, error: (error as Error).message } : current,
        );
        void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
        return;
      }
    }
    setUpdateRun((current) => (current ? { ...current, running: false, done: true } : current));
    void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    toast({
      kind: "success",
      title: targets.length > 1 ? t("Containers updated") : t("{name} updated", { name: targets[0].name }),
      description:
        targets.length > 1
          ? t("{count} images pulled and recreated.", { count: targets.length })
          : t("1 image pulled and recreated."),
    });
  };

  const checkUpdates = useMutation({
    mutationFn: api.checkContainerUpdates,
    onSuccess: (checks) => {
      const available = checks.filter((check) => check.update).length;
      const failed = checks.filter((check) => check.error).length;
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({
        kind: available ? "warning" : "success",
        title: available
          ? available === 1
            ? t("1 update available")
            : t("{count} updates available", { count: available })
          : t("All images are up to date"),
        description:
          (checks.length === 1
            ? t("1 image checked")
            : t("{count} images checked", { count: checks.length })) +
          (failed
            ? failed === 1
              ? t(" · 1 could not be checked")
              : t(" · {count} could not be checked", { count: failed })
            : ""),
      });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Update check failed"), description: error.message });
    },
  });

  const remove = useMutation({
    mutationFn: (container: Container) => api.removeContainer(container.id),
    onSuccess: (_result, container) => {
      toast({ kind: "info", title: t("{name} removed", { name: container.name }) });
      if (selectedName === container.name) closeContainer();
      setRemoving(null);
      setRemoveError("");
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
    onError: (error: Error) => {
      setRemoveError(error.message);
      toast({ kind: "error", title: t("Removal failed"), description: error.message });
    },
  });

  const totalCpu = running.reduce((sum, container) => sum + container.cpu, 0);
  const totalMemory = running.reduce((sum, container) => sum + container.memory, 0);

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("Containers")}
        description={t("Every Docker workload on the server, with live resource usage and one-click lifecycle controls.")}
        meta={
          <>
            <Badge tone="success" dot pulse>
              {running.length === 1
                ? t("1 running")
                : t("{count} running", { count: running.length })}
            </Badge>
            {stopped.length > 0 ? (
              <Badge tone="neutral">
                {stopped.length === 1
                  ? t("1 stopped")
                  : t("{count} stopped", { count: stopped.length })}
              </Badge>
            ) : null}
            {updates.length > 0 ? (
              <Badge tone="warning">
                {updates.length === 1
                  ? t("1 update")
                  : t("{count} updates", { count: updates.length })}
              </Badge>
            ) : null}
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
              {t("Refresh")}
            </Button>
            <Button
              variant="secondary"
              icon={<CloudDownload className="size-3.5" />}
              loading={checkUpdates.isPending}
              onClick={() => checkUpdates.mutate()}
            >
              {t("Check updates")}
            </Button>
            <Button
              variant="secondary"
              icon={<ArrowDownToLine className="size-3.5" />}
              loading={Boolean(updateRun?.running)}
              disabled={updates.length === 0 || Boolean(updateRun?.running)}
              onClick={() => void runUpdates(updates)}
            >
              {updates.length > 0
                ? t("Update all ({count})", { count: updates.length })
                : t("Update all")}
            </Button>
            <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setCreating(true)}>
              {t("New container")}
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <MetricCard
          icon={<Boxes className="size-4" />}
          label={t("Running")}
          value={`${running.length}`}
          hint={containers.length === 1 ? t("1 total") : t("{count} total", { count: containers.length })}
        />
        <MetricCard icon={<Cpu className="size-4" />} label={t("CPU footprint")} value={formatPercent(totalCpu, 1)} hint={t("across all containers")} />
        <MetricCard icon={<MemoryStick className="size-4" />} label={t("Memory")} value={formatMemory(totalMemory)} hint={t("resident memory")} />
        <MetricCard
          icon={<TriangleAlert className="size-4" />}
          label={t("Updates")}
          value={`${updates.length}`}
          hint={updates.length ? t("images ready to pull") : t("everything is current")}
          tone={updates.length ? "warning" : "success"}
        />
      </div>

      <div className="mb-5 flex flex-wrap items-center gap-3">
        <Tabs
          size="md"
          items={[
            { value: "all", label: t("All"), count: containers.length },
            { value: "running", label: t("Running"), count: running.length },
            { value: "stopped", label: t("Stopped"), count: stopped.length },
            { value: "updates", label: t("Updates"), count: updates.length },
          ]}
          value={filter}
          onChange={(value) => setFilter(value as Filter)}
        />
        <div className="ml-auto flex items-center gap-2">
          <InputBase
            value={query}
            onChange={setQuery}
            placeholder={t("Search containers…")}
            icon={<Search className="size-3.5" />}
            className="w-52"
          />
          <div className="flex items-center rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
            <button
              type="button"
              onClick={() => setView("grid")}
              aria-label={t("Grid view")}
              className={cn("grid size-7 place-items-center rounded-md transition", view === "grid" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
            >
              <Boxes className="size-3.5" />
            </button>
            <button
              type="button"
              onClick={() => setView("list")}
              aria-label={t("List view")}
              className={cn("grid size-7 place-items-center rounded-md transition", view === "list" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
            >
              <List className="size-3.5" />
            </button>
          </div>
        </div>
      </div>

      {filtered.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Boxes className="size-5" />}
            title={t("No containers match this view")}
            description={t("Adjust the filter or search query to see your workloads.")}
            action={
              <Button variant="secondary" onClick={() => { setFilter("all"); setQuery(""); }}>
                {t("Clear filters")}
              </Button>
            }
          />
        </Card>
      ) : view === "grid" ? (
        <div className="stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map((container) => (
            <ContainerCard
              key={container.id}
              container={container}
              onOpen={() => openContainer(container)}
              onEdit={() => setEditing(container)}
              onUpdate={() => void runUpdates([container])}
              onRemove={() => {
                setRemoveError("");
                setRemoving(container);
              }}
              busy={action.isPending && action.variables?.target === container.name}
            />
          ))}
        </div>
      ) : (
        <Card>
          <div className="hidden items-center gap-3 border-b border-line bg-[var(--surface-2)]/35 px-4 py-2 md:flex">
            <span className="min-w-0 flex-1 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle lg:w-52 lg:flex-none">
              {t("Container")}
            </span>
            <span className="hidden w-44 shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle md:block">{t("Image")}</span>
            <span className="hidden w-24 shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle lg:block">{t("Network")}</span>
            <span className="hidden w-28 shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle lg:block">IP</span>
            <span className="hidden w-[124px] shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle min-[1600px]:block min-[1800px]:w-64">
              {t("Mounts")}
            </span>
            <span className="hidden flex-1 lg:block" aria-hidden="true" />
            <span className="w-20 shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("State")}</span>
            <span className="hidden w-16 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle sm:block">CPU</span>
            <span className="hidden w-20 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle sm:block">{t("Memory")}</span>
            <span className="hidden w-16 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle md:block">{t("Uptime")}</span>
            <span className="w-[176px] shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Actions")}</span>
          </div>
          <div className="divide-y divide-line/60">
            {filtered.map((container) => (
              <ContainerRow
                key={container.id}
                container={container}
                onOpen={() => openContainer(container)}
                onEdit={() => setEditing(container)}
                onUpdate={() => void runUpdates([container])}
                onRemove={() => {
                  setRemoveError("");
                  setRemoving(container);
                }}
                busy={action.isPending && action.variables?.target === container.name}
              />
            ))}
          </div>
        </Card>
      )}

      <ContainerDrawer
        container={selected}
        onClose={closeContainer}
        onEdit={(container) => {
          setEditing(container);
          closeContainer();
        }}
        onUpdate={(container) => void runUpdates([container])}
        busy={action.isPending && action.variables?.target === selected?.name}
      />

      <ContainerSettingsModal container={editing} onClose={() => setEditing(null)} />

      <ContainerCreateModal open={creating} onClose={() => setCreating(false)} />

      <ContainerUpdateModal
        run={updateRun}
        onClose={() => {
          if (!updateRun?.running) setUpdateRun(null);
        }}
      />

      <Modal
        open={Boolean(removing)}
        onClose={() => setRemoving(null)}
        title={t("Remove {name}?", { name: removing?.name ?? t("container") })}
        description={t("The container is stopped and deleted. Bind mounts and appdata stay on disk.")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setRemoving(null)} disabled={remove.isPending}>
              {t("Cancel")}
            </Button>
            <Button
              variant="danger"
              icon={<Trash2 className="size-3.5" />}
              loading={remove.isPending}
              onClick={() => {
                if (removing) remove.mutate(removing);
              }}
            >
              {t("Remove container")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <p className="text-[11px] leading-relaxed text-muted">
            {t("This action cannot be undone from Aurora. Recreate the container from its image if needed.")}
          </p>
          {removeError ? (
            <div className="rounded-lg border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] text-[var(--danger)]">
              {removeError}
            </div>
          ) : null}
        </div>
      </Modal>
    </>
  );
}

function ContainerCard({
  container,
  onOpen,
  onEdit,
  onUpdate,
  onRemove,
  busy,
}: {
  container: Container;
  onOpen: () => void;
  onEdit: () => void;
  onUpdate: () => void;
  onRemove: () => void;
  busy?: boolean;
}) {
  const action = useAction();
  const { t } = useI18n();
  const running = container.state === "running";
  const ports = (container.ports ?? []).filter((port) => port.host > 0);
  const mounts = container.mounts ?? [];
  const memoryUsage = container.memoryLimit ? (container.memory / container.memoryLimit) * 100 : 0;

  return (
    <Card hover className="card-pad group relative overflow-hidden">
      <div
        className="pointer-events-none absolute -right-16 -top-20 size-48 rounded-full blur-3xl"
        style={{ background: hexToRgba(container.color, 0.14) }}
      />
      <div className="relative flex items-start justify-between gap-3">
        <button type="button" onClick={onOpen} className="flex min-w-0 items-center gap-3 text-left">
          <EntityIcon
            src={containerIconURL(container.id)}
            name={container.name}
            size={40}
          />
          <span className="min-w-0">
            <span className="flex items-center gap-2">
              <span className="truncate text-[13px] font-semibold text-ink transition group-hover:text-[var(--accent)]">
                {container.name}
              </span>
              {container.updateAvailable ? <span className="size-1.5 shrink-0 rounded-full bg-[var(--warning)]" /> : null}
            </span>
            <span className="mt-0.5 block truncate text-[10px] text-subtle">{container.image}</span>
          </span>
        </button>
        <Menu
          trigger={
            <IconButton label={t("Actions for {name}", { name: container.name })} size="sm">
              <span className="text-[15px] leading-none">⋯</span>
            </IconButton>
          }
          items={[
            { label: t("Details & logs"), icon: <InfoIcon className="size-3.5" />, onSelect: onOpen },
            { label: t("Edit settings"), icon: <Settings2 className="size-3.5" />, onSelect: onEdit },
            ...(container.updateAvailable
              ? [{
                  label: t("Update image & restart"),
                  icon: <ArrowDownToLine className="size-3.5" />,
                  onSelect: onUpdate,
                }]
              : []),
            {
              label: running ? t("Restart") : t("Start"),
              icon: running ? <RotateCw className="size-3.5" /> : <Play className="size-3.5" />,
              onSelect: () =>
                action.mutate({
                  kind: running ? "container.restart" : "container.start",
                  target: container.name,
                  success: running
                    ? t("{name} restarted", { name: container.name })
                    : t("{name} started", { name: container.name }),
                }),
            },
            ...(container.webUi
              ? [{ label: t("Open web UI"), onSelect: () => window.open(container.webUi, "_blank", "noopener") }]
              : []),
            { label: "", separator: true },
            {
              label: running ? t("Stop") : t("Already stopped"),
              icon: <Square className="size-3.5" />,
              danger: running,
              disabled: !running,
              onSelect: () =>
                action.mutate({
                  kind: "container.stop",
                  target: container.name,
                  success: t("{name} stopped", { name: container.name }),
                }),
            },
            { label: t("Remove container"), icon: <Trash2 className="size-3.5" />, danger: true, onSelect: onRemove },
          ]}
        />
      </div>

      <div className="relative mt-4 flex items-center gap-2">
        <Badge tone={running ? "success" : container.state === "paused" ? "warning" : "neutral"} dot pulse={running}>
          {running ? t("Running") : container.state}
        </Badge>
        <span className="text-[10px] text-subtle">{running ? t("Up {time}", { time: formatUptime(container.uptime) }) : container.status}</span>
        {container.updateAvailable ? <Badge tone="warning" className="ml-auto">{t("Update")}</Badge> : null}
      </div>

      <div className="relative mt-4 grid grid-cols-2 gap-3">
        <div>
          <div className="flex items-center justify-between text-[9.5px]">
            <span className="text-subtle">CPU</span>
            <span className="num text-soft">{container.cpu.toFixed(1)}%</span>
          </div>
          <Progress value={Math.min(100, container.cpu * 2)} height={4} className="mt-1.5" />
        </div>
        <div>
          <div className="flex items-center justify-between text-[9.5px]">
            <span className="text-subtle">{t("Memory")}</span>
            <span className="num text-soft">{formatMemory(container.memory)}</span>
          </div>
          <Progress value={memoryUsage} height={4} className="mt-1.5" tone="var(--info)" />
        </div>
      </div>

      <div className="relative mt-3 flex flex-wrap items-center gap-1.5">
        {ports.slice(0, 3).map((port) => (
          <span key={`${port.host}-${port.protocol}`} className="chip font-mono text-[9.5px]">
            {port.host}:{port.container}/{port.protocol}
          </span>
        ))}
        {ports.length > 3 ? <span className="text-[9.5px] text-subtle">+{ports.length - 3}</span> : null}
        {ports.length === 0 ? <span className="text-[9.5px] text-subtle">{t("No published ports")}</span> : null}
      </div>

      <div className="relative mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[9.5px] text-subtle">
        <span className="flex items-center gap-1">
          <Network className="size-3" /> {container.network || "—"}
        </span>
        <span className="font-mono">{container.ip || t("No address")}</span>
      </div>

      {mounts.length > 0 ? (
        <div className="relative mt-3 space-y-1 border-t border-line pt-3">
          <p className="text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Mounts")}</p>
          {mounts.map((mount) => (
            <div
              key={`${mount.source}-${mount.destination}`}
              className="flex min-w-0 items-center gap-1.5"
              title={`${mount.source} → ${mount.destination} (${mount.readOnly ? "ro" : "rw"})`}
            >
              <span className="min-w-0 flex-1 truncate font-mono text-[9px] text-subtle">{mount.source}</span>
              <span className="shrink-0 text-[9px] text-subtle">→</span>
              <span className="shrink-0 font-mono text-[9px] text-muted">{mount.destination}</span>
              <span
                className={cn(
                  "shrink-0 rounded border px-1 py-px font-mono text-[7.5px] uppercase tracking-wide",
                  mount.readOnly
                    ? "border-[color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[var(--warning-soft)] text-[var(--warning)]"
                    : "border-[color-mix(in_srgb,var(--success)_28%,transparent)] bg-[var(--success-soft)] text-[var(--success)]",
                )}
              >
                {mount.readOnly ? "ro" : "rw"}
              </span>
            </div>
          ))}
        </div>
      ) : null}

      <div className="relative mt-4 flex items-center justify-end gap-2 border-t border-line pt-3">
        <div className="flex items-center gap-1">
          {container.updateAvailable ? (
            <IconButton
              label={t("Update image and restart")}
              size="sm"
              disabled={busy}
              onClick={onUpdate}
            >
              <ArrowDownToLine className="size-3.5 text-[var(--warning)]" />
            </IconButton>
          ) : (
            <span className="size-8" aria-hidden="true" />
          )}
          <IconButton
            label={running ? t("Restart container") : t("Start container")}
            size="sm"
            disabled={busy}
            onClick={() =>
              action.mutate({
                kind: running ? "container.restart" : "container.start",
                target: container.name,
                success: running
                  ? t("{name} restarted", { name: container.name })
                  : t("{name} started", { name: container.name }),
              })
            }
          >
            {running ? <RotateCw className={cn("size-3.5", busy && "animate-spin")} /> : <Play className="size-3.5" />}
          </IconButton>
          {running ? (
            <IconButton
              label={t("Stop container")}
              size="sm"
              onClick={() =>
                action.mutate({
                  kind: "container.stop",
                  target: container.name,
                  success: t("{name} stopped", { name: container.name }),
                })
              }
            >
              <Square className="size-3.5" />
            </IconButton>
          ) : null}
          {container.webUi ? (
            <IconButton label={t("Open web UI")} size="sm" onClick={() => window.open(container.webUi, "_blank", "noopener")}>
              <ExternalLink className="size-3.5" />
            </IconButton>
          ) : null}
        </div>
      </div>
    </Card>
  );
}

function ContainerRow({
  container,
  onOpen,
  onEdit,
  onUpdate,
  onRemove,
  busy,
}: {
  container: Container;
  onOpen: () => void;
  onEdit: () => void;
  onUpdate: () => void;
  onRemove: () => void;
  busy?: boolean;
}) {
  const action = useAction();
  const { t } = useI18n();
  const running = container.state === "running";
  const mounts = container.mounts ?? [];
  return (
    <div className="flex items-center gap-3 px-4 py-3 transition hover:bg-[var(--surface-2)]/50">
      <button type="button" onClick={onOpen} className="flex min-w-0 flex-1 items-center gap-3 text-left lg:w-52 lg:flex-none">
        <EntityIcon
          src={containerIconURL(container.id)}
          name={container.name}
          size={32}
          className="rounded-lg"
        />
        <span className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[12px] font-medium text-ink">{container.name}</span>
          {container.updateAvailable ? <Badge tone="warning">{t("update")}</Badge> : null}
        </span>
      </button>
      <span className="hidden w-44 shrink-0 truncate text-[9.5px] text-subtle md:block" title={container.image}>
        {container.image}
      </span>
      <span className="hidden w-24 shrink-0 truncate text-[10px] text-muted lg:block" title={container.network || "—"}>
        {container.network || "—"}
      </span>
      <span className="hidden w-28 shrink-0 truncate font-mono text-[9.5px] text-subtle lg:block" title={container.ip || "—"}>
        {container.ip || "—"}
      </span>
      <span className="hidden w-[124px] shrink-0 flex-col gap-0.5 min-[1600px]:flex min-[1800px]:w-64">
        {mounts.length === 0 ? (
          <span className="text-[9px] text-subtle">—</span>
        ) : (
          mounts.map((mount) => (
            <span
              key={`${mount.source}-${mount.destination}`}
              className="flex min-w-0 items-center gap-1"
              title={`${mount.source} → ${mount.destination} (${mount.readOnly ? "ro" : "rw"})`}
            >
              <span className="truncate font-mono text-[9px] text-subtle">
                <span className="hidden min-[1800px]:inline">{mount.source} → </span>
                {mount.destination}
              </span>
              <span
                className={cn(
                  "shrink-0 rounded border px-1 py-px font-mono text-[7.5px] uppercase tracking-wide",
                  mount.readOnly
                    ? "border-[color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[var(--warning-soft)] text-[var(--warning)]"
                    : "border-[color-mix(in_srgb,var(--success)_28%,transparent)] bg-[var(--success-soft)] text-[var(--success)]",
                )}
              >
                {mount.readOnly ? "ro" : "rw"}
              </span>
            </span>
          ))
        )}
      </span>
      <span className="hidden flex-1 lg:block" aria-hidden="true" />
      <span className="w-20 shrink-0">
        <Badge tone={running ? "success" : "neutral"} dot>{running ? t("Running") : container.state}</Badge>
      </span>
      <span className="hidden w-16 text-right num text-[10px] text-muted sm:block">{container.cpu.toFixed(1)}% CPU</span>
      <span className="hidden w-20 text-right num text-[10px] text-muted sm:block">{formatMemory(container.memory)}</span>
      <span className="hidden w-16 text-right font-mono text-[9px] text-subtle md:block">{running ? formatUptime(container.uptime) : "—"}</span>
      <div className="flex w-[176px] shrink-0 items-center justify-end gap-1">
        {container.updateAvailable ? (
          <IconButton
            label={t("Update image and restart")}
            size="sm"
            disabled={busy}
            onClick={onUpdate}
          >
            <ArrowDownToLine className="size-3.5 text-[var(--warning)]" />
          </IconButton>
        ) : (
          <span className="size-8" aria-hidden="true" />
        )}
        <IconButton
          label={running ? t("Restart") : t("Start")}
          size="sm"
          disabled={busy}
          onClick={() =>
            action.mutate({
              kind: running ? "container.restart" : "container.start",
              target: container.name,
              success: running
                ? t("{name} restarted", { name: container.name })
                : t("{name} started", { name: container.name }),
            })
          }
        >
          {running ? <RotateCw className={cn("size-3.5", busy && "animate-spin")} /> : <Play className="size-3.5" />}
        </IconButton>
        <IconButton label={t("Edit settings")} size="sm" onClick={onEdit}>
          <Settings2 className="size-3.5" />
        </IconButton>
        <IconButton label={t("Details and logs")} size="sm" onClick={onOpen}>
          <InfoIcon className="size-3.5" />
        </IconButton>
        <IconButton label={t("Remove container")} size="sm" onClick={onRemove}>
          <Trash2 className="size-3.5" />
        </IconButton>
      </div>
    </div>
  );
}

function ContainerDrawer({
  container,
  onClose,
  onEdit,
  onUpdate,
  busy,
}: {
  container: Container | null;
  onClose: () => void;
  onEdit: (container: Container) => void;
  onUpdate: (container: Container) => void;
  busy?: boolean;
}) {
  const action = useAction();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const [copied, setCopied] = useState(false);
  const logRef = useRef<HTMLDivElement>(null);
  const running = container?.state === "running";
  const publishedPorts = (container?.ports ?? []).filter((port) => port.host > 0);
  const autostart = useMutation({
    mutationFn: (enabled: boolean) => api.setContainerAutostart(container?.id ?? "", enabled),
    onSuccess: (_result, enabled) => {
      toast({
        kind: "success",
        title: enabled ? t("Autostart enabled") : t("Autostart disabled"),
        description: container?.name,
      });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Autostart update failed"), description: error.message });
    },
  });

  const logs = useQuery({
    queryKey: ["container-logs", container?.id],
    queryFn: () => api.containerLogs(container?.id ?? "", 160),
    enabled: Boolean(container),
    refetchInterval: 6000,
  });

  useEffect(() => {
    if (!logs.data || !logRef.current) return;
    logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [logs.data]);

  const copyLogs = async () => {
    if (!logs.data) return;
    const text = logs.data.map((line) => `${line.time} ${line.level.toUpperCase()} ${line.message}`).join("\n");
    await navigator.clipboard.writeText(text);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  };

  if (!container) return null;

  return (
    <Drawer
      open={Boolean(container)}
      onClose={onClose}
      title={container.name}
      width="max-w-[1040px]"
      icon={
        <EntityIcon
          src={containerIconURL(container.id)}
          name={container.name}
          size={56}
        />
      }
      subtitle={
        <span className="flex flex-wrap items-center gap-2">
          <Badge tone={running ? "success" : container.state === "paused" ? "warning" : "neutral"} dot>
            {running ? t("Running") : container.state}
          </Badge>
          <span>{container.image}</span>
        </span>
      }
      footer={
        <div className="flex gap-2">
          <Button
            variant="secondary"
            className="flex-1"
            icon={<RotateCw className="size-3.5" />}
            loading={busy}
            onClick={() =>
              action.mutate({
                kind: running ? "container.restart" : "container.start",
                target: container.name,
                success: running
                  ? t("{name} restarted", { name: container.name })
                  : t("{name} started", { name: container.name }),
              })
            }
          >
            {running ? t("Restart") : t("Start")}
          </Button>
          {running ? (
            <Button
              variant="danger"
              className="flex-1"
              onClick={() =>
                action.mutate({
                  kind: "container.stop",
                  target: container.name,
                  success: t("{name} stopped", { name: container.name }),
                })
              }
            >
              {t("Stop")}
            </Button>
          ) : null}
          {container.webUi ? (
            <Button variant="primary" icon={<ExternalLink className="size-3.5" />} onClick={() => window.open(container.webUi, "_blank", "noopener")}>
              {t("Open")}
            </Button>
          ) : null}
        </div>
      }
    >
      <div className="space-y-5">
        <Button
          variant="secondary"
          icon={<Settings2 className="size-3.5" />}
          className="w-full"
          onClick={() => onEdit(container)}
        >
          {t("Edit configuration")}
        </Button>
        <div className="grid grid-cols-2 gap-3">
          <Info label={t("Repository")} value={container.repository} className="col-span-2" />
          <Info label={t("Tag")} value={container.tag} />
          <Info label={t("Version")} value={container.version || "—"} />
          <Info label={t("Category")} value={container.category || "—"} />
          <Info label={t("Restart policy")} value={container.restartPolicy || "—"} />
          <Info label={t("Created")} value={relativeTime(container.created)} />
          <Info label={t("Network")} value={container.network || "—"} />
          <Info label={t("IP address")} value={container.ip || "—"} />
          <Info label={t("Uptime")} value={running ? formatUptime(container.uptime) : "—"} />
          <Info label={t("Memory")} value={formatMemory(container.memory)} />
        </div>

        <label className="flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
          <span>
            <span className="block text-[11px] font-medium text-soft">{t("Start with the array")}</span>
            <span className="block text-[9.5px] text-subtle">{t("Unraid autostart list")}</span>
          </span>
          <Switch
            checked={container.autostart}
            onChange={(enabled) => autostart.mutate(enabled)}
            label={t("Autostart container")}
            disabled={autostart.isPending}
          />
        </label>

        {container.updateAvailable ? (
          <div className="flex items-center justify-between gap-3 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3">
            <p className="flex items-center gap-2 text-[11px] text-[var(--warning)]">
              <TriangleAlert className="size-3.5" /> {t("A newer image is available")}
            </p>
            <Button size="sm" variant="soft" onClick={() => onUpdate(container)}>
              {t("Update now")}
            </Button>
          </div>
        ) : null}

        <div>
          <h3 className="text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Published ports")}</h3>
          {publishedPorts.length === 0 ? (
            <p className="mt-2 text-[11px] text-muted">{t("No ports are published for this container.")}</p>
          ) : (
            <div className="mt-2 overflow-hidden rounded-xl border border-line">
              {publishedPorts.map((port) => (
                <div key={`${port.host}-${port.container}-${port.protocol}`} className="flex items-center justify-between border-b border-line/60 px-3 py-2 text-[11px] last:border-0">
                  <span className="num text-soft">{port.host} → {port.container}</span>
                  <Badge tone="neutral">{port.protocol.toUpperCase()}</Badge>
                </div>
              ))}
            </div>
          )}
        </div>

        <div>
          <div className="flex items-center justify-between">
            <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
              <Terminal className="size-3.5" /> {t("Container logs")}
            </h3>
            <div className="flex items-center gap-1">
              <IconButton label={t("Reload logs")} size="sm" onClick={() => void logs.refetch()}>
                <RefreshCw className={cn("size-3.5", logs.isFetching && "animate-spin")} />
              </IconButton>
              <IconButton label={t("Copy logs")} size="sm" onClick={() => void copyLogs()}>
                {copied ? <Check className="size-3.5 text-[var(--success)]" /> : <Copy className="size-3.5" />}
              </IconButton>
            </div>
          </div>
          <div
            ref={logRef}
            className="mt-2 h-64 overflow-y-auto rounded-xl border border-[#23252e] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed"
          >
            {logs.isLoading ? (
              <p className="text-[#6b7280]">{t("Loading logs…")}</p>
            ) : logs.data?.length ? (
              logs.data.map((line, index) => (
                <div key={`${line.time}-${index}`} className="flex gap-2">
                  <span className="shrink-0 text-[#4b5563]">{new Date(line.time).toLocaleTimeString([], { hour12: false })}</span>
                  <span
                    className={cn(
                      "shrink-0 uppercase",
                      line.level === "error" ? "text-[#ff6b81]" : line.level === "warn" ? "text-[#f5b44a]" : "text-[#5aa9ff]",
                    )}
                  >
                    {line.level}
                  </span>
                  <span className="text-[#c7cbd6]">{line.message}</span>
                </div>
              ))
            ) : (
              <p className="text-[#6b7280]">{t("No logs available.")}</p>
            )}
          </div>
        </div>
      </div>
    </Drawer>
  );
}

function MetricCard({
  icon,
  label,
  value,
  hint,
  tone,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint: string;
  tone?: "warning" | "success";
}) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className={cn("num mt-3 text-[21px] font-semibold leading-none", tone === "warning" ? "text-[var(--warning)]" : tone === "success" ? "text-[var(--success)]" : "text-ink")}>
        {value}
      </p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}

function Info({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={cn("rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2", className)}>
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-1 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}
