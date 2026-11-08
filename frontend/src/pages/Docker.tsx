import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowDownToLine,
  Boxes,
  Cable,
  Check,
  ChevronLeft,
  ChevronsDownUp,
  ChevronsUpDown,
  CloudDownload,
  ChevronDown,
  Cpu,
  ExternalLink,
  Folder as FolderIcon,
  Globe2,
  HardDrive,
  Info as InfoIcon,
  Layers,
  List,
  ListOrdered,
  MemoryStick,
  Network,
  Play,
  Plus,
  RefreshCw,
  RotateCw,
  ScanSearch,
  Search,
  Settings2,
  ShieldAlert,
  ShieldCheck,
  Square,
  Terminal,
  Trash2,
  TriangleAlert,
  Waypoints,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { ContainerCreateModal } from "../components/containers/ContainerCreate";
import { ContainerUpdateModal, type ContainerUpdateRun } from "../components/containers/ContainerUpdateModal";
import { ContainerSettingsModal } from "../components/containers/ContainerSettings";
import { ContainerMap } from "../components/containers/ContainerMap";
import { ContainerConnections } from "../components/containers/ContainerConnections";
import { ContainerNetworkMap } from "../components/containers/ContainerNetworkMap";
import { ContainerPriorityModal } from "../components/containers/ContainerPriorityModal";
import { ContainerImages } from "../components/containers/ContainerImages";
import { ContainerSecurityOverview, type SecurityGroupBy } from "../components/containers/ContainerSecurityOverview";
import { SecurityCounts, SEVERITY_STYLES } from "../components/containers/security";
import { EntityIcon, containerIconURL } from "../components/ui/entity-icon";
import { LogViewer } from "../components/ui/log-viewer";
import { FolderDropdown } from "../components/folders/FolderDropdown";
import { FolderCard } from "../components/folders/FolderCard";
import { FolderTable } from "../components/folders/FolderTable";
import { FolderManager } from "../components/folders/FolderManager";
import { useVisualFolders, type VisualFolderState } from "../components/folders/use-visual-folders";
import { FolderPicker } from "../components/folders/FolderPicker";
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
import type { Container, ContainerSecurity, ContainerSecurityDetail } from "../lib/types";

type Filter = "all" | "running" | "stopped" | "updates";

export function DockerPage() {
  const { snapshot, refresh } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const action = useAction();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [filter, setFilter] = useState<Filter>("all");
  const [folderFilter, setFolderFilter] = useState("all");
  const [folderManager, setFolderManager] = useState(false);
  const [priorityOpen, setPriorityOpen] = useState(false);
  const [collapsedFolders, setCollapsedFolders] = useState<Set<string>>(new Set());
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set());
  const visualFolders = useVisualFolders("containers");
  useEffect(() => {
    if (folderFilter === "all" || folderFilter === "none") return;
    if (!visualFolders.folders.some((folder) => folder.id === folderFilter)) setFolderFilter("all");
  }, [folderFilter, visualFolders.folders]);
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
  const [selecting, setSelecting] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    try {
      localStorage.setItem("aurora.docker.view", view);
    } catch {
      // Storage can be unavailable; the view still works for this session.
    }
  }, [view]);

  const tabParam = params.get("tab");
  const tab =
    tabParam === "map"
      ? "map"
      : tabParam === "connections" || tabParam === "network"
        ? "network"
        : tabParam === "images"
          ? "images"
          : tabParam === "security"
            ? "security"
            : "containers";
  const setTab = (value: string) => {
    const next = new URLSearchParams(params);
    if (value === "map" || value === "network" || value === "images" || value === "security") {
      next.set("tab", value);
      if (value !== "network") next.delete("sub");
    } else {
      next.delete("tab");
      next.delete("sub");
    }
    setParams(next, { replace: true });
  };
  const securityGroupParam = params.get("securityGroup");
  const securityGroup: SecurityGroupBy | undefined =
    securityGroupParam === "image" ||
    securityGroupParam === "container" ||
    securityGroupParam === "cve" ||
    securityGroupParam === "package" ||
    securityGroupParam === "severity"
      ? securityGroupParam
      : undefined;
  const networkView = params.get("sub") === "map" ? "map" : "calls";
  const setNetworkView = (value: string) => {
    const next = new URLSearchParams(params);
    next.set("tab", "network");
    next.set("sub", value === "map" ? "map" : "calls");
    setParams(next, { replace: true });
  };

  const selectedName = params.get("container");
  const selected = snapshot?.containers.find((container) => container.name === selectedName) ?? null;

  const containers = snapshot?.containers ?? [];
  const running = containers.filter((container) => container.state === "running");
  const stopped = containers.filter((container) => container.state !== "running");
  const updates = containers.filter((container) => container.updateAvailable);

  const folderCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const container of containers) {
      const folderID = visualFolders.assignment(container.name) || "none";
      counts[folderID] = (counts[folderID] ?? 0) + 1;
    }
    return counts;
  }, [containers, visualFolders.assignments, visualFolders.folders]);

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
      .filter((container) => {
        if (folderFilter === "all") return true;
        const folderID = visualFolders.assignment(container.name);
        if (folderFilter === "none") return !folderID;
        return folderID === folderFilter;
      })
      .sort((a, b) => {
        if (a.state !== b.state) return a.state === "running" ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
  }, [containers, filter, query, folderFilter, visualFolders.assignments, visualFolders.folders]);

  const folderEntries = visualFolders.folders.map((folder) => ({
    folder,
    items: filtered.filter((container) => visualFolders.assignment(container.name) === folder.id),
  }));
  const showFolders = folderFilter === "all";
  const entries = showFolders
    ? [
        ...folderEntries
          .filter(({ items }) => items.length > 0 || (filter === "all" && !query.trim()))
          .map(({ folder, items }) => ({ kind: "folder" as const, folder, items })),
        ...filtered
          .filter((container) => !visualFolders.assignment(container.name))
          .map((container) => ({ kind: "container" as const, container })),
      ]
    : filtered.map((container) => ({ kind: "container" as const, container }));

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
      startedAt: Date.now(),
      events: [],
      running: true,
      error: "",
      done: false,
    });
    for (let index = 0; index < targets.length; index += 1) {
      setUpdateRun((current) => (current ? { ...current, index, startedAt: Date.now(), events: [] } : current));
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

  const trivy = useQuery({
    queryKey: ["trivy-status"],
    queryFn: api.trivyStatus,
    refetchInterval: 30_000,
    staleTime: 30_000,
  });
  const securityReady = Boolean(
    trivy.data?.enabled &&
    trivy.data?.installed &&
    trivy.data?.runtimeReady &&
    trivy.data?.dbReady &&
    trivy.data?.javaDbReady &&
    !trivy.data?.installing &&
    !trivy.data?.updatingDb,
  );
  const scanAll = useMutation({
    mutationFn: api.scanAllContainers,
    onSuccess: (result) => {
      toast({
        kind: "info",
        title: t("Security scans started"),
        description: t("{count} images queued", { count: result.started }),
      });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["security-overview"] });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not start the security scans"), description: error.message }),
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

  const bulk = useMutation({
    mutationFn: ({ action, targets }: { action: "start" | "stop" | "restart"; targets: string[] }) =>
      api.bulkContainerAction(action, targets),
    onSuccess: (result) => {
      const failed = result.failed?.length ?? 0;
      toast({
        kind: failed ? "warning" : "success",
        title: failed ? t("Some containers could not be updated") : t("Bulk action completed"),
        description: t("{count} containers updated", { count: result.succeeded?.length ?? 0 }),
      });
      setSelectedIds(new Set());
      setSelecting(false);
      void refresh();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Bulk action failed"), description: error.message }),
  });

  const toggleSelected = (id: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const runBulk = (action: "start" | "stop" | "restart") => {
    const targets = containers.filter((container) => selectedIds.has(container.id)).map((container) => container.name);
    if (targets.length) bulk.mutate({ action, targets });
  };

  const renderContainerCard = (container: Container) => (
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
      selectable={selecting}
      selected={selectedIds.has(container.id)}
      onSelect={() => toggleSelected(container.id)}
      folderName={visualFolders.folders.find((folder) => folder.id === visualFolders.assignment(container.name))?.name}
    />
  );

  const renderContainerRow = (container: Container) => (
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
      selectable={selecting}
      selected={selectedIds.has(container.id)}
      onSelect={() => toggleSelected(container.id)}
    />
  );

  const unfiledItems = filtered.filter((container) => !visualFolders.assignment(container.name));
  const tableItems = showFolders ? unfiledItems : filtered;
  const allFoldersCollapsed =
    folderEntries.length > 0 && folderEntries.every(({ folder }) => collapsedFolders.has(folder.id));
  const allFoldersExpanded =
    folderEntries.length > 0 && folderEntries.every(({ folder }) => expandedFolders.has(folder.id));
  const foldersAllOpen = view === "list" ? !allFoldersCollapsed : allFoldersExpanded;
  const toggleAllFolders = () => {
    const ids = folderEntries.map(({ folder }) => folder.id);
    if (view === "list") {
      setCollapsedFolders(allFoldersCollapsed ? new Set() : new Set(ids));
      return;
    }
    setExpandedFolders(allFoldersExpanded ? new Set() : new Set(ids));
  };
  const toggleFolder = (id: string) => {
    if (view === "list") {
      setCollapsedFolders((current) => {
        const next = new Set(current);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
      });
      return;
    }
    setExpandedFolders((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const containerColumns = (
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
      <span className="hidden w-[176px] shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle min-[1500px]:block">{t("Security")}</span>
      <span className="w-20 shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("State")}</span>
      <span className="hidden w-16 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle sm:block">CPU</span>
      <span className="hidden w-20 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle sm:block">{t("Memory")}</span>
      <span className="hidden w-16 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle md:block">{t("Uptime")}</span>
      <span className="w-[176px] shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Actions")}</span>
    </div>
  );

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
            <Button
              variant="secondary"
              icon={<ListOrdered className="size-3.5" />}
              disabled={containers.length === 0}
              onClick={() => setPriorityOpen(true)}
            >
              {t("Priorities")}
            </Button>
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
              icon={<ScanSearch className="size-3.5" />}
              loading={scanAll.isPending}
              disabled={!securityReady || containers.length === 0 || scanAll.isPending}
              title={!securityReady ? t("Enable image scanning in Settings") : undefined}
              onClick={() => scanAll.mutate()}
            >
              {t("Scan images")}
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

      <div className="mb-5 flex flex-wrap items-center gap-2">
        <Tabs
          items={[
            { value: "containers", label: t("Containers"), count: containers.length },
            { value: "map", label: t("Resource map"), icon: <Network className="size-3.5" />, count: running.length },
            { value: "network", label: t("Network"), icon: <Waypoints className="size-3.5" />, count: running.length },
            { value: "images", label: t("Images"), icon: <Layers className="size-3.5" /> },
            { value: "security", label: t("Security"), icon: <ShieldCheck className="size-3.5" /> },
          ]}
          value={tab}
          onChange={setTab}
        />
      </div>

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

      {tab === "map" ? (
        <ContainerMap containers={running} onOpen={openContainer} />
      ) : tab === "network" ? (
        <div className="space-y-4">
          <Tabs
            size="sm"
            items={[
              { value: "calls", label: t("Calls"), icon: <Globe2 className="size-3.5" /> },
              { value: "map", label: t("Map"), icon: <Waypoints className="size-3.5" />, count: running.length },
            ]}
            value={networkView}
            onChange={setNetworkView}
          />
          {networkView === "map" ? (
            <ContainerNetworkMap containers={running} />
          ) : (
            <ContainerConnections containers={running} />
          )}
        </div>
      ) : tab === "images" ? (
        <ContainerImages />
      ) : tab === "security" ? (
        <ContainerSecurityOverview
          containers={containers}
          onOpenContainer={openContainer}
          initialQuery={params.get("securityQuery") ?? ""}
          initialGroup={securityGroup}
        />
      ) : (
        <>
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
        <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
          <FolderDropdown
            folders={visualFolders.folders}
            selected={folderFilter}
            onSelect={setFolderFilter}
            total={containers.length}
            ungrouped={folderCounts.none ?? 0}
            counts={folderCounts}
            onManage={() => setFolderManager(true)}
          />
          {showFolders && folderEntries.length ? (
            <Button
              size="sm"
              variant="secondary"
              icon={foldersAllOpen ? <ChevronsDownUp className="size-3.5" /> : <ChevronsUpDown className="size-3.5" />}
              onClick={toggleAllFolders}
              title={foldersAllOpen ? t("Collapse all") : t("Expand all")}
              aria-label={foldersAllOpen ? t("Collapse all") : t("Expand all")}
            >
              <span className="hidden xl:inline">{foldersAllOpen ? t("Collapse all") : t("Expand all")}</span>
            </Button>
          ) : null}
          <InputBase
            value={query}
            onChange={setQuery}
            placeholder={t("Search containers…")}
            icon={<Search className="size-3.5" />}
            className="w-52"
          />
          <Button
            size="sm"
            variant={selecting ? "primary" : "secondary"}
            icon={<Check className="size-3.5" />}
            onClick={() => {
              setSelecting((value) => !value);
              setSelectedIds(new Set());
            }}
          >
            {selecting ? t("Cancel selection") : t("Select")}
          </Button>
          <div data-glass-control="segmented" className="flex items-center rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
            <button
              type="button"
              onClick={() => setView("grid")}
              aria-label={t("Grid view")}
              aria-pressed={view === "grid"}
              className={cn("grid size-7 place-items-center rounded-md transition", view === "grid" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
            >
              <Boxes className="size-3.5" />
            </button>
            <button
              type="button"
              onClick={() => setView("list")}
              aria-label={t("List view")}
              aria-pressed={view === "list"}
              className={cn("grid size-7 place-items-center rounded-md transition", view === "list" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
            >
              <List className="size-3.5" />
            </button>
          </div>
        </div>
      </div>

      {selecting ? (
        <div className="mb-3 flex flex-wrap items-center gap-2 rounded-xl border border-[var(--accent-line)] bg-[var(--accent-soft)] px-3 py-2">
          <span className="text-[10.5px] font-medium text-soft">
            {t("{count} selected", { count: selectedIds.size })}
          </span>
          <Button size="sm" variant="ghost" onClick={() => setSelectedIds(new Set(filtered.map((container) => container.id)))}>
            {t("Select all")}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setSelectedIds(new Set())}>
            {t("Clear selection")}
          </Button>
          <div className="ml-auto flex flex-wrap items-center gap-2">
            <Button size="sm" variant="secondary" icon={<Play className="size-3" />} disabled={!selectedIds.size || bulk.isPending} title={t("Starts containers by priority, lowest number first")} onClick={() => runBulk("start")}>
              {t("Start")}
            </Button>
            <Button size="sm" variant="secondary" icon={<RotateCw className="size-3" />} disabled={!selectedIds.size || bulk.isPending} onClick={() => runBulk("restart")}>
              {t("Restart")}
            </Button>
            <Button size="sm" variant="danger" icon={<Square className="size-3" />} disabled={!selectedIds.size || bulk.isPending} loading={bulk.isPending} onClick={() => runBulk("stop")}>
              {t("Stop")}
            </Button>
          </div>
        </div>
      ) : null}

      {folderFilter !== "all" && folderFilter !== "none" ? (
        <div className="mb-3 flex items-center gap-2">
          <Button size="sm" variant="ghost" icon={<ChevronLeft className="size-3.5" />} onClick={() => setFolderFilter("all")}>
            {t("All folders")}
          </Button>
          <span className="text-[11px] font-medium text-soft">
            {visualFolders.folders.find((folder) => folder.id === folderFilter)?.name}
          </span>
        </div>
      ) : null}

      {entries.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Boxes className="size-5" />}
            title={t("No containers match this view")}
            description={t("Adjust the filter or search query to see your workloads.")}
            action={
              <Button variant="secondary" onClick={() => { setFilter("all"); setQuery(""); setFolderFilter("all"); }}>
                {t("Clear filters")}
              </Button>
            }
          />
        </Card>
      ) : view === "grid" ? (
        <div className="stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {entries.map((entry) =>
            entry.kind === "folder" ? (
              <FolderCard
                key={`folder-${entry.folder.id}`}
                name={entry.folder.name}
                count={entry.items.length}
                open={expandedFolders.has(entry.folder.id)}
                onToggle={() => toggleFolder(entry.folder.id)}
              >
                {entry.items.map(renderContainerCard)}
              </FolderCard>
            ) : (
              renderContainerCard(entry.container)
            ),
          )}
        </div>
      ) : (
        <>
          {showFolders
            ? folderEntries.map(({ folder, items }) => (
                <FolderTable
                  key={folder.id}
                  name={folder.name}
                  count={items.length}
                  columns={containerColumns}
                  open={!collapsedFolders.has(folder.id)}
                  onToggle={() => toggleFolder(folder.id)}
                >
                  {items.length ? (
                    items.map(renderContainerRow)
                  ) : (
                    <p className="px-4 py-3 text-[10.5px] text-subtle">{t("No items")}</p>
                  )}
                </FolderTable>
              ))
            : null}
          {tableItems.length > 0 || !showFolders ? (
            <Card className="overflow-hidden">
              {showFolders && folderEntries.length ? (
                <div className="flex flex-wrap items-center gap-3 border-b border-line bg-[var(--surface-2)]/50 px-4 py-2.5">
                  <span className="grid size-7 shrink-0 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">
                    <Boxes className="size-3.5" />
                  </span>
                  <span className="text-[12px] font-semibold text-ink">{t("Unfiled")}</span>
                  <span className="font-mono text-[9px] text-subtle">{t("{count} items", { count: tableItems.length })}</span>
                </div>
              ) : null}
              {containerColumns}
              <div className="divide-y divide-line/60">
                {tableItems.map(renderContainerRow)}
              </div>
            </Card>
          ) : null}
        </>
      )}
        </>
      )}

      {tab !== "network" && tab !== "images" ? (
        <ContainerDrawer
          container={selected}
          onClose={closeContainer}
          onEdit={(container) => {
            setEditing(container);
            closeContainer();
          }}
          onUpdate={(container) => void runUpdates([container])}
          visualFolders={visualFolders}
          busy={action.isPending && action.variables?.target === selected?.name}
        />
      ) : null}

      <ContainerSettingsModal container={editing} onClose={() => setEditing(null)} />

      <ContainerCreateModal open={creating} onClose={() => setCreating(false)} />
      <ContainerPriorityModal open={priorityOpen} onClose={() => setPriorityOpen(false)} containers={containers} />
      <FolderManager
        open={folderManager}
        onClose={() => setFolderManager(false)}
        folders={visualFolders.folders}
        assignments={visualFolders.assignments}
        counts={folderCounts}
        onSave={visualFolders.saveFolders}
      />

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

function CardSection({
  icon,
  label,
  summary,
  children,
}: {
  icon: ReactNode;
  label: string;
  summary: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="flex w-full items-center gap-1.5 py-2 text-left transition hover:text-ink"
      >
        <span className="text-subtle">{icon}</span>
        <span className="text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{label}</span>
        <span className="ml-auto text-[9px] text-subtle">{summary}</span>
        <ChevronDown className={cn("size-3 text-subtle transition-transform", open && "rotate-180")} />
      </button>
      {open ? <div className="pb-2.5">{children}</div> : null}
    </div>
  );
}

// "Healthy" is the normal state and adds noise; only surface problems.
function ContainerHealthBadge({ health, className }: { health?: string; className?: string }) {
  const { t } = useI18n();
  if (!health || health === "healthy") return null;
  const unhealthy = health === "unhealthy";
  return (
    <Badge tone={unhealthy ? "danger" : "warning"} className={className}>
      {t(unhealthy ? "Unhealthy" : "Starting")}
    </Badge>
  );
}

function ContainerCard({
  container,
  onOpen,
  onEdit,
  onUpdate,
  onRemove,
  busy,
  selectable,
  selected,
  onSelect,
  folderName,
}: {
  container: Container;
  onOpen: () => void;
  onEdit: () => void;
  onUpdate: () => void;
  onRemove: () => void;
  busy?: boolean;
  selectable?: boolean;
  selected?: boolean;
  onSelect?: () => void;
  folderName?: string;
}) {
  const action = useAction();
  const { t } = useI18n();
  const running = container.state === "running";
  const ports = (container.ports ?? []).filter((port) => port.host > 0);
  const mounts = container.mounts ?? [];
  const memoryUsage = container.memoryLimit ? (container.memory / container.memoryLimit) * 100 : 0;
  const security = container.security;
  const securitySummary = !security?.enabled
    ? t("Disabled")
    : security.scanning
      ? t("Scanning")
      : security.error
        ? t("Failed")
        : !security.available
          ? t("Not scanned")
          : security.total === 0
            ? "0"
            : t("{count} findings", { count: security.total });

  return (
    <Card hover className="card-pad group relative overflow-hidden">
      <div
        className="pointer-events-none absolute -right-16 -top-20 size-48 rounded-full blur-3xl"
        style={{ background: hexToRgba(container.color, 0.14) }}
      />
      <div className="relative flex items-start justify-between gap-3">
        {selectable ? (
          <input
            type="checkbox"
            checked={Boolean(selected)}
            onChange={onSelect}
            aria-label={t("Select {name}", { name: container.name })}
            className="mt-1 size-4 shrink-0 accent-[var(--accent)]"
          />
        ) : null}
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
            {folderName ? (
              <span className="mt-1 inline-flex max-w-full items-center gap-1 truncate rounded border border-line bg-[var(--surface-2)] px-1.5 py-0.5 text-[9px] text-subtle">
                <FolderIcon className="size-2.5 shrink-0" /> {folderName}
              </span>
            ) : null}
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
            {
              label: t("Force update"),
              icon: <ArrowDownToLine className="size-3.5" />,
              onSelect: onUpdate,
            },
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

      <div className="relative mt-4 flex flex-wrap items-center gap-2">
        <Badge tone={running ? "success" : container.state === "paused" ? "warning" : "neutral"} dot pulse={running}>
          {running ? t("Running") : container.state}
        </Badge>
        <ContainerHealthBadge health={container.health} />
        <span className="text-[10px] text-subtle">{running ? t("Up {time}", { time: formatUptime(container.uptime) }) : container.status}</span>
        <div className="ml-auto flex items-center gap-2">
          {container.updateAvailable ? <Badge tone="warning">{t("Update")}</Badge> : null}
          <Button
            size="sm"
            variant="secondary"
            className="!border-white/20 !bg-white !text-[#101116] hover:!bg-white/90"
            icon={<ArrowDownToLine className="size-3" />}
            disabled={busy}
            onClick={onUpdate}
          >
            {t("Force update")}
          </Button>
        </div>
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

      <div className="relative mt-3 divide-y divide-line/60 border-t border-line/60">
        <CardSection
          icon={<Cable className="size-3" />}
          label={t("Published ports")}
          summary={ports.length > 0 ? String(ports.length) : t("None")}
        >
          <div className="flex flex-wrap items-center gap-1.5">
            {ports.slice(0, 6).map((port) => (
              <span key={`${port.host}-${port.container}-${port.protocol}`} className="chip font-mono text-[9.5px]">
                {port.host}:{port.container}/{port.protocol}
              </span>
            ))}
            {ports.length > 6 ? <span className="text-[9.5px] text-subtle">+{ports.length - 6}</span> : null}
            {ports.length === 0 ? <span className="text-[9.5px] text-subtle">{t("No published ports")}</span> : null}
          </div>
        </CardSection>

        <CardSection
          icon={<Network className="size-3" />}
          label={t("Network")}
          summary={container.network || "—"}
        >
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[9.5px] text-subtle">
            <span className="flex items-center gap-1">
              <Network className="size-3" /> {container.network || "—"}
            </span>
            <span className="font-mono">{container.ip || t("No address")}</span>
          </div>
        </CardSection>

        <CardSection
          icon={<HardDrive className="size-3" />}
          label={t("Mounts")}
          summary={mounts.length > 0 ? String(mounts.length) : t("None")}
        >
          {mounts.length === 0 ? (
            <span className="text-[9.5px] text-subtle">{t("No mounts")}</span>
          ) : (
            <div className="space-y-1">
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
          )}
        </CardSection>

        <CardSection icon={<ShieldCheck className="size-3" />} label={t("Security")} summary={securitySummary}>
          <SecurityBreakdown security={security} />
        </CardSection>
      </div>

      <div className="relative mt-4 flex items-center justify-end gap-2 border-t border-line pt-3">
        <div className="flex items-center gap-1">
          <IconButton
            label={t("Force update")}
            size="sm"
            disabled={busy}
            onClick={onUpdate}
          >
            <ArrowDownToLine className="size-3.5 text-soft" />
          </IconButton>
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
  selectable,
  selected,
  onSelect,
  folderName,
}: {
  container: Container;
  onOpen: () => void;
  onEdit: () => void;
  onUpdate: () => void;
  onRemove: () => void;
  busy?: boolean;
  selectable?: boolean;
  selected?: boolean;
  onSelect?: () => void;
  folderName?: string;
}) {
  const action = useAction();
  const { t } = useI18n();
  const running = container.state === "running";
  const mounts = container.mounts ?? [];
  return (
    <div className="flex items-center gap-3 px-4 py-3 transition hover:bg-[var(--surface-2)]/50">
      {selectable ? (
        <input
          type="checkbox"
          checked={Boolean(selected)}
          onChange={onSelect}
          aria-label={t("Select {name}", { name: container.name })}
          className="size-4 shrink-0 accent-[var(--accent)]"
        />
      ) : null}
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
          {folderName ? (
            <span className="hidden items-center gap-1 rounded border border-line bg-[var(--surface-2)] px-1 py-0.5 text-[8px] text-subtle lg:inline-flex">
              <FolderIcon className="size-2.5" /> {folderName}
            </span>
          ) : null}
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
      <span className="hidden w-[176px] shrink-0 min-[1500px]:block"><SecurityCounts security={container.security} /></span>
      <span className="w-20 shrink-0">
        <Badge tone={running ? "success" : "neutral"} dot>{running ? t("Running") : container.state}</Badge>
        <ContainerHealthBadge health={container.health} className="ml-1" />
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
  visualFolders,
  busy,
}: {
  container: Container | null;
  onClose: () => void;
  onEdit: (container: Container) => void;
  onUpdate: (container: Container) => void;
  visualFolders: VisualFolderState;
  busy?: boolean;
}) {
  const action = useAction();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
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
        <div className="grid gap-2 sm:grid-cols-3">
          <Button
            variant="secondary"
            icon={<Settings2 className="size-3.5" />}
            onClick={() => onEdit(container)}
          >
            {t("Edit configuration")}
          </Button>
          <Button
            variant="secondary"
            icon={<Terminal className="size-3.5" />}
            onClick={() => window.open(`/docker/terminal/${encodeURIComponent(container.id)}`, "_blank", "noopener")}
          >
            {t("Open terminal")}
          </Button>
          <Button
            variant="secondary"
            className="!border-white/20 !bg-white !text-[#101116] hover:!bg-white/90"
            icon={<ArrowDownToLine className="size-3.5" />}
            onClick={() => onUpdate(container)}
          >
            {t("Force update")}
          </Button>
        </div>
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

        <div className="rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
          <FolderPicker
            folders={visualFolders.folders}
            value={visualFolders.assignment(container.name)}
            onChange={(folderID) => visualFolders.setAssignment(container.name, folderID)}
            onCreate={visualFolders.createFolder}
            label={t("Folder")}
            disabled={visualFolders.loading}
          />
          <p className="mt-1 text-[9.5px] text-subtle">
            {t("Visual only. Saved immediately without recreating the container.")}
          </p>
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

        <ContainerSecuritySection container={container} />

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

        <ContainerConnections container={container} compact />

        <LogViewer
          lines={logs.data}
          loading={logs.isLoading}
          refreshing={logs.isFetching}
          onRefresh={() => void logs.refetch()}
          downloadName={`${container.name}.log`}
          title={t("Container logs")}
        />
      </div>
    </Drawer>
  );
}

function SecurityBreakdown({ security }: { security?: ContainerSecurity }) {
  const { t } = useI18n();
  if (!security?.enabled) {
    return <p className="text-[9.5px] leading-relaxed text-subtle">{t("Image scanning is disabled. Enable Trivy in Settings to inspect this image.")}</p>;
  }
  if (security.scanning) return <p className="text-[9.5px] text-[var(--info)]">{t("Scanning…")}</p>;
  if (security.error) return <p className="text-[9.5px] leading-relaxed text-[var(--danger)]">{security.error}</p>;
  if (!security.available) return <p className="text-[9.5px] leading-relaxed text-subtle">{t("No scan result yet. A scan starts automatically after an image pull.")}</p>;
  return (
    <div className="space-y-2">
      <SecurityCounts security={security} showLabel />
      {security.total === 0 ? <p className="text-[9.5px] text-[var(--success)]">{t("No known vulnerabilities were found for this image.")}</p> : null}
      <div className="flex flex-wrap gap-x-3 gap-y-1 text-[9px] text-subtle">
        {security.scannedAt ? <span>{t("Scanned {time}", { time: new Date(security.scannedAt).toLocaleString() })}</span> : null}
        {security.stale ? <span className="text-[var(--warning)]">{t("Database updated since this scan")}</span> : null}
      </div>
    </div>
  );
}

type SecurityFilter = "all" | "critical" | "high" | "medium" | "low" | "unknown";

function ContainerSecuritySection({ container }: { container: Container }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<SecurityFilter>("all");
  const enabled = Boolean(container.security?.enabled);
  const security = useQuery<ContainerSecurityDetail>({
    queryKey: ["container-security", container.id],
    queryFn: () => api.containerSecurity(container.id),
    enabled,
    refetchInterval: (query) => (query.state.data?.scanning ? 4000 : false),
  });
  const scan = useMutation({
    mutationFn: () => api.scanContainer(container.id),
    onSuccess: () => {
      toast({ kind: "info", title: t("Image scan started") });
      void security.refetch();
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["security-overview"] });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not start the image scan"), description: error.message }),
  });

  if (!enabled) {
    return (
      <div className="flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/45 p-3 text-[10.5px] leading-relaxed text-muted">
        <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
        {t("Image scanning is disabled. Enable Trivy in Settings to inspect this image.")}
      </div>
    );
  }

  const summary = security.data ?? container.security;
  const findings = (security.data?.vulnerabilities ?? []).filter((finding) => {
    if (filter === "all") return true;
    if (filter === "unknown") return !["CRITICAL", "HIGH", "MEDIUM", "LOW"].includes(finding.severity);
    return finding.severity === filter.toUpperCase();
  });
  const visible = findings.slice(0, 200);
  const scanning = summary.scanning || scan.isPending;
  const counts: { key: SecurityFilter; label: string; value: number }[] = [
    { key: "critical", label: t("Critical"), value: summary.critical },
    { key: "high", label: t("High"), value: summary.high },
    { key: "medium", label: t("Medium"), value: summary.medium },
    { key: "low", label: t("Low"), value: summary.low },
    { key: "unknown", label: t("Unknown"), value: summary.unknown },
  ];
  const severityText = (key: SecurityFilter) =>
    key === "critical"
      ? "text-[var(--danger)]"
      : key === "high"
        ? "text-orange-600 dark:text-orange-400"
        : key === "medium"
          ? "text-yellow-700 dark:text-yellow-300"
          : key === "low"
            ? "text-[var(--info)]"
            : "text-subtle";
  const stateTone = scanning ? "info" : summary.error ? "danger" : !summary.available ? "neutral" : summary.critical > 0 ? "danger" : summary.high > 0 ? "warning" : summary.total > 0 ? "info" : "success";
  const stateLabel = scanning
    ? t("Scanning")
    : summary.error
      ? t("Failed")
      : !summary.available
        ? t("Not scanned")
        : summary.critical > 0
          ? t("{count} critical", { count: summary.critical })
          : summary.high > 0
            ? t("{count} high", { count: summary.high })
            : summary.total > 0
              ? t("{count} findings", { count: summary.total })
              : "0";

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
          <ShieldCheck className="size-3.5" /> {t("Image security")}
        </h3>
        <div className="flex items-center gap-1.5">
          <Badge tone={stateTone} dot pulse={scanning}>{stateLabel}</Badge>
          <Button
            size="sm"
            variant="secondary"
            icon={<ScanSearch className="size-3.5" />}
            loading={scanning}
            disabled={scanning}
            onClick={() => scan.mutate()}
          >
            {scanning ? t("Scanning…") : t("Scan now")}
          </Button>
        </div>
      </div>

      {summary.error ? (
        <p className="mt-2 flex items-start gap-1.5 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-2.5 text-[10px] leading-relaxed text-[var(--danger)]">
          <ShieldAlert className="mt-0.5 size-3.5 shrink-0" /> {summary.error}
        </p>
      ) : null}

      {!summary.available && !scanning && !summary.error ? (
        <p className="mt-2 rounded-xl border border-line bg-[var(--surface-2)]/40 p-3 text-[10.5px] leading-relaxed text-muted">
          {t("No scan result yet. A scan starts automatically after an image pull.")}
        </p>
      ) : null}

      {summary.available && !summary.error ? (
        <>
          <div className="mt-3 grid grid-cols-3 gap-1.5 sm:grid-cols-5">
            {counts.map((count) => (
              <div key={count.key} className="rounded-lg border border-line bg-[var(--surface-2)]/40 px-2 py-1.5 text-center">
                <p className={cn("num text-[14px] font-semibold", count.value > 0 ? severityText(count.key) : "text-subtle")}>{count.value}</p>
                <p className="text-[8px] uppercase tracking-[0.08em] text-subtle">{count.label}</p>
              </div>
            ))}
          </div>
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[9px] text-subtle">
            {summary.scannedAt ? <span>{t("Scanned {time}", { time: new Date(summary.scannedAt).toLocaleString() })}</span> : null}
            {summary.trivyVersion ? <span className="font-mono">Trivy v{summary.trivyVersion}</span> : null}
            {summary.dbUpdatedAt ? <span>{t("Database {time}", { time: new Date(summary.dbUpdatedAt).toLocaleString() })}</span> : null}
            {summary.stale ? <span className="text-[var(--warning)]">{t("Database updated since this scan")}</span> : null}
          </div>
          {summary.total === 0 ? (
            <p className="mt-3 flex items-center gap-1.5 text-[10px] text-[var(--success)]">
              <ShieldCheck className="size-3.5" /> {t("No known vulnerabilities were found for this image.")}
            </p>
          ) : (
            <div className="mt-3">
              <div className="flex flex-wrap gap-1">
                {([["all", t("All"), summary.total], ...counts.map((count) => [count.key, count.label, count.value] as const)] as const).map(([key, label, count]) => (
                  <button
                    key={key}
                    type="button"
                    onClick={() => setFilter(key as SecurityFilter)}
                    className={cn(
                      "rounded-md border px-2 py-1 text-[9px] font-medium transition",
                      filter === key ? "border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)]" : "border-line text-subtle hover:text-ink",
                    )}
                  >
                    {label} <span className="font-mono">{count}</span>
                  </button>
                ))}
              </div>
              <div className="mt-2 max-h-72 overflow-y-auto rounded-xl border border-line">
                {visible.length === 0 ? (
                  <p className="px-3 py-4 text-center text-[10px] text-subtle">{t("No finding matches this filter.")}</p>
                ) : (
                  visible.map((finding, index) => (
                    <div key={`${finding.id}-${finding.package}-${index}`} className="flex items-start gap-2 border-b border-line/60 px-3 py-2 last:border-0">
                      <span
                        className={cn(
                          "inline-flex h-[19px] shrink-0 items-center rounded-full border px-1.5 font-mono text-[8.5px] font-semibold",
                          SEVERITY_STYLES[
                            finding.severity === "CRITICAL"
                              ? "critical"
                              : finding.severity === "HIGH"
                                ? "high"
                                : finding.severity === "MEDIUM"
                                  ? "medium"
                                  : finding.severity === "LOW"
                                    ? "low"
                                    : "unknown"
                          ],
                        )}
                      >
                        {finding.severity}
                      </span>
                      <div className="min-w-0 flex-1">
                        <p className="font-mono text-[9.5px] text-soft">{finding.id}</p>
                        <p className="mt-0.5 text-[9.5px] text-subtle">
                          {finding.package} {finding.installedVersion}{finding.fixedVersion ? ` → ${finding.fixedVersion}` : ""}
                        </p>
                        <p className="truncate text-[9px] text-subtle" title={finding.title || finding.target}>{finding.title || finding.target}</p>
                      </div>
                      {finding.url ? (
                        <a href={finding.url} target="_blank" rel="noreferrer" className="mt-0.5 text-subtle hover:text-ink" aria-label={t("Open vulnerability details")}>
                          <ExternalLink className="size-3" />
                        </a>
                      ) : null}
                    </div>
                  ))
                )}
              </div>
              {findings.length > visible.length ? (
                <p className="mt-1.5 text-[9px] text-subtle">{t("Showing the first {shown} of {count} findings.", { shown: visible.length, count: findings.length })}</p>
              ) : null}
            </div>
          )}
        </>
      ) : null}
    </div>
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
