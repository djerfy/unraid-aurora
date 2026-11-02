import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowRightLeft,
  Check,
  ChevronRight,
  File,
  Folder,
  FolderOpen,
  Play,
  Square,
  X,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Modal } from "../components/ui/overlays";
import { Badge, Button, Card, EmptyState, Progress, Switch, cn } from "../components/ui/primitives";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatBytes, formatDateTime, formatDuration, formatSpeed, relativeTime } from "../lib/format";
import type { BalanceEntry, BalanceJob } from "../lib/types";

const PHASES = [
  { key: "scanning", label: "Scanning" },
  { key: "copying", label: "Copying" },
  { key: "cleaning", label: "Cleaning" },
  { key: "done", label: "Done" },
];

const JOB_STATES: Record<string, string> = {
  idle: "Idle",
  scanning: "Scanning",
  running: "Running",
  cancelling: "Cancelling",
  completed: "Completed",
  failed: "Failed",
  cancelled: "Cancelled",
};

const DISK_ROLES: Record<string, string> = {
  data: "Data",
  cache: "Cache",
  pool: "Pool",
};

export function BalancePage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const [source, setSource] = useState("");
  const [destinations, setDestinations] = useState<string[]>([]);
  const [mode, setMode] = useState<"empty" | "selection">("empty");
  const [browsePath, setBrowsePath] = useState("");
  const [selectedPaths, setSelectedPaths] = useState<string[]>([]);
  const [dryRun, setDryRun] = useState(true);
  const [deleteSource, setDeleteSource] = useState(true);
  const [overwrite, setOverwrite] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [historyJob, setHistoryJob] = useState<BalanceJob | null>(null);

  const disks = useQuery({ queryKey: ["balance", "disks"], queryFn: api.balanceDisks, refetchInterval: 10_000 });
  const status = useQuery({ queryKey: ["balance", "status"], queryFn: api.balanceStatus, refetchInterval: 1500 });
  const history = useQuery({ queryKey: ["balance", "history"], queryFn: api.balanceHistory, refetchInterval: 5000 });
  const browse = useQuery({
    queryKey: ["balance", "browse", source, browsePath],
    queryFn: () => api.balanceBrowse(source, browsePath),
    enabled: mode === "selection" && Boolean(source),
  });

  const job = status.data;
  const running = Boolean(job && ["scanning", "running", "cancelling"].includes(job.state));
  const diskList = disks.data ?? [];
  const diskKey = diskList.map((disk) => disk.name).join(",");

  useEffect(() => {
    if (!source && diskList.length > 0) setSource(diskList[0].name);
  }, [diskList, source]);

  useEffect(() => {
    setDestinations(diskList.filter((disk) => disk.name !== source && disk.role === "data").map((disk) => disk.name));
    setSelectedPaths([]);
    setBrowsePath("");
  }, [source, diskKey]);

  const start = useMutation({
    mutationFn: api.startBalance,
    onSuccess: (created) => {
      setConfirm(false);
      toast({
        kind: "success",
        title: t("Balance job started"),
        description: created.dryRun ? t("Dry run: nothing will be written.") : t("Running in the background."),
      });
      void queryClient.invalidateQueries({ queryKey: ["balance"] });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Unable to start balance"), description: error.message });
    },
  });

  const cancel = useMutation({
    mutationFn: api.cancelBalance,
    onSuccess: () => toast({ kind: "info", title: t("Cancellation requested") }),
    onError: (error: Error) => toast({ kind: "error", title: t("Cancel failed"), description: error.message }),
  });

  const destinationFree = useMemo(
    () => diskList.filter((disk) => destinations.includes(disk.name)).reduce((sum, disk) => sum + disk.free, 0),
    [diskList, destinations],
  );

  const submit = () => {
    start.mutate({
      source,
      paths: mode === "selection" ? selectedPaths : [],
      destinations,
      deleteSource,
      dryRun,
      overwrite,
    });
  };

  const togglePath = (entry: BalanceEntry) => {
    const entryPath = entry.path;
    setSelectedPaths((current) =>
      current.includes(entryPath)
        ? current.filter((item) => item !== entryPath && !item.startsWith(entryPath + "/"))
        : [...current.filter((item) => !entryPath.startsWith(item + "/")), entryPath],
    );
  };

  const breadcrumbs = browsePath.split("/").filter(Boolean);

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("Balance")}
        description={t(
          "Empty a disk or move folders to other disks in the background, with progress and per-file details.",
        )}
        meta={
          <>
            <Badge tone={running ? "accent" : "neutral"} dot pulse={running}>
              {running ? stateLabel(t, job?.state ?? "") : t("Ready")}
            </Badge>
            <span className="chip">
              {diskList.length === 1 ? t("1 disk mounted") : t("{count} disks mounted", { count: diskList.length })}
            </span>
            {running ? (job?.dryRun ? <Badge tone="warning">{t("Dry run")}</Badge> : null) : dryRun ? <Badge tone="warning">{t("Dry run")}</Badge> : null}
          </>
        }
      />

      {running && job ? <LivePanel job={job} onCancel={() => cancel.mutate()} cancelling={cancel.isPending} /> : null}

      <div className={cn("grid grid-cols-1 gap-4", running ? "mt-4" : "")}>
        <Card className="card-pad">
          <div className="flex items-center gap-2">
            <ArrowRightLeft className="size-4 text-[var(--accent)]" />
            <h2 className="text-[13px] font-semibold text-ink">{t("New operation")}</h2>
            <span className="ml-auto text-[10px] text-subtle">{t("source → destination(s)")}</span>
          </div>

          <div className="mt-4 grid gap-4 lg:grid-cols-2">
            <div>
              <p className="mb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Source disk")}</p>
              <div className="space-y-1.5">
                {diskList.map((disk) => (
                  <button
                    key={disk.name}
                    type="button"
                    disabled={running}
                    onClick={() => setSource(disk.name)}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-xl border px-3 py-2.5 text-left transition",
                      source === disk.name
                        ? "border-[var(--accent-line)] bg-[var(--accent-soft)]"
                        : "border-line bg-[var(--surface-2)]/40 hover:border-line-strong",
                    )}
                  >
                    <span className={cn("grid size-8 place-items-center rounded-lg border", source === disk.name ? "border-[var(--accent-line)] text-[var(--accent)]" : "border-line text-muted")}>
                      <FolderOpen className="size-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-2">
                        <span className="text-[11.5px] font-medium text-ink">{disk.name}</span>
                        <Badge tone={disk.role === "data" ? "neutral" : "violet"}>{diskRoleLabel(t, disk.role)}</Badge>
                      </span>
                      <span className="mt-0.5 block text-[9.5px] text-subtle">
                        {t("{used} used · {free} free", { used: formatBytes(disk.used), free: formatBytes(disk.free) })}
                      </span>
                    </span>
                    {source === disk.name ? <Check className="size-4 text-[var(--accent)]" /> : null}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <p className="mb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Destination(s)")}</p>
              <div className="space-y-1.5">
                {diskList.filter((disk) => disk.name !== source).map((disk) => {
                  const selected = destinations.includes(disk.name);
                  return (
                    <button
                      key={disk.name}
                      type="button"
                      disabled={running}
                      onClick={() =>
                        setDestinations((current) =>
                          current.includes(disk.name) ? current.filter((name) => name !== disk.name) : [...current, disk.name],
                        )
                      }
                      className={cn(
                        "flex w-full items-center gap-3 rounded-xl border px-3 py-2.5 text-left transition",
                        selected ? "border-[var(--color-success)]/40 bg-[var(--success-soft)]" : "border-line bg-[var(--surface-2)]/40 hover:border-line-strong",
                      )}
                    >
                      <span className={cn("grid size-8 place-items-center rounded-lg border", selected ? "border-[color-mix(in_srgb,var(--success)_40%,transparent)] text-[var(--success)]" : "border-line text-muted")}>
                        <Folder className="size-4" />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="text-[11.5px] font-medium text-ink">{disk.name}</span>
                        <span className="mt-0.5 block text-[9.5px] text-subtle">{t("{free} free", { free: formatBytes(disk.free) })}</span>
                      </span>
                      {selected ? <Check className="size-4 text-[var(--success)]" /> : null}
                    </button>
                  );
                })}
              </div>
              <p className="mt-2 text-[9.5px] text-subtle">
                {t("Total space available:")} <span className="text-soft">{formatBytes(destinationFree)}</span>
              </p>
            </div>
          </div>

          <div className="mt-5 flex flex-wrap items-center gap-2">
            <div className="flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
              {[
                { value: "empty", label: t("Empty disk") },
                { value: "selection", label: t("Folders / files") },
              ].map((item) => (
                <button
                  key={item.value}
                  type="button"
                  disabled={running}
                  onClick={() => setMode(item.value as typeof mode)}
                  className={cn(
                    "rounded-md px-2.5 py-1.5 text-[10.5px] font-medium transition",
                    mode === item.value ? "bg-[var(--surface)] text-ink shadow-sm" : "text-muted hover:text-ink",
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
            <label className="flex items-center gap-2 text-[10.5px] text-muted">
              <Switch checked={dryRun} onChange={setDryRun} label={t("Dry run")} disabled={running} />
              {t("Dry run")}
            </label>
            <label className="flex items-center gap-2 text-[10.5px] text-muted">
              <Switch checked={deleteSource} onChange={setDeleteSource} label={t("Delete source")} disabled={running} />
              {t("Delete source")}
            </label>
            <label className="flex items-center gap-2 text-[10.5px] text-muted">
              <Switch checked={overwrite} onChange={setOverwrite} label={t("Overwrite")} disabled={running} />
              {t("Overwrite")}
            </label>
            <Button
              variant="primary"
              icon={<Play className="size-3.5" />}
              className="ml-auto"
              disabled={running || !source || destinations.length === 0 || (mode === "selection" && selectedPaths.length === 0)}
              onClick={() => setConfirm(true)}
            >
              {t("Start")}
            </Button>
          </div>

          {mode === "selection" ? (
            <div className="mt-5 rounded-xl border border-line">
              <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-3 py-2 text-[10px]">
                <button type="button" onClick={() => setBrowsePath("")} className="rounded px-1.5 py-0.5 font-mono text-[10px] text-[var(--accent)] hover:bg-[var(--surface-2)]">
                  {source || t("disk")}/
                </button>
                {breadcrumbs.map((crumb, index) => {
                  const target = breadcrumbs.slice(0, index + 1).join("/");
                  return (
                    <span key={target} className="flex items-center gap-1">
                      <button type="button" onClick={() => setBrowsePath(target)} className="rounded px-1.5 py-0.5 font-mono text-[10px] text-muted hover:bg-[var(--surface-2)] hover:text-ink">
                        {crumb}
                      </button>
                      <ChevronRight className="size-3 text-subtle" />
                    </span>
                  );
                })}
              </div>
              {browse.isLoading ? (
                <p className="px-3 py-4 text-[10.5px] text-muted">{t("Reading disk…")}</p>
              ) : browse.isError ? (
                <p className="px-3 py-4 text-[10.5px] text-[var(--danger)]">{(browse.error as Error).message}</p>
              ) : (browse.data ?? []).length === 0 ? (
                <p className="px-3 py-4 text-[10.5px] text-muted">{t("Empty folder.")}</p>
              ) : (
                <div className="max-h-72 divide-y divide-line/50 overflow-y-auto">
                  {(browse.data ?? []).map((entry) => {
                    const selected = selectedPaths.includes(entry.path) || selectedPaths.some((item) => item.startsWith(entry.path + "/"));
                    return (
                      <div key={entry.path} className="flex items-center gap-2 px-3 py-2 text-[10.5px]">
                        <button
                          type="button"
                          onClick={() => togglePath(entry)}
                          className={cn(
                            "grid size-4 shrink-0 place-items-center rounded border",
                            selected ? "border-[var(--accent)] bg-[var(--accent)] text-[var(--accent-contrast)]" : "border-line",
                          )}
                          aria-label={
                            selected
                              ? t("Deselect {name}", { name: entry.name })
                              : t("Select {name}", { name: entry.name })
                          }
                        >
                          {selected ? <Check className="size-3" /> : null}
                        </button>
                        {entry.dir ? (
                          <button type="button" onClick={() => setBrowsePath(entry.path)} className="flex min-w-0 flex-1 items-center gap-2 text-left">
                            <Folder className="size-3.5 shrink-0 text-[var(--violet)]" />
                            <span className="truncate text-soft hover:text-ink">{entry.name}</span>
                          </button>
                        ) : (
                          <span className="flex min-w-0 flex-1 items-center gap-2">
                            <File className="size-3.5 shrink-0 text-muted" />
                            <span className="truncate text-soft">{entry.name}</span>
                          </span>
                        )}
                        <span className="num shrink-0 text-[9.5px] text-subtle">{entry.dir && entry.size === 0 ? "—" : formatBytes(entry.size)}</span>
                      </div>
                    );
                  })}
                </div>
              )}
              {selectedPaths.length > 0 ? (
                <div className="flex flex-wrap gap-1.5 border-t border-line px-3 py-2">
                  {selectedPaths.map((selected) => (
                    <span key={selected} className="flex items-center gap-1 rounded-md border border-line bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-[9px] text-soft">
                      {selected}
                      <button type="button" onClick={() => setSelectedPaths((current) => current.filter((item) => item !== selected))} aria-label={t("Remove {path}", { path: selected })}>
                        <X className="size-2.5" />
                      </button>
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          ) : (
            <p className="mt-4 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] px-3 py-2.5 text-[10.5px] leading-relaxed text-muted">
              {t("All content on the source disk will be moved, except")}{" "}
              <span className="font-mono text-[10px]">lost+found</span> {t("and hidden folders.")}
            </p>
          )}
        </Card>
      </div>

      <Card className="card-pad mt-4">
        <div className="flex items-center justify-between">
          <h2 className="text-[13px] font-semibold text-ink">{t("History")}</h2>
          <Badge tone="neutral">
            {(history.data ?? []).length === 1
              ? t("1 run")
              : t("{count} runs", { count: (history.data ?? []).length })}
          </Badge>
        </div>
        {(history.data ?? []).length === 0 ? (
          <EmptyState
            icon={<ArrowRightLeft className="size-5" />}
            title={t("No balance run yet")}
            description={t("Completed operations will appear here with file details.")}
          />
        ) : (
          <div className="mt-3 divide-y divide-line/60">
            {(history.data ?? []).map((entry) => (
              <button key={entry.id} type="button" onClick={() => setHistoryJob(entry)} className="flex w-full flex-wrap items-center gap-2 py-2.5 text-left text-[10.5px] hover:bg-[var(--surface-2)]/40">
                <Badge tone={entry.state === "completed" ? "success" : entry.state === "cancelled" ? "warning" : "danger"}>{stateLabel(t, entry.state)}</Badge>
                <span className="min-w-0 flex-1 truncate text-muted">
                  {entry.source} → {entry.destinations.join(", ")}
                  {entry.dryRun ? ` · ${t("Dry run")}` : ""}
                </span>
                <span className="num text-subtle">{formatBytes(entry.doneBytes)}</span>
                <span className="text-subtle">
                  {entry.files.length === 1 ? t("1 file") : t("{count} files", { count: entry.files.length })}
                </span>
                <span className="text-subtle">{relativeTime(entry.finished || entry.started)}</span>
              </button>
            ))}
          </div>
        )}
      </Card>

      <Modal
        open={confirm}
        onClose={() => setConfirm(false)}
        title={t("Start balance?")}
        description={
          dryRun
            ? t("Dry run mode: no writes, no deletions.")
            : t("Files are copied and verified before the source is deleted.")
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(false)} disabled={start.isPending}>
              {t("Cancel")}
            </Button>
            <Button variant="primary" loading={start.isPending} onClick={submit}>
              {dryRun ? t("Start dry run") : t("Start")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <Summary label={t("Source")} value={source} />
          <Summary label={t("Destinations")} value={destinations.join(", ") || "—"} />
          <Summary
            label={t("Scope")}
            value={
              mode === "empty"
                ? t("Whole disk")
                : selectedPaths.length === 1
                  ? t("1 selection")
                  : t("{count} selections", { count: selectedPaths.length })
            }
          />
          <Summary label={t("Delete source")} value={deleteSource ? t("Yes") : t("No")} />
          <Summary label={t("Overwrite")} value={overwrite ? t("Yes") : t("No")} />
          {!dryRun ? (
            <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
              {t("Check the destinations: existing files are skipped unless overwrite is enabled.")}
            </div>
          ) : null}
        </div>
      </Modal>

      <Modal
        open={Boolean(historyJob)}
        onClose={() => setHistoryJob(null)}
        title={t("Balance · {id}", { id: historyJob?.id ?? "" })}
        description={historyJob ? `${formatDateTime(historyJob.started)} · ${stateLabel(t, historyJob.state)}` : undefined}
        size="xl"
        footer={
          <Button variant="secondary" onClick={() => setHistoryJob(null)}>
            {t("Close")}
          </Button>
        }
      >
        {historyJob ? <JobDetails job={historyJob} /> : null}
      </Modal>
    </>
  );
}

function Summary({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2">
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-0.5 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}

function LivePanel({ job, onCancel, cancelling }: { job: BalanceJob; onCancel: () => void; cancelling: boolean }) {
  const { t } = useI18n();
  return (
    <Card className="card-pad">
      <div className="flex flex-wrap items-center gap-2">
        <BarChartIcon />
        <h2 className="text-[13px] font-semibold text-ink">{t("Running")}</h2>
        <Badge tone={job.dryRun ? "warning" : "accent"} dot pulse>
          {job.dryRun ? t("Dry run") : phaseLabel(t, job.phase)}
        </Badge>
        <span className="text-[10px] text-subtle">
          {job.source} → {job.destinations.join(", ")}
        </span>
        <Button
          size="sm"
          variant="danger"
          className="ml-auto"
          icon={<Square className="size-3" />}
          loading={cancelling}
          onClick={onCancel}
        >
          {t("Cancel")}
        </Button>
      </div>
      <div className="mt-4">
        <JobDetails job={job} compact />
      </div>
    </Card>
  );
}

function BarChartIcon() {
  return <ArrowRightLeft className="size-4 text-[var(--accent)]" />;
}

function JobDetails({ job, compact }: { job: BalanceJob; compact?: boolean }) {
  const { t } = useI18n();
  const percent = job.totalBytes > 0 ? Math.min(100, (job.doneBytes / job.totalBytes) * 100) : 0;
  const phaseIndex = PHASES.findIndex((phase) => phase.key === job.phase);
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-4 gap-1.5">
        {PHASES.map((phase, index) => (
          <div
            key={phase.key}
            className={cn(
              "rounded-lg border px-2 py-1.5 text-center text-[9px] font-medium uppercase tracking-[0.08em]",
              index === phaseIndex
                ? "border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)]"
                : index < phaseIndex
                  ? "border-[color-mix(in_srgb,var(--success)_30%,transparent)] bg-[var(--success-soft)] text-[var(--success)]"
                  : "border-line bg-[var(--surface-2)]/40 text-subtle",
            )}
          >
            {t(phase.label)}
          </div>
        ))}
      </div>
      <Progress value={percent} height={7} />
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Stat label={t("Progress")} value={`${percent.toFixed(1)}%`} />
        <Stat label={t("Bytes")} value={`${formatBytes(job.doneBytes)} / ${formatBytes(job.totalBytes)}`} />
        <Stat label={t("Files")} value={`${job.doneFiles} / ${job.totalFiles}`} />
        <Stat label={t("Speed")} value={formatSpeed(job.speed)} />
        <Stat label={t("ETA")} value={job.eta > 0 ? formatDuration(job.eta) : "—"} />
        <Stat label={t("State")} value={stateLabel(t, job.state)} />
        <Stat label={t("Updated")} value={job.finished ? formatDateTime(job.finished) : relativeTime(job.started)} />
        <Stat label={t("Mode")} value={job.dryRun ? t("Dry run") : subjectLabel(t, job.mode)} />
      </div>
      {job.currentPath && !job.finished ? (
        <p className="truncate rounded-lg border border-line bg-[var(--surface-2)]/40 px-2.5 py-1.5 font-mono text-[9.5px] text-soft">
          {job.currentPath}
        </p>
      ) : null}
      {job.errors.length > 0 ? (
        <div className="rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] p-3">
          <p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-[var(--danger)]">{t("Errors")}</p>
          <ul className="mt-1.5 max-h-32 space-y-1 overflow-y-auto text-[10px] text-muted">
            {job.errors.slice(-20).map((error, index) => (
              <li key={`${error}-${index}`}>• {error}</li>
            ))}
          </ul>
        </div>
      ) : null}
      {job.files.length > 0 ? (
        <div className="overflow-hidden rounded-xl border border-line">
          <div className="flex items-center justify-between border-b border-line bg-[var(--surface-2)]/50 px-3 py-2">
            <p className="text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Files")}</p>
            <span className="text-[9.5px] text-subtle">{t("{count} entries", { count: job.files.length })}</span>
          </div>
          <div className={cn("divide-y divide-line/50 overflow-y-auto", compact ? "max-h-48" : "max-h-80")}>
            {job.files.slice(-500).reverse().map((file, index) => (
              <div key={`${file.path}-${index}`} className="flex items-center gap-2 px-3 py-1.5 text-[10px]">
                <span
                  className={cn(
                    "size-1.5 shrink-0 rounded-full",
                    file.status === "moved" ? "bg-[var(--success)]" : file.status === "planned" ? "bg-[var(--info)]" : file.status === "skipped" ? "bg-[var(--warning)]" : "bg-[var(--danger)]",
                  )}
                />
                <span className="min-w-0 flex-1 truncate font-mono text-[9.5px] text-soft">{file.path}</span>
                <span className="shrink-0 text-subtle">→ {file.destination}</span>
                <span className="num shrink-0 text-subtle">{formatBytes(file.size)}</span>
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {job.log.length > 0 ? (
        <details className="rounded-xl border border-line bg-[#08090d]">
          <summary className="cursor-pointer px-3 py-2 font-mono text-[9px] uppercase tracking-[0.08em] text-[#8b93a7]">
            {t("log ({count} lines)", { count: job.log.length })}
          </summary>
          <pre className="max-h-40 overflow-auto px-3 pb-2 font-mono text-[9px] leading-relaxed text-[#8b93a7]">{job.log.join("\n")}</pre>
        </details>
      ) : null}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/40 px-2.5 py-1.5">
      <p className="text-[8.5px] uppercase tracking-[0.08em] text-subtle">{label}</p>
      <p className="num mt-0.5 truncate text-[10.5px] text-soft">{value}</p>
    </div>
  );
}

function subjectLabel(t: (key: string) => string, mode: string) {
  return mode === "empty" ? t("Whole disk") : t("Selection");
}

function stateLabel(t: (key: string) => string, state: string) {
  const label = JOB_STATES[state];
  return label ? t(label) : state;
}

function diskRoleLabel(t: (key: string) => string, role: string) {
  const label = DISK_ROLES[role];
  return label ? t(label) : role;
}

function phaseLabel(t: (key: string) => string, key: string) {
  const phase = PHASES.find((item) => item.key === key);
  return phase ? t(phase.label) : key;
}
