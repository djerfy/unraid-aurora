import { useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, CheckCircle2, Cpu, Download, ExternalLink, RefreshCw } from "lucide-react";
import { Badge, Button, Card, cn } from "../ui/primitives";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { api } from "../../lib/api";
import type { NvidiaStatus, NvidiaVersions } from "../../lib/types";

const COMMUNITY_APPS = "https://ca.unraid.net/apps/nvidia-driver-1aax6kn0zkg6t7";

const BRANCHES = [
  { id: "latest", versionKey: "latest" },
  { id: "latest_prb", versionKey: "production" },
  { id: "latest_nfb", versionKey: "newFeature" },
  { id: "latest_nos", versionKey: "openSource" },
] as const;

type BranchID = (typeof BRANCHES)[number]["id"];

function isBranch(value: string): value is BranchID {
  return BRANCHES.some((branch) => branch.id === value);
}

function branchVersion(versions: NvidiaVersions | undefined, id: BranchID): string {
  if (!versions) return "";
  const branch = BRANCHES.find((item) => item.id === id);
  return branch ? String(versions[branch.versionKey] ?? "") : "";
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 py-2.5 first:pt-0 last:pb-0">
      <span className="text-[10.5px] text-subtle">{label}</span>
      <span className="text-[11px] font-medium text-soft">{children}</span>
    </div>
  );
}

function licenseLabel(t: (key: string) => string, license: string): string {
  switch (license) {
    case "proprietary":
      return t("Proprietary");
    case "opensource":
      return t("Open source");
    default:
      return t("Not detected");
  }
}

export function NvidiaDriverPanel() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [selection, setSelection] = useState("");

  const status = useQuery<NvidiaStatus>({
    queryKey: ["nvidia-driver"],
    queryFn: api.nvidiaStatus,
    refetchInterval: (query) => (query.state.data?.download?.active ? 2000 : 20_000),
  });
  const versions = useQuery<NvidiaVersions>({
    queryKey: ["nvidia-versions"],
    queryFn: () => api.nvidiaVersions(false),
    enabled: status.data?.pluginInstalled ?? false,
    staleTime: 5 * 60_000,
    retry: false,
  });

  useEffect(() => {
    if (!status.data) return;
    setSelection((current) => current || status.data!.selectedVersion || "latest");
  }, [status.data]);

  const refreshVersions = useMutation({
    mutationFn: () => api.nvidiaVersions(true),
    onSuccess: (data) => queryClient.setQueryData(["nvidia-versions"], data),
    onError: (error: Error) => toast({ kind: "error", title: t("Could not load the driver list"), description: error.message }),
  });

  const download = useMutation({
    mutationFn: (version: string) => api.downloadNvidiaDriver(version),
    onSuccess: (data) => {
      queryClient.setQueryData(["nvidia-driver"], data);
      toast({ kind: "success", title: t("Driver download started"), description: t("The running driver is unchanged until the server reboots.") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not download the driver"), description: error.message }),
  });

  const data = status.data;
  const versionList = versions.data;
  const resolved = isBranch(selection) ? branchVersion(versionList, selection) : selection;
  const busy = Boolean(data?.download?.active) || download.isPending;
  const progress = data?.download?.progress ?? 0;
  const staged = data?.downloadedVersion ?? "";
  const rebootNeeded = Boolean(staged && data?.installedVersion && staged !== data.installedVersion);

  if (status.isLoading && !data) {
    return <Card className="card-pad"><p className="py-8 text-center text-[10.5px] text-muted">{t("Loading NVIDIA driver status…")}</p></Card>;
  }
  if (status.isError) {
    return <Card className="card-pad"><p className="text-[10.5px] text-[var(--danger)]">{(status.error as Error).message}</p></Card>;
  }

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <Cpu className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("NVIDIA driver")}</h2>
              <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                {t("Manage the official Nvidia Driver plugin and stage a package for the next boot.")}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={data?.pluginInstalled ? "success" : "danger"} dot>
              {data?.pluginInstalled ? t("Plugin installed") : t("Plugin missing")}
            </Badge>
            <Badge tone={data?.driverInstalled ? "success" : "neutral"} dot>
              {data?.driverInstalled ? t("Driver installed") : t("No driver")}
            </Badge>
            {data?.license && data.license !== "none" ? <Badge tone="neutral">{licenseLabel(t, data.license)}</Badge> : null}
            <Button
              size="sm"
              variant="secondary"
              icon={<RefreshCw className={cn("size-3.5", (status.isFetching || refreshVersions.isPending || versions.isFetching) && "animate-spin")} />}
              onClick={() => {
                void status.refetch();
                if (data?.pluginInstalled) refreshVersions.mutate();
              }}
            >
              {t("Refresh")}
            </Button>
          </div>
        </div>
        <div className="mt-4 divide-y divide-line border-t border-line pt-1">
          <Row label={t("Installed driver")}>{data?.installedVersion || t("Not detected")}</Row>
          <Row label={t("Kernel")}><span className="font-mono text-[10.5px]">{data?.kernel || "—"}</span></Row>
          <Row label={t("Selected driver")}><span className="font-mono text-[10.5px]">{data?.selectedVersion || "—"}</span></Row>
          <Row label={t("Automatic update check")}>{data?.updateCheck ? t("Enabled") : t("Disabled")}</Row>
        </div>
      </Card>

      {!data?.pluginInstalled ? (
        <Card className="card-pad">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] text-[var(--warning)]">
              <AlertTriangle className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("Nvidia Driver plugin required")}</h2>
              <p className="mt-1 text-[10.5px] leading-relaxed text-subtle">
                {t("Install the official Nvidia Driver plugin from Community Applications. Aurora writes to its settings and package directory.")}
              </p>
              <Button
                className="mt-3"
                size="sm"
                variant="secondary"
                icon={<ExternalLink className="size-3.5" />}
                onClick={() => window.open(COMMUNITY_APPS, "_blank", "noopener,noreferrer")}
              >
                {t("Open Community Applications")}
              </Button>
            </div>
          </div>
        </Card>
      ) : (
        <>
          <Card className="card-pad">
            <h2 className="text-[13px] font-semibold text-ink">{t("Detected GPUs")}</h2>
            {data.gpus.length === 0 ? (
              <p className="mt-2 text-[10.5px] text-muted">{t("No NVIDIA GPU was detected by nvidia-smi.")}</p>
            ) : (
              <div className="mt-2 space-y-2">
                {data.gpus.map((gpu) => (
                  <div key={gpu.uuid || gpu.index} className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-line px-3 py-2">
                    <span className="text-[11px] font-medium text-ink">
                      <span className="mr-2 font-mono text-[10px] text-muted">#{gpu.index}</span>
                      {gpu.name}
                    </span>
                    <span className="font-mono text-[10px] text-subtle">{gpu.uuid}</span>
                  </div>
                ))}
              </div>
            )}
          </Card>

          <Card className="card-pad">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div>
                <h2 className="text-[13px] font-semibold text-ink">{t("Driver package")}</h2>
                <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                  {t("The official plugin installs the staged package at boot. Downloading never changes the running module.")}
                </p>
              </div>
              <Badge tone="neutral">{t("Kernel {kernel}", { kernel: data.kernel || "—" })}</Badge>
            </div>

            {versions.isError ? (
              <div className="mt-3 rounded-[7px] border border-[var(--danger)]/30 bg-[var(--danger-soft)] px-3 py-2">
                <p className="text-[10.5px] text-[var(--danger)]">{(versions.error as Error).message}</p>
                <Button className="mt-2" size="sm" variant="secondary" onClick={() => refreshVersions.mutate()} loading={refreshVersions.isPending}>
                  {t("Retry")}
                </Button>
              </div>
            ) : (
              <>
                {versionList?.branchError ? (
                  <p className="mt-3 rounded-[7px] border border-[var(--warning)]/30 bg-[var(--warning-soft)] px-3 py-2 text-[10px] leading-relaxed text-[var(--warning)]">
                    {t("Branch metadata could not be loaded: {error}", { error: versionList.branchError })}
                  </p>
                ) : null}
                {versions.isLoading ? <p className="mt-3 text-[10px] text-muted">{t("Loading driver versions…")}</p> : null}
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  {BRANCHES.map((branch) => {
                    const version = branchVersion(versionList, branch.id);
                    const branchUnavailable = Boolean(versionList?.branchError) && (branch.id === "latest_prb" || branch.id === "latest_nfb");
                    const available = Boolean(version);
                    return (
                      <label
                        key={branch.id}
                        className={cn(
                          "flex cursor-pointer items-start gap-2.5 rounded-lg border px-3 py-2.5 transition",
                          selection === branch.id ? "border-[var(--accent-line)] bg-[color-mix(in_srgb,var(--accent)_5%,transparent)]" : "border-line",
                          !available && "cursor-not-allowed opacity-60",
                        )}
                      >
                        <input
                          type="radio"
                          name="nvidia-branch"
                          className="mt-0.5 size-3.5 accent-[var(--accent)]"
                          checked={selection === branch.id}
                          disabled={!available || busy}
                          onChange={() => setSelection(branch.id)}
                        />
                        <span className="min-w-0">
                          <span className="block text-[11.5px] font-medium text-ink">{t(branchLabel(branch.id))}</span>
                          <span className="mt-0.5 block font-mono text-[10px] text-subtle">
                            {available ? `v${version}` : branchUnavailable ? t("Branch data unavailable") : t("Not available for this kernel")}
                          </span>
                        </span>
                      </label>
                    );
                  })}
                </div>

                <div className="mt-3">
                  <label className="mb-1 block text-[10px] font-medium uppercase tracking-wide text-subtle">{t("Specific version")}</label>
                  <select
                    value={isBranch(selection) ? "" : selection}
                    disabled={busy || !versionList}
                    onChange={(event) => setSelection(event.target.value)}
                    className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
                  >
                    <option value="">{t("Choose a version…")}</option>
                    {(versionList?.versions ?? []).map((version) => (
                      <option key={version} value={version}>v{version}</option>
                    ))}
                  </select>
                </div>

                <div className="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 py-2.5">
                  <div className="min-w-0">
                    <p className="text-[11px] font-medium text-soft">
                      {resolved ? t("Will download v{version}.", { version: resolved }) : t("Choose an available driver.")}
                    </p>
                    <p className="mt-0.5 text-[10px] text-subtle">{t("A server reboot is required to install a downloaded driver.")}</p>
                  </div>
                  <Button
                    size="sm"
                    variant="primary"
                    icon={<Download className="size-3.5" />}
                    disabled={!resolved || busy}
                    loading={busy}
                    onClick={() => download.mutate(selection)}
                  >
                    {busy ? t("Downloading…") : t("Download driver")}
                  </Button>
                </div>

                {data.download?.active ? (
                  <div className="mt-3">
                    <div className="flex items-center justify-between text-[10px] text-subtle">
                      <span>{t("Downloading v{version}…", { version: data.download.version })}</span>
                      <span>{progress >= 0 ? `${progress}%` : t("Working…")}</span>
                    </div>
                    <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-[var(--surface-2)]">
                      <div
                        className={cn("h-full rounded-full bg-[var(--accent)] transition-all", progress < 0 && "w-full animate-pulse")}
                        style={progress >= 0 ? { width: `${progress}%` } : undefined}
                      />
                    </div>
                  </div>
                ) : null}

                {data.download?.error ? (
                  <p className="mt-3 rounded-[7px] border border-[var(--danger)]/30 bg-[var(--danger-soft)] px-3 py-2 text-[10.5px] text-[var(--danger)]">
                    {data.download.error}
                  </p>
                ) : null}

                {data.download?.done && !data.download.error ? (
                  <div className="mt-3 flex items-start gap-2 rounded-[7px] border border-[var(--success)]/30 bg-[var(--success-soft)] px-3 py-2">
                    <CheckCircle2 className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
                    <p className="text-[10.5px] leading-relaxed text-soft">
                      {t("Driver v{version} was downloaded and is ready to install.", { version: data.download.version })}
                    </p>
                  </div>
                ) : null}

                {staged ? (
                  <div className="mt-3 flex items-start justify-between gap-3 rounded-[7px] border border-line px-3 py-2">
                    <div className="min-w-0">
                      <p className="text-[10.5px] text-soft">
                        {t("Staged package: {package}", { package: data.downloadedPackage })}
                      </p>
                      <p className="mt-0.5 text-[10px] text-subtle">
                        {rebootNeeded ? t("Reboot the server to install v{version}.", { version: staged }) : t("This package matches the running driver.")}
                      </p>
                    </div>
                    {rebootNeeded ? <Badge tone="warning">{t("Reboot required")}</Badge> : null}
                  </div>
                ) : null}
              </>
            )}
          </Card>
        </>
      )}
    </div>
  );
}

function branchLabel(id: BranchID): string {
  switch (id) {
    case "latest_prb":
      return "Production Branch";
    case "latest_nfb":
      return "New Feature Branch";
    case "latest_nos":
      return "Open Source";
    default:
      return "Latest";
  }
}
