import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, AlertTriangle, Play, RefreshCw, RotateCcw, Square, Terminal } from "lucide-react";
import { Badge, Button, Card } from "../ui/primitives";
import { useToast } from "../../app/ToastProvider";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import type { LogLine, UnraidAPIStatus } from "../../lib/types";

export function UnraidAPIPanel() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [lines, setLines] = useState(200);

  const status = useQuery<UnraidAPIStatus>({
    queryKey: ["unraid-api"],
    queryFn: api.unraidAPIStatus,
    refetchInterval: 30_000,
  });
  const logs = useQuery<LogLine[]>({
    queryKey: ["unraid-api-logs", lines],
    queryFn: () => api.unraidAPILogs(lines),
    enabled: Boolean(status.data?.installed),
  });

  const action = useMutation({
    mutationFn: (name: "start" | "stop" | "restart") => api.unraidAPIAction(name),
    onSuccess: (_result, name) => {
      toast({ kind: "success", title: t("Unraid API {action} requested", { action: name }) });
      void queryClient.invalidateQueries({ queryKey: ["unraid-api"] });
      setTimeout(() => void logs.refetch(), 2_000);
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Unraid API action failed"), description: error.message }),
  });

  const data = status.data;
  const running = Boolean(data?.running);
  const stateTone = !data?.installed ? "neutral" : running ? "success" : data.state === "errored" ? "danger" : "warning";

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <Activity className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("Unraid API")}</h2>
              <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                {t("The official API service used by Unraid Connect and integrations. Manage its process and read its logs here.")}
              </p>
            </div>
          </div>
          <Badge tone={stateTone} dot pulse={running}>
            {!data?.installed ? t("Not installed") : running ? t("Running") : data?.state ?? t("Stopped")}
          </Badge>
        </div>

        {status.isLoading ? <p className="mt-4 text-[10.5px] text-muted">{t("Reading the Unraid API status…")}</p> : null}
        {status.isError ? <p className="mt-4 text-[10.5px] text-[var(--danger)]">{(status.error as Error).message}</p> : null}

        {data && !data.installed ? (
          <div className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
            <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
            {t("The Unraid API binary was not found. Install the Unraid Connect plugin to expose the official API service.")}
          </div>
        ) : null}

        {data?.installed ? (
          <>
            <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Info label={t("Version")} value={data.version ? `v${data.version}` : "—"} />
              <Info label={t("State")} value={data.state || "—"} />
              <Info label={t("Log level")} value={data.logLevel || "—"} />
            </div>
            <div className="mt-4 flex flex-wrap gap-2 border-t border-line pt-4">
              <Button size="sm" variant="secondary" icon={<Play className="size-3.5" />} loading={action.isPending && action.variables === "start"} disabled={running || action.isPending} onClick={() => action.mutate("start")}>
                {t("Start")}
              </Button>
              <Button size="sm" variant="secondary" icon={<RotateCcw className="size-3.5" />} loading={action.isPending && action.variables === "restart"} disabled={action.isPending} onClick={() => action.mutate("restart")}>
                {t("Restart")}
              </Button>
              <Button size="sm" variant="danger" icon={<Square className="size-3.5" />} loading={action.isPending && action.variables === "stop"} disabled={!running || action.isPending} onClick={() => {
                if (window.confirm(t("Stop the Unraid API? Unraid Connect and API integrations will be unavailable until it is started again."))) action.mutate("stop");
              }}>
                {t("Stop")}
              </Button>
            </div>
            {data.config ? (
              <details className="mt-4 rounded-xl border border-line bg-[var(--surface-2)]/40 p-3">
                <summary className="cursor-pointer text-[10px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Service configuration")}</summary>
                <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap break-words font-mono text-[9.5px] leading-relaxed text-muted">{data.config}</pre>
              </details>
            ) : null}
          </>
        ) : null}
      </Card>

      {data?.installed ? (
        <Card className="card-pad">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <Terminal className="size-3.5 text-subtle" />
              <h2 className="text-[13px] font-semibold text-ink">{t("API logs")}</h2>
            </div>
            <div className="flex items-center gap-2">
              <select
                value={lines}
                onChange={(event) => setLines(Number(event.target.value))}
                aria-label={t("Log lines")}
                className="h-8 rounded-md border border-line bg-[var(--surface-2)] px-2 text-[10px] text-soft outline-none focus:border-[var(--accent-line)]"
              >
                {[100, 200, 500, 1000].map((value) => <option key={value} value={value}>{t("{count} lines", { count: value })}</option>)}
              </select>
              <Button size="sm" variant="ghost" icon={<RefreshCw className="size-3" />} loading={logs.isFetching} onClick={() => void logs.refetch()}>
                {t("Refresh")}
              </Button>
            </div>
          </div>
          {logs.isLoading ? <p className="mt-3 text-[10.5px] text-muted">{t("Loading API logs…")}</p> : null}
          {logs.isError ? <p className="mt-3 text-[10.5px] text-[var(--danger)]">{(logs.error as Error).message}</p> : null}
          {logs.data ? (
            <pre className="mt-3 max-h-80 max-w-full overflow-auto whitespace-pre-wrap break-words rounded-xl border border-[#1f2230] bg-[#08090d] p-3 font-mono text-[9.5px] leading-relaxed text-[#c7cbd6]">
              {logs.data.length ? logs.data.map((entry) => entry.message).join("\n") : t("The log file is empty.")}
            </pre>
          ) : null}
        </Card>
      ) : null}
    </div>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2">
      <p className="text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-0.5 truncate font-mono text-[11px] text-soft">{value}</p>
    </div>
  );
}
