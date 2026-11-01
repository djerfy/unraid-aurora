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
import { api } from "../lib/api";
import { formatBytes, formatDateTime, formatDuration, formatSpeed, relativeTime } from "../lib/format";
import type { BalanceEntry, BalanceJob } from "../lib/types";

const PHASES = [
  { key: "scanning", label: "Recherche" },
  { key: "copying", label: "Copie" },
  { key: "cleaning", label: "Nettoyage" },
  { key: "done", label: "Terminé" },
];

export function BalancePage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
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

  useEffect(() => {
    if (!source && diskList.length > 0) setSource(diskList[0].name);
  }, [diskList, source]);

  useEffect(() => {
    setDestinations(diskList.filter((disk) => disk.name !== source).map((disk) => disk.name));
    setSelectedPaths([]);
    setBrowsePath("");
  }, [source, diskList]);

  const start = useMutation({
    mutationFn: api.startBalance,
    onSuccess: (created) => {
      setConfirm(false);
      toast({ kind: "success", title: "Balance job started", description: created.dryRun ? "Dry run: nothing will be written." : "Running in the background." });
      void queryClient.invalidateQueries({ queryKey: ["balance"] });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: "Unable to start balance", description: error.message });
    },
  });

  const cancel = useMutation({
    mutationFn: api.cancelBalance,
    onSuccess: () => toast({ kind: "info", title: "Cancellation requested" }),
    onError: (error: Error) => toast({ kind: "error", title: "Cancel failed", description: error.message }),
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
        eyebrow="Infrastructure"
        title="Équilibrage"
        description="Vider un disque ou déplacer des dossiers vers d'autres disques, en arrière-plan, avec une progression et un détail fichier par fichier."
        meta={
          <>
            <Badge tone={running ? "accent" : "neutral"} dot pulse={running}>
              {running ? job?.state : "prêt"}
            </Badge>
            <span className="chip">{diskList.length} disques montés</span>
            {dryRun ? <Badge tone="warning">dry run</Badge> : null}
          </>
        }
      />

      {running && job ? <LivePanel job={job} onCancel={() => cancel.mutate()} cancelling={cancel.isPending} /> : null}

      <div className={cn("grid grid-cols-1 gap-4", running ? "mt-4" : "")}>
        <Card className="card-pad">
          <div className="flex items-center gap-2">
            <ArrowRightLeft className="size-4 text-[var(--accent)]" />
            <h2 className="text-[13px] font-semibold text-ink">Nouvelle opération</h2>
            <span className="ml-auto text-[10px] text-subtle">source → destination(s)</span>
          </div>

          <div className="mt-4 grid gap-4 lg:grid-cols-2">
            <div>
              <p className="mb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">Disque source</p>
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
                        <Badge tone={disk.role === "data" ? "neutral" : "violet"}>{disk.role}</Badge>
                      </span>
                      <span className="mt-0.5 block text-[9.5px] text-subtle">
                        {formatBytes(disk.used)} utilisés · {formatBytes(disk.free)} libres
                      </span>
                    </span>
                    {source === disk.name ? <Check className="size-4 text-[var(--accent)]" /> : null}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <p className="mb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">Destination(s)</p>
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
                        <span className="mt-0.5 block text-[9.5px] text-subtle">{formatBytes(disk.free)} libres</span>
                      </span>
                      {selected ? <Check className="size-4 text-[var(--success)]" /> : null}
                    </button>
                  );
                })}
              </div>
              <p className="mt-2 text-[9.5px] text-subtle">
                Espace cumulé disponible : <span className="text-soft">{formatBytes(destinationFree)}</span>
              </p>
            </div>
          </div>

          <div className="mt-5 flex flex-wrap items-center gap-2">
            <div className="flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
              {[
                { value: "empty", label: "Vider le disque" },
                { value: "selection", label: "Dossiers / fichiers" },
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
              <Switch checked={dryRun} onChange={setDryRun} label="Dry run" disabled={running} />
              Dry run
            </label>
            <label className="flex items-center gap-2 text-[10.5px] text-muted">
              <Switch checked={deleteSource} onChange={setDeleteSource} label="Delete source" disabled={running} />
              Supprimer la source
            </label>
            <label className="flex items-center gap-2 text-[10.5px] text-muted">
              <Switch checked={overwrite} onChange={setOverwrite} label="Overwrite" disabled={running} />
              Écraser
            </label>
            <Button
              variant="primary"
              icon={<Play className="size-3.5" />}
              className="ml-auto"
              disabled={running || !source || destinations.length === 0 || (mode === "selection" && selectedPaths.length === 0)}
              onClick={() => setConfirm(true)}
            >
              Lancer
            </Button>
          </div>

          {mode === "selection" ? (
            <div className="mt-5 rounded-xl border border-line">
              <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-3 py-2 text-[10px]">
                <button type="button" onClick={() => setBrowsePath("")} className="rounded px-1.5 py-0.5 font-mono text-[10px] text-[var(--accent)] hover:bg-[var(--surface-2)]">
                  {source || "disque"}/
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
                <p className="px-3 py-4 text-[10.5px] text-muted">Lecture du disque…</p>
              ) : browse.isError ? (
                <p className="px-3 py-4 text-[10.5px] text-[var(--danger)]">{(browse.error as Error).message}</p>
              ) : (browse.data ?? []).length === 0 ? (
                <p className="px-3 py-4 text-[10.5px] text-muted">Dossier vide.</p>
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
                          aria-label={selected ? `Désélectionner ${entry.name}` : `Sélectionner ${entry.name}`}
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
                      <button type="button" onClick={() => setSelectedPaths((current) => current.filter((item) => item !== selected))} aria-label={`Retirer ${selected}`}>
                        <X className="size-2.5" />
                      </button>
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          ) : (
            <p className="mt-4 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] px-3 py-2.5 text-[10.5px] leading-relaxed text-muted">
              Tout le contenu du disque source sera déplacé (hors <span className="font-mono text-[10px]">lost+found</span> et dossiers cachés).
            </p>
          )}
        </Card>
      </div>

      <Card className="card-pad mt-4">
        <div className="flex items-center justify-between">
          <h2 className="text-[13px] font-semibold text-ink">Historique</h2>
          <Badge tone="neutral">{(history.data ?? []).length} exécutions</Badge>
        </div>
        {(history.data ?? []).length === 0 ? (
          <EmptyState icon={<ArrowRightLeft className="size-5" />} title="Aucun équilibrage" description="Les opérations terminées apparaîtront ici avec le détail des fichiers." />
        ) : (
          <div className="mt-3 divide-y divide-line/60">
            {(history.data ?? []).map((entry) => (
              <button key={entry.id} type="button" onClick={() => setHistoryJob(entry)} className="flex w-full flex-wrap items-center gap-2 py-2.5 text-left text-[10.5px] hover:bg-[var(--surface-2)]/40">
                <Badge tone={entry.state === "completed" ? "success" : entry.state === "cancelled" ? "warning" : "danger"}>{entry.state}</Badge>
                <span className="min-w-0 flex-1 truncate text-muted">
                  {entry.source} → {entry.destinations.join(", ")}
                  {entry.dryRun ? " · dry run" : ""}
                </span>
                <span className="num text-subtle">{formatBytes(entry.doneBytes)}</span>
                <span className="text-subtle">{entry.files.length} fichiers</span>
                <span className="text-subtle">{relativeTime(entry.finished || entry.started)}</span>
              </button>
            ))}
          </div>
        )}
      </Card>

      <Modal
        open={confirm}
        onClose={() => setConfirm(false)}
        title="Lancer l'équilibrage ?"
        description={dryRun ? "Mode dry run : aucune écriture, aucune suppression." : "Les fichiers sont copiés puis vérifiés avant suppression de la source."}
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(false)} disabled={start.isPending}>
              Annuler
            </Button>
            <Button variant="primary" loading={start.isPending} onClick={submit}>
              {dryRun ? "Lancer le dry run" : "Lancer"}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <Summary label="Source" value={source} />
          <Summary label="Destinations" value={destinations.join(", ") || "—"} />
          <Summary label="Périmètre" value={mode === "empty" ? "Tout le disque" : `${selectedPaths.length} sélection(s)`} />
          <Summary label="Suppression de la source" value={deleteSource ? "oui" : "non"} />
          <Summary label="Écrasement" value={overwrite ? "oui" : "non"} />
          {!dryRun ? (
            <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
              Vérifie les destinations : les fichiers existants sont ignorés sauf si l'écrasement est activé.
            </div>
          ) : null}
        </div>
      </Modal>

      <Modal
        open={Boolean(historyJob)}
        onClose={() => setHistoryJob(null)}
        title={`Équilibrage · ${historyJob?.id ?? ""}`}
        description={historyJob ? `${formatDateTime(historyJob.started)} · ${historyJob.state}` : undefined}
        size="xl"
        footer={
          <Button variant="secondary" onClick={() => setHistoryJob(null)}>
            Fermer
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
  return (
    <Card className="card-pad">
      <div className="flex flex-wrap items-center gap-2">
        <BarChartIcon />
        <h2 className="text-[13px] font-semibold text-ink">En cours</h2>
        <Badge tone={job.dryRun ? "warning" : "accent"} dot pulse>
          {job.dryRun ? "dry run" : job.phase}
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
          Annuler
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
            {phase.label}
          </div>
        ))}
      </div>
      <Progress value={percent} height={7} />
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Stat label="Progression" value={`${percent.toFixed(1)}%`} />
        <Stat label="Octets" value={`${formatBytes(job.doneBytes)} / ${formatBytes(job.totalBytes)}`} />
        <Stat label="Fichiers" value={`${job.doneFiles} / ${job.totalFiles}`} />
        <Stat label="Vitesse" value={formatSpeed(job.speed)} />
        <Stat label="ETA" value={job.eta > 0 ? formatDuration(job.eta) : "—"} />
        <Stat label="État" value={job.state} />
        <Stat label="Mis à jour" value={job.finished ? formatDateTime(job.finished) : relativeTime(job.started)} />
        <Stat label="Mode" value={job.dryRun ? "dry run" : subjectLabel(job.mode)} />
      </div>
      {job.currentPath && !job.finished ? (
        <p className="truncate rounded-lg border border-line bg-[var(--surface-2)]/40 px-2.5 py-1.5 font-mono text-[9.5px] text-soft">
          {job.currentPath}
        </p>
      ) : null}
      {job.errors.length > 0 ? (
        <div className="rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] p-3">
          <p className="text-[10px] font-semibold uppercase tracking-[0.1em] text-[var(--danger)]">Erreurs</p>
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
            <p className="text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">Fichiers</p>
            <span className="text-[9.5px] text-subtle">{job.files.length} entrées</span>
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
            journal ({job.log.length} lignes)
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

function subjectLabel(mode: string) {
  return mode === "empty" ? "disque entier" : "sélection";
}
