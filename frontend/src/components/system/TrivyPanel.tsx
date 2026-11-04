import { useMutation, useQuery } from "@tanstack/react-query";
import { CheckCircle2, Database, DownloadCloud, FileCode2, RefreshCw, ScanSearch, ShieldAlert, Terminal } from "lucide-react";
import { Badge, Button, Card, Switch } from "../ui/primitives";
import { useToast } from "../../app/ToastProvider";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import { formatBytes } from "../../lib/format";
import type { TrivyStatus, TrivyVersion } from "../../lib/types";

export function TrivyPanel({
  enabled,
  version,
  hour,
  onEnabledChange,
  onVersionChange,
  onHourChange,
}: {
  enabled: boolean;
  version: string;
  hour: number;
  onEnabledChange: (value: boolean) => void;
  onVersionChange: (value: string) => void;
  onHourChange: (value: number) => void;
}) {
  const { t } = useI18n();
  const { toast } = useToast();

  const status = useQuery<TrivyStatus>({
    queryKey: ["trivy-status"],
    queryFn: api.trivyStatus,
    refetchInterval: (query) => {
      const data = query.state.data;
      return data?.installing || data?.updatingDb ? 2000 : 30_000;
    },
  });
  const versions = useQuery<TrivyVersion[]>({
    queryKey: ["trivy-versions"],
    queryFn: api.trivyVersions,
    retry: false,
    staleTime: 30 * 60_000,
  });

  const install = useMutation({
    mutationFn: (requested: string) => api.installTrivy(requested),
    onSuccess: (_result, requested) => {
      onVersionChange(requested.trim());
      toast({
        kind: "info",
        title: requested.trim() ? t("Installing Trivy {version}", { version: requested.trim() }) : t("Installing the latest Trivy release"),
        description: t("The binary and both vulnerability databases are downloaded in the background."),
      });
      void status.refetch();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not start the Trivy installation"), description: error.message }),
  });

  const updateDB = useMutation({
    mutationFn: api.updateTrivyDB,
    onSuccess: () => {
      toast({ kind: "info", title: t("Database update started") });
      void status.refetch();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not update the vulnerability databases"), description: error.message }),
  });

  const stageLabel = (stage?: string) => {
    if (!stage) return "";
    if (stage.startsWith("Downloading Trivy")) return t("Downloading Trivy…");
    if (stage === "Downloading vulnerability database") return t("Downloading vulnerability database");
    if (stage === "Downloading Java vulnerability database") return t("Downloading Java vulnerability database");
    if (stage === "Starting installation") return t("Starting installation");
    if (stage === "Starting database update") return t("Starting database update");
    return stage;
  };
  const data = status.data;
  const busy = Boolean(data?.installing || data?.updatingDb) || install.isPending || updateDB.isPending;
  const installed = Boolean(data?.installed);
  const runtimeReady = Boolean(data?.runtimeReady);
  const ready = installed && runtimeReady;
  const latest = versions.data?.[0]?.version ?? "";
  const desired = version.trim();
  const mismatch = installed && desired !== "" && data?.version !== desired.replace(/^v/, "");

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <ScanSearch className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("Image scanning with Trivy")}</h2>
              <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                {t("Scan container images for known vulnerabilities. Trivy and its databases are stored persistently in the Aurora configuration directory.")}
              </p>
            </div>
          </div>
          <Badge tone={data?.installing || data?.updatingDb ? "info" : ready ? "success" : installed ? "danger" : "neutral"} dot pulse={data?.installing || data?.updatingDb}>
            {data?.installing ? t("Installing") : data?.updatingDb ? t("Updating databases") : ready ? t("Installed") : installed ? t("Not executable") : t("Not installed")}
          </Badge>
        </div>

        <div className="mt-4 flex items-start justify-between gap-4 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-3">
          <div className="min-w-0">
            <p className="text-[11.5px] font-medium text-soft">{t("Enable image scanning")}</p>
            <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">
              {t("Save the settings to download Trivy if needed and scan images after every pull or container creation.")}
            </p>
          </div>
          <Switch checked={enabled} onChange={onEnabledChange} label={t("Enable image scanning")} />
        </div>

        {status.isLoading ? <p className="mt-4 text-[10.5px] text-muted">{t("Reading Trivy status…")}</p> : null}
        {status.isError ? <p className="mt-4 text-[10.5px] text-[var(--danger)]">{(status.error as Error).message}</p> : null}

        {data ? (
          <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5">
              <p className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <Terminal className="size-3" /> {t("Binary")}
              </p>
              <p className="mt-1 font-mono text-[11px] text-soft">{data.version ? `v${data.version}` : data.installed ? t("Unknown") : t("Not installed")}</p>
            </div>
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5">
              <p className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <Database className="size-3" /> {t("Vulnerability database")}
              </p>
              <p className="mt-1 text-[11px] text-soft">
                {data.dbReady ? (
                  <>
                    {data.dbUpdatedAt ? new Date(data.dbUpdatedAt).toLocaleString() : t("Ready")}
                    {data.dbSize ? <span className="ml-1 text-subtle">· {formatBytes(data.dbSize)}</span> : null}
                  </>
                ) : (
                  t("Not downloaded")
                )}
              </p>
            </div>
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5">
              <p className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <FileCode2 className="size-3" /> {t("Java database")}
              </p>
              <p className="mt-1 text-[11px] text-soft">
                {data.javaDbReady ? (
                  <>
                    {t("Ready")}
                    {data.javaDbSize ? <span className="ml-1 text-subtle">· {formatBytes(data.javaDbSize)}</span> : null}
                  </>
                ) : (
                  t("Not downloaded")
                )}
              </p>
            </div>
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5">
              <p className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <RefreshCw className="size-3" /> {t("Nightly update")}
              </p>
              <p className="mt-1 text-[11px] text-soft">{t("Every day at {time}", { time: `${String(hour).padStart(2, "0")}:00` })}</p>
            </div>
          </div>
        ) : null}

        {data?.stage ? (
          <p className="mt-3 flex items-center gap-1.5 text-[10px] text-[var(--info)]">
            <RefreshCw className="size-3 animate-spin" /> {stageLabel(data.stage)}
          </p>
        ) : null}
        {data?.error ? (
          <p className="mt-3 flex items-start gap-1.5 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-2.5 text-[10px] leading-relaxed text-[var(--danger)]">
            <ShieldAlert className="mt-0.5 size-3.5 shrink-0" /> {data.error}
          </p>
        ) : null}
        {installed && data?.dbReady && !data.javaDbReady ? (
          <p className="mt-3 rounded-xl border border-[var(--warning-soft)] bg-[var(--warning-soft)] p-2.5 text-[9.5px] leading-relaxed text-[var(--warning)]">
            {t("The Java database is missing. Scans start only once both databases are ready; Aurora downloads it automatically.")}
          </p>
        ) : null}
        {installed && !runtimeReady ? (
          <p className="mt-3 rounded-xl border border-[var(--warning-soft)] bg-[var(--warning-soft)] p-2.5 text-[9.5px] leading-relaxed text-[var(--warning)]">
            {t("Aurora copies Trivy into an executable runtime directory. If this fails, set AURORA_TRIVY_RUNTIME_DIR to a writable, executable directory.")}
          </p>
        ) : null}
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Binary and databases")}</h2>
        <p className="mt-0.5 text-[10.5px] text-subtle">
          {t("Choose a specific Trivy release or leave the field empty to use the latest stable release.")}
        </p>

        <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
          <label className="block min-w-0">
            <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Trivy version")}</span>
            <input
              list="trivy-version-options"
              value={version}
              onChange={(event) => onVersionChange(event.target.value)}
              placeholder={latest || t("Latest stable")}
              className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
            />
            <datalist id="trivy-version-options">
              {(versions.data ?? []).slice(0, 20).map((item) => (
                <option key={item.version} value={item.version}>{item.name || item.version}</option>
              ))}
            </datalist>
          </label>
          <div className="flex flex-wrap items-end gap-2">
            <Button
              variant="primary"
              icon={<DownloadCloud className="size-3.5" />}
              loading={install.isPending}
              disabled={busy}
              onClick={() => install.mutate(desired)}
            >
              {installed ? t("Install selected version") : t("Install Trivy")}
            </Button>
            <Button
              variant="secondary"
              icon={<RefreshCw className="size-3.5" />}
              disabled={busy}
              onClick={() => install.mutate("")}
            >
              {t("Update to latest")}
            </Button>
          </div>
        </div>

        {versions.isError ? (
          <p className="mt-2 text-[9.5px] text-subtle">{t("The release list is unavailable. You can still type a version manually.")}</p>
        ) : latest ? (
          <p className="mt-2 text-[9.5px] text-subtle">{t("Latest stable release: {version}", { version: latest })}</p>
        ) : null}
        <p className="mt-2 text-[9px] leading-relaxed text-subtle">
          {t("Set AURORA_TRIVY_DIR to keep the binary and databases on another persistent volume, such as an appdata share.")}
        </p>
        {installed && latest && data?.version && data.version !== latest ? (
          <p className="mt-2 text-[9.5px] text-[var(--warning)]">
            {t("A newer Trivy release is available: v{version}", { version: latest })}
          </p>
        ) : null}
        {mismatch ? (
          <p className="mt-2 flex items-center gap-1.5 text-[9.5px] text-[var(--warning)]">
            <ShieldAlert className="size-3" /> {t("Installed v{installed}, selected v{selected}. Install it to apply the change.", { installed: data?.version ?? "", selected: desired })}
          </p>
        ) : null}

        <div className="mt-5 grid grid-cols-1 gap-3 border-t border-line pt-4 sm:grid-cols-2">
          <div className="flex flex-wrap items-end gap-2">
            <label className="min-w-0 flex-1">
              <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Database update hour")}</span>
              <select
                value={hour}
                onChange={(event) => onHourChange(Number(event.target.value))}
                className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
              >
                {Array.from({ length: 24 }, (_, value) => (
                  <option key={value} value={value}>{String(value).padStart(2, "0")}:00</option>
                ))}
              </select>
            </label>
            <Button
              variant="secondary"
              icon={<Database className="size-3.5" />}
              loading={updateDB.isPending}
              disabled={busy || !ready}
              onClick={() => updateDB.mutate()}
            >
              {t("Update databases now")}
            </Button>
          </div>
          <div className="flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/40 p-3 text-[9.5px] leading-relaxed text-subtle">
            <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
            {t("The vulnerability and Java databases are downloaded after installation and refreshed every night at the selected hour while scanning is enabled. Save the settings to apply the schedule.")}
          </div>
        </div>
      </Card>
    </div>
  );
}
