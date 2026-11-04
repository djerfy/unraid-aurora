import { useDeferredValue, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  ArrowRight,
  Download,
  ExternalLink,
  Package,
  Plug,
  RefreshCw,
  Search,
  ShoppingBag,
  ShieldCheck,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Drawer, Modal } from "../components/ui/overlays";
import { Badge, Button, Card, EmptyState, InputBase, cn } from "../components/ui/primitives";
import { EntityIcon } from "../components/ui/entity-icon";
import { ContainerCreateModal } from "../components/containers/ContainerCreate";
import { ContainerUpdateModal, type ContainerUpdateRun } from "../components/containers/ContainerUpdateModal";
import { emptyContainerAdvanced } from "../components/containers/ContainerAdvancedFields";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { useToast } from "../app/ToastProvider";
import { useApp } from "../app/AppDataProvider";
import type { AppCatalogEntry, Container, ContainerCreateOptions, ContainerTemplate, PluginEvent } from "../lib/types";

const PAGE_SIZE = 48;

function appIconURL(value?: string): string | undefined {
  return value ? `/api/v1/apps/icon?url=${encodeURIComponent(value)}` : undefined;
}

function formatDownloads(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}k`;
  return String(value);
}

function containerOptions(template: ContainerTemplate): ContainerCreateOptions {
  return {
    name: template.name,
    image: template.image,
    networkMode: template.network || "bridge",
    restartPolicy: template.restartPolicy || "unless-stopped",
    ports: template.ports ?? [],
    volumes: template.volumes ?? [],
    env: template.env ?? [],
    icon: template.icon,
    webUi: template.webUi,
    privileged: template.privileged,
    advanced: template.advanced ?? emptyContainerAdvanced,
  };
}

export function AppsPage() {
  const { t } = useI18n();
  const { toast } = useToast();
  const { snapshot } = useApp();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);
  const [kind, setKind] = useState("all");
  const [category, setCategory] = useState("all");
  const [order, setOrder] = useState("popular");
  const [offset, setOffset] = useState(0);
  const [selected, setSelected] = useState<AppCatalogEntry | null>(null);
  const [installPreset, setInstallPreset] = useState<ContainerCreateOptions | null>(null);
  const [pluginTarget, setPluginTarget] = useState<AppCatalogEntry | null>(null);
  const [pluginRun, setPluginRun] = useState<{ running: boolean; lines: string[]; error: string } | null>(null);
  const [updateRun, setUpdateRun] = useState<ContainerUpdateRun | null>(null);

  const catalog = useQuery({
    queryKey: ["apps-catalog", deferredQuery, kind, category, order, offset],
    queryFn: () => api.appsCatalog({ query: deferredQuery, kind, category, order, limit: PAGE_SIZE, offset }),
    staleTime: 60_000,
  });

  const template = useMutation({
    mutationFn: (entry: AppCatalogEntry) => api.appTemplate(entry.templateUrl ?? ""),
    onSuccess: (value) => {
      setInstallPreset(containerOptions(value));
      setSelected(null);
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not load the application template"), description: error.message }),
  });

  const checkUpdates = useMutation({
    mutationFn: api.checkContainerUpdates,
    onSuccess: (checks) => {
      const available = checks.filter((check) => check.update).length;
      const failed = checks.filter((check) => check.error).length;
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({
        kind: available ? "warning" : "success",
        title: available ? t("{count} updates available", { count: available }) : t("All images are up to date"),
        description:
          (checks.length === 1 ? t("1 image checked") : t("{count} images checked", { count: checks.length })) +
          (failed ? (failed === 1 ? t(" · 1 could not be checked") : t(" · {count} could not be checked", { count: failed })) : ""),
      });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Update check failed"), description: error.message }),
  });

  const pluginInstall = useMutation({
    mutationFn: (entry: AppCatalogEntry) => {
      setPluginRun({ running: true, lines: [], error: "" });
      return api.installPlugin(entry.pluginUrl ?? "", (event: PluginEvent) => {
        setPluginRun((current) => current ? {
          ...current,
          lines: event.line ? [...current.lines.slice(-400), event.line] : current.lines,
          error: event.error ? event.line : current.error,
        } : current);
      });
    },
    onSuccess: () => {
      setPluginRun((current) => current ? { ...current, running: false } : current);
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({ kind: "success", title: t("Plugin installed") });
    },
    onError: (error: Error) => {
      setPluginRun((current) => current ? { ...current, running: false, error: error.message } : current);
      toast({ kind: "error", title: t("Plugin operation failed"), description: error.message });
    },
  });

  const entries = catalog.data?.entries ?? [];
  const categories = useMemo(() => catalog.data?.categories ?? [], [catalog.data?.categories]);
  const total = catalog.data?.total ?? 0;
  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const chooseKind = (value: string) => {
    setKind(value);
    setOffset(0);
  };
  const chooseCategory = (value: string) => {
    setCategory(value);
    setOffset(0);
  };
  const chooseOrder = (value: string) => {
    setOrder(value);
    setOffset(0);
  };

  const install = () => {
    if (!selected) return;
    if (selected.kind === "plugin") {
      setPluginRun(null);
      setPluginTarget(selected);
      setSelected(null);
    } else {
      template.mutate(selected);
    }
  };

  const installedContainers = useMemo(() => {
    const map = new Map<string, Container>();
    for (const container of snapshot?.containers ?? []) {
      const keys = [container.name, container.repository, container.image, container.image.split(":")[0]];
      for (const key of keys) {
        if (key) map.set(key.toLowerCase(), container);
      }
    }
    return map;
  }, [snapshot?.containers]);

  const installedFor = (entry: AppCatalogEntry) => {
    const keys = [entry.name, entry.repository, entry.repo, entry.repository?.split(":")[0]].filter(Boolean) as string[];
    for (const key of keys) {
      const match = installedContainers.get(key.toLowerCase());
      if (match) return match;
    }
    const repository = entry.repository?.toLowerCase();
    if (!repository) return undefined;
    return (snapshot?.containers ?? []).find((container) => container.image.toLowerCase().includes(repository));
  };

  const runUpdate = async (container: Container) => {
    setUpdateRun({
      names: [container.name], index: 0, startedAt: Date.now(),
      events: [], running: true, error: "", done: false,
    });
    let streamError = "";
    try {
      await api.updateContainer(container.id, (event) => {
        if (event.error) streamError = event.message;
        setUpdateRun((current) => (current ? { ...current, events: [...current.events.slice(-40), event] } : current));
      });
      if (streamError) throw new Error(streamError);
      setUpdateRun((current) => (current ? { ...current, running: false, done: true } : current));
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({ kind: "success", title: t("{name} updated", { name: container.name }) });
    } catch (error) {
      setUpdateRun((current) => (current ? { ...current, running: false, error: (error as Error).message } : current));
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    }
  };

  const selectedInstalled = selected ? installedFor(selected) : undefined;

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("Applications")}
        description={t("Browse Community Applications for Docker containers and plugins, then review every setting before installing.")}
        meta={
          <>
            <Badge tone="accent" dot>{t("Community Applications")}</Badge>
            {catalog.data?.updatedAt ? <span className="chip">{t("Feed updated {time}", { time: new Date(catalog.data.updatedAt).toLocaleDateString() })}</span> : null}
          </>
        }
        actions={
          <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} loading={checkUpdates.isPending} onClick={() => checkUpdates.mutate()}>
            {t("Check updates")}
          </Button>
        }
      />

      <Card className="card-pad">
        <div className="flex flex-wrap items-end gap-2">
          <InputBase
            value={query}
            onChange={(value) => { setQuery(value); setOffset(0); }}
            placeholder={t("Search applications…")}
            icon={<Search className="size-3.5" />}
            className="w-full sm:w-80"
          />
          <div className="flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
            {[
              { value: "all", label: t("All") },
              { value: "docker", label: t("Docker") },
              { value: "plugin", label: t("Plugins") },
            ].map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => chooseKind(option.value)}
                className={cn(
                  "rounded-md px-3 py-1.5 text-[10.5px] font-medium transition",
                  kind === option.value ? "bg-[var(--surface)] text-ink shadow-sm" : "text-muted hover:text-ink",
                )}
              >
                {option.label}
              </button>
            ))}
          </div>
          <select
            value={category}
            onChange={(event) => chooseCategory(event.target.value)}
            aria-label={t("Category")}
            className="h-9 max-w-[220px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
          >
            <option value="all">{t("All categories")}</option>
            {categories.map((item) => <option key={item} value={item}>{item}</option>)}
          </select>
          <select
            value={order}
            onChange={(event) => chooseOrder(event.target.value)}
            aria-label={t("Sort order")}
            className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
          >
            <option value="popular">{t("Most downloaded")}</option>
            <option value="name">{t("Name")}</option>
            <option value="newest">{t("Newest")}</option>
          </select>
        </div>

        {catalog.isLoading ? (
          <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 6 }, (_, index) => <div key={index} className="skeleton h-32 rounded-xl" />)}
          </div>
        ) : catalog.isError ? (
          <div className="mt-5 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-[10.5px] text-[var(--danger)]">
            {(catalog.error as Error).message}
          </div>
        ) : entries.length === 0 ? (
          <EmptyState
            icon={<ShoppingBag className="size-5" />}
            title={t("No application matches these filters")}
            description={t("Try another search, category or application type.")}
          />
        ) : (
          <>
            <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {entries.map((entry) => (
                <button
                  key={entry.id}
                  type="button"
                  onClick={() => setSelected(entry)}
                  className="flex min-h-[132px] flex-col rounded-xl border border-line bg-[var(--surface-2)]/35 p-3 text-left transition hover:border-line-strong hover:bg-[var(--surface-2)]/65"
                >
                  <div className="flex items-start gap-3">
                    <EntityIcon src={appIconURL(entry.icon)} name={entry.name} size={38} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-start gap-1.5">
                        <p className="min-w-0 flex-1 truncate text-[12px] font-semibold text-ink">{entry.name}</p>
                        <Badge tone={entry.kind === "plugin" ? "violet" : "info"}>{entry.kind === "plugin" ? t("Plugin") : t("Docker")}</Badge>
                        {(() => {
                          const installed = installedFor(entry);
                          if (!installed) return null;
                          return (
                            <Badge tone={installed.updateAvailable ? "warning" : "success"}>
                              {installed.updateAvailable ? t("Update available") : t("Installed")}
                            </Badge>
                          );
                        })()}
                      </div>
                      <p className="mt-0.5 truncate font-mono text-[9px] text-subtle">{entry.repository || entry.author || entry.repo || "—"}</p>
                    </div>
                  </div>
                  <p className="mt-2 line-clamp-3 text-[10px] leading-relaxed text-muted">{entry.overview || entry.requires || "—"}</p>
                  <div className="mt-auto flex items-center gap-2 pt-2 text-[9px] text-subtle">
                    <span className="flex items-center gap-1"><Download className="size-2.5" />{formatDownloads(entry.downloads)}</span>
                    {entry.categories.slice(0, 2).map((item) => <span key={item} className="truncate rounded border border-line px-1 py-0.5">{item}</span>)}
                  </div>
                </button>
              ))}
            </div>
            <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3 text-[10px] text-subtle">
              <span>{t("{count} applications", { count: total })}</span>
              <div className="flex items-center gap-1.5">
                <Button size="sm" variant="secondary" icon={<ArrowLeft className="size-3.5" />} disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
                  {t("Previous")}
                </Button>
                <span className="px-1 font-mono">{page} / {pages}</span>
                <Button size="sm" variant="secondary" disabled={offset + PAGE_SIZE >= total} onClick={() => setOffset(offset + PAGE_SIZE)}>
                  {t("Next")}<ArrowRight className="size-3.5" />
                </Button>
              </div>
            </div>
          </>
        )}
      </Card>

      <Drawer
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        title={selected?.name ?? ""}
        subtitle={selected ? <span className="flex flex-wrap items-center gap-2"><Badge tone={selected.kind === "plugin" ? "violet" : "info"}>{selected.kind === "plugin" ? t("Plugin") : t("Docker")}</Badge><span className="font-mono text-[9.5px]">{selected.repository || selected.repo}</span></span> : null}
        icon={selected ? <EntityIcon src={appIconURL(selected.icon)} name={selected.name} size={56} /> : undefined}
        width="max-w-[760px]"
        footer={selected ? (
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="flex flex-wrap items-center gap-2 text-[9.5px] text-subtle">
              {t("Aurora never installs an application without an explicit review.")}
              {selectedInstalled ? (
                <Badge tone={selectedInstalled.updateAvailable ? "warning" : "success"}>
                  {selectedInstalled.updateAvailable ? t("Update available") : t("Installed")}
                </Badge>
              ) : null}
            </span>
            <div className="flex gap-2">
              {selected.support ? <Button variant="secondary" icon={<ExternalLink className="size-3.5" />} onClick={() => window.open(selected.support, "_blank", "noopener")}>{t("Support")}</Button> : null}
              {selectedInstalled?.updateAvailable ? (
                <Button
                  variant="primary"
                  icon={<RefreshCw className="size-3.5" />}
                  loading={Boolean(updateRun?.running)}
                  disabled={Boolean(updateRun?.running)}
                  onClick={() => void runUpdate(selectedInstalled)}
                >
                  {t("Update container")}
                </Button>
              ) : null}
              <Button variant={selectedInstalled?.updateAvailable ? "secondary" : "primary"} icon={selected.kind === "plugin" ? <Plug className="size-3.5" /> : <Package className="size-3.5" />} loading={template.isPending} onClick={install}>
                {selected.kind === "plugin" ? t("Install plugin") : t("Review & install")}
              </Button>
            </div>
          </div>
        ) : null}
      >
        {selected ? (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <Info label={t("Author")} value={selected.author || "—"} />
              <Info label={t("Repository")} value={selected.repo || "—"} />
              <Info label={t("Downloads")} value={formatDownloads(selected.downloads)} />
              <Info label={t("Updated")} value={selected.date || "—"} />
            </div>
            {selectedInstalled?.updateAvailable ? (
              <p className="rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] px-3 py-2.5 text-[10.5px] leading-relaxed text-muted">
                {t("The last update check found a newer image digest. Updating pulls it and recreates {name} with its current settings.", { name: selectedInstalled.name })}
              </p>
            ) : null}
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/45 p-3">
              <p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Overview")}</p>
              <p className="mt-2 whitespace-pre-line text-[11px] leading-relaxed text-muted">{selected.overview || t("No description provided.")}</p>
            </div>
            {selected.requires ? <p className="text-[10px] text-subtle"><strong className="text-soft">{t("Requires")}:</strong> {selected.requires}</p> : null}
            <div className="flex flex-wrap gap-2 text-[9.5px] text-subtle">
              {selected.categories.map((item) => <span key={item} className="rounded-md border border-line px-2 py-1">{item}</span>)}
            </div>
            {selected.kind === "plugin" ? (
              <div className="flex items-start gap-2 rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
                <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
                <span>{t("Plugins run as root and can add pages, services and scripts. Verify the author and support thread before installing.")}</span>
              </div>
            ) : null}
          </div>
        ) : null}
      </Drawer>

      <Modal
        open={Boolean(pluginTarget)}
        onClose={() => { if (!pluginInstall.isPending) { setPluginTarget(null); setPluginRun(null); } }}
        title={t("Install {name}?", { name: pluginTarget?.name ?? "" })}
        description={t("Review the plugin warning and install it through the official Unraid plugin manager.")}
        footer={
          <>
            <Button variant="secondary" onClick={() => { setPluginTarget(null); setPluginRun(null); }} disabled={pluginInstall.isPending}> {t("Cancel")}</Button>
            <Button variant="primary" icon={<Plug className="size-3.5" />} loading={pluginInstall.isPending} onClick={() => { if (pluginTarget) pluginInstall.mutate(pluginTarget); }}>
              {t("Install plugin")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div className="flex items-start gap-2 rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
            <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
            <span>{t("Plugins execute root code on this server. Only install applications you trust.")}</span>
          </div>
          <p className="break-all rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2 font-mono text-[9px] text-subtle">{pluginTarget?.pluginUrl}</p>
          {pluginRun ? (
            <div className="max-h-64 overflow-y-auto rounded-lg border border-line bg-[#08090d] p-2 font-mono text-[9px] leading-relaxed text-[#c7cbd6]">
              {pluginRun.lines.length ? pluginRun.lines.map((line, index) => <p key={index}>{line}</p>) : <p>{t("Starting plugin manager…")}</p>}
            </div>
          ) : null}
          {pluginRun?.error ? <p className="text-[10px] text-[var(--danger)]">{pluginRun.error}</p> : null}
        </div>
      </Modal>

      <ContainerUpdateModal
        run={updateRun}
        onClose={() => {
          if (!updateRun?.running) setUpdateRun(null);
        }}
      />

      <ContainerCreateModal open={Boolean(installPreset)} onClose={() => setInstallPreset(null)} preset={installPreset} />
    </>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/45 px-2.5 py-2">
      <p className="text-[8.5px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-1 truncate text-[10.5px] text-soft">{value}</p>
    </div>
  );
}
