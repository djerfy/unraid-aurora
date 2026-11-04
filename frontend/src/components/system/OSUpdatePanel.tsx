import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowUpCircle,
  CheckCircle2,
  Download,
  ExternalLink,
  RefreshCw,
  RotateCcw,
  ServerCog,
} from "lucide-react";
import { Badge, Button, Card } from "../ui/primitives";
import { useToast } from "../../app/ToastProvider";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import type { OSStatus, PluginEvent } from "../../lib/types";

let lastAutoCheckAt = 0;

export function OSUpdatePanel() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [installing, setInstalling] = useState(false);
  const [log, setLog] = useState<string[]>([]);
  const logRef = useRef<HTMLPreElement | null>(null);

  const status = useQuery<OSStatus>({
    queryKey: ["os-status"],
    queryFn: api.osStatus,
  });

  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [log]);

  const check = useMutation({
    mutationFn: (_options?: { silent?: boolean }) => api.checkOSUpdate(),
    onSuccess: (result, options) => {
      queryClient.setQueryData(["os-status"], result);
      if (options?.silent) return;
      if (result.updateAvailable) toast({ kind: "info", title: t("Unraid OS {version} is available", { version: result.latest ?? "" }) });
      else if (result.checkError) toast({ kind: "warning", title: t("Could not check for Unraid OS updates"), description: result.checkError });
      else toast({ kind: "success", title: t("Unraid OS is up to date") });
    },
    onError: (error: Error, options) => {
      if (!options?.silent) toast({ kind: "error", title: t("Could not check for Unraid OS updates"), description: error.message });
      else console.error("automatic Unraid OS update check failed", error);
    },
  });

  // Opening the section is enough to look for an update; the automatic check
  // stays silent (the panel already surfaces the result) and is throttled so
  // quick back-and-forth navigation does not hammer the release service.
  const autoChecked = useRef(false);
  useEffect(() => {
    if (autoChecked.current || status.isLoading || check.isPending) return;
    autoChecked.current = true;
    if (Date.now() - lastAutoCheckAt < 30_000) return;
    lastAutoCheckAt = Date.now();
    check.mutate({ silent: true });
  }, [status.isLoading, check]);

  const restore = useMutation({
    mutationFn: api.restorePreviousOS,
    onSuccess: (result) => {
      toast({
        kind: "success",
        title: result.version ? t("Previous version {version} restored", { version: result.version }) : t("Previous version restored"),
        description: t("Reboot the server to boot the restored version."),
      });
      void queryClient.invalidateQueries({ queryKey: ["os-status"] });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not restore the previous version"), description: error.message }),
  });

  const install = async () => {
    setInstalling(true);
    setLog([]);
    try {
      let failure = "";
      await api.installOSUpdate((event: PluginEvent) => {
        if (event.line) setLog((current) => [...current, event.line]);
        if (event.error) failure = event.line;
      });
      if (failure) {
        toast({ kind: "error", title: t("Could not install the Unraid OS update"), description: failure });
      } else {
        toast({
          kind: "success",
          title: t("Unraid OS update installed"),
          description: t("Reboot the server to finish the update."),
        });
      }
    } catch (error) {
      toast({ kind: "error", title: t("Could not install the Unraid OS update"), description: (error as Error).message });
    } finally {
      setInstalling(false);
      void queryClient.invalidateQueries({ queryKey: ["os-status"] });
    }
  };

  const data = status.data;
  const busy = installing || restore.isPending;

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <ServerCog className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("Unraid OS")}</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">
                {t("Install official releases and roll back to the version kept on the flash drive.")}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {data?.version ? <Badge tone="neutral">Unraid {data.version}</Badge> : null}
            {data?.branch ? <Badge tone="info">{data.branch}</Badge> : null}
          </div>
        </div>

        {status.isLoading ? <p className="mt-4 text-[10.5px] text-muted">{t("Reading the installed Unraid OS…")}</p> : null}
        {status.isError ? <p className="mt-4 text-[10.5px] text-[var(--danger)]">{(status.error as Error).message}</p> : null}

        {data?.rebootRequired ? (
          <div className="mt-4 flex items-start gap-2 rounded-xl border border-[var(--warning-soft)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-[var(--warning)]">
            <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
            <div className="min-w-0">
              <p className="font-medium">
                {data.rebootType === "downgrade" ? t("Reboot required to finish the downgrade") : t("Reboot required to finish the update")}
                {data.rebootVersion ? ` — ${data.rebootVersion}` : ""}
              </p>
              <p className="mt-0.5 text-[10px] opacity-90">
                {data.thirdPartyDrivers
                  ? t("Third-party drivers are still downloading. Wait for the Unraid notification before rebooting.")
                  : t("Reboot the server from Unraid to boot the installed version.")}
              </p>
            </div>
          </div>
        ) : null}

        {data?.updateAvailable && !data.rebootRequired ? (
          <div className="mt-4 rounded-xl border border-[var(--accent-line)] bg-[var(--accent-softer)] p-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <ArrowUpCircle className="size-4 shrink-0 text-[var(--accent)]" />
                <p className="text-[11.5px] font-medium text-ink">
                  {t("Unraid {version} is available", { version: data.latest ?? "" })}
                  {data.releaseDate ? <span className="ml-1.5 font-normal text-subtle">· {data.releaseDate}</span> : null}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                {data.changelogPretty ? (
                  <Button size="sm" variant="ghost" icon={<ExternalLink className="size-3.5" />} onClick={() => window.open(data.changelogPretty, "_blank", "noopener")}>
                    {t("Release notes")}
                  </Button>
                ) : null}
                <Button size="sm" variant="primary" loading={installing} disabled={busy} onClick={() => {
                  if (window.confirm(t("Install Unraid OS {version}? The server must be rebooted afterwards.", { version: data.latest ?? "" }))) void install();
                }}>
                  {t("Install update")}
                </Button>
              </div>
            </div>
          </div>
        ) : null}

        {data && !data.updateAvailable && !data.rebootRequired && data.checkedAt ? (
          <p className="mt-4 flex items-center gap-1.5 text-[10.5px] text-[var(--success)]">
            <CheckCircle2 className="size-3.5" /> {t("Unraid OS is up to date")}
          </p>
        ) : null}

        {data?.checkError ? <p className="mt-3 text-[10px] leading-relaxed text-[var(--warning)]">{data.checkError}</p> : null}

        <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-line pt-4">
          <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} loading={check.isPending} disabled={busy} onClick={() => check.mutate({})}>
            {t("Check for updates")}
          </Button>
          {data?.checkedAt ? (
            <span className="text-[9.5px] text-subtle">{t("Last checked {time}", { time: new Date(data.checkedAt).toLocaleString() })}</span>
          ) : null}
        </div>
      </Card>

      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 className="text-[13px] font-semibold text-ink">{t("Downgrade")}</h2>
            <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
              {t("The installer keeps the previous boot files in /boot/previous. Restoring them swaps the kernel and Unraid OS on the next reboot.")}
            </p>
          </div>
          <Badge tone={data?.restoreAvailable ? "warning" : "neutral"} dot>
            {data?.restoreAvailable ? (data.previousVersion ? `v${data.previousVersion}` : t("Available")) : t("Not available")}
          </Badge>
        </div>
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <Button
            variant="secondary"
            icon={<RotateCcw className="size-3.5" />}
            loading={restore.isPending}
            disabled={busy || !data?.restoreAvailable}
            onClick={() => {
              if (window.confirm(t("Restore the previous Unraid OS version? The server must be rebooted afterwards."))) restore.mutate();
            }}
          >
            {t("Restore previous version")}
          </Button>
          {data?.previousVersion ? (
            <span className="text-[9.5px] text-subtle">
              {t("Previous version: {version}", { version: data.previousVersion })}
              {data.previousDate ? ` · ${data.previousDate}` : ""}
            </span>
          ) : (
            <span className="text-[9.5px] text-subtle">{t("A downgrade is only available after an update.")}</span>
          )}
        </div>
        <p className="mt-3 flex items-start gap-1.5 text-[9.5px] leading-relaxed text-subtle">
          <Download className="mt-0.5 size-3 shrink-0" />
          {t("Updates and downgrades never touch your array configuration. Keep a flash backup before a major version change.")}
        </p>
      </Card>

      {log.length > 0 ? (
        <Card className="card-pad">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-[13px] font-semibold text-ink">{t("Installer output")}</h2>
            {installing ? <Badge tone="info" dot pulse>{t("Installing")}</Badge> : null}
          </div>
          <pre ref={logRef} className="mt-3 max-h-64 max-w-full overflow-auto whitespace-pre-wrap break-words rounded-xl border border-[#1f2230] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed text-[#c7cbd6]">
            {log.join("\n")}
          </pre>
        </Card>
      ) : null}
    </div>
  );
}
