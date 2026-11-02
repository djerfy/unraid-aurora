import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  CircleAlert,
  Download,
  Info,
  RefreshCw,
  ScrollText,
  Search,
  Terminal,
  TriangleAlert,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  InputBase,
  Switch,
  Tabs,
  cn,
} from "../components/ui/primitives";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatClock } from "../lib/format";
import type { LogLine } from "../lib/types";

const SOURCES = [
  { value: "syslog", label: "Syslog" },
  { value: "docker", label: "Docker" },
  { value: "nginx", label: "Nginx" },
  { value: "libvirt", label: "Libvirt" },
  { value: "maillog", label: "Maillog" },
  { value: "parity", label: "Parity" },
];

const SOURCE_PATHS: Record<string, string> = {
  syslog: "/var/log/syslog",
  docker: "/var/log/docker.log",
  nginx: "/var/log/nginx/*.log",
  libvirt: "/var/log/libvirt/libvirtd.log",
  maillog: "/var/log/maillog",
  parity: "/var/log/parity.log",
};

export function LogsPage() {
  const { toast } = useToast();
  const { t } = useI18n();
  const [source, setSource] = useState("syslog");
  const [level, setLevel] = useState("all");
  const [query, setQuery] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);

  const logs = useQuery({
    queryKey: ["logs", source],
    queryFn: () => api.logs(source, 300),
    refetchInterval: 6000,
  });

  const lines = logs.data ?? [];
  const filtered = useMemo(
    () =>
      lines.filter((line) => {
        if (level !== "all" && line.level !== level) return false;
        if (query.trim() && !line.message.toLowerCase().includes(query.toLowerCase())) return false;
        return true;
      }),
    [lines, level, query],
  );

  useEffect(() => {
    if (!autoScroll || !scrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [filtered, autoScroll]);

  const errors = lines.filter((line) => line.level === "error").length;
  const warnings = lines.filter((line) => line.level === "warn").length;
  const sourceTabs = SOURCES.map((item) => ({ ...item, label: t(item.label) }));

  const download = () => {
    const text = filtered.map((line) => `${line.time} ${line.level.toUpperCase()} ${line.message}`).join("\n");
    const blob = new Blob([text], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `aurora-${source}-${new Date().toISOString().slice(0, 19).replace(/:/g, "-")}.log`;
    anchor.click();
    URL.revokeObjectURL(url);
    toast({
      kind: "success",
      title: t("Log file downloaded"),
      description: t(
        filtered.length === 1 ? "{count} line exported." : "{count} lines exported.",
        { count: filtered.length },
      ),
    });
  };

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Logs")}
        description={t("Tail system, Docker, Nginx, libvirt, mail and parity logs with instant filtering — no SSH required.")}
        meta={
          <>
            <Badge tone={errors > 0 ? "danger" : "success"} dot>
              {t(errors === 1 ? "{count} error" : "{count} errors", { count: errors })}
            </Badge>
            <Badge tone={warnings > 0 ? "warning" : "neutral"}>
              {t(warnings === 1 ? "{count} warning" : "{count} warnings", { count: warnings })}
            </Badge>
            <span className="chip">
              {t(
                lines.length === 1 ? "{count} recent line" : "{count} recent lines",
                { count: lines.length },
              )}
            </span>
          </>
        }
        actions={
          <>
            <Button
              variant="secondary"
              icon={<RefreshCw className={cn("size-3.5", logs.isFetching && "animate-spin")} />}
              onClick={() => void logs.refetch()}
            >
              {t("Refresh")}
            </Button>
            <Button variant="primary" icon={<Download className="size-3.5" />} onClick={download}>
              {t("Export logs")}
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Stat icon={<ScrollText className="size-4" />} label={t("Source")} value={sourceTabs.find((item) => item.value === source)?.label ?? source} hint={t("active log stream")} />
        <Stat icon={<CircleAlert className="size-4" />} label={t("Errors")} value={String(errors)} hint={t("in current buffer")} tone={errors ? "danger" : "success"} />
        <Stat icon={<TriangleAlert className="size-4" />} label={t("Warnings")} value={String(warnings)} hint={t("in current buffer")} tone={warnings ? "warning" : "success"} />
        <Stat icon={<Info className="size-4" />} label={t("Visible")} value={String(filtered.length)} hint={t("after filters")} />
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Tabs items={sourceTabs} value={source} onChange={setSource} />
        <div className="flex items-center gap-1">
          {[
            { value: "all", label: "All" },
            { value: "info", label: "Info" },
            { value: "warn", label: "Warn" },
            { value: "error", label: "Error" },
          ].map((item) => (
            <button
              key={item.value}
              type="button"
              onClick={() => setLevel(item.value)}
              className={cn(
                "h-7 rounded-md px-2 text-[11px] font-medium transition",
                level === item.value ? "bg-[var(--surface-3)] text-ink" : "text-muted hover:text-ink",
              )}
            >
              {t(item.label)}
            </button>
          ))}
        </div>
        <div className="ml-auto flex items-center gap-3">
          <label className="flex items-center gap-2 text-[10.5px] text-muted">
            <Switch checked={autoScroll} onChange={setAutoScroll} label={t("Auto scroll")} />
            {t("Follow")}
          </label>
          <InputBase
            value={query}
            onChange={setQuery}
            placeholder={t("Filter lines…")}
            icon={<Search className="size-3.5" />}
            className="w-52"
          />
        </div>
      </div>

      <Card className="overflow-hidden border-[#1f2230] bg-[#08090d]">
        <div className="flex items-center gap-2 border-b border-[#1f2230] px-4 py-2.5">
          <span className="flex items-center gap-2 font-mono text-[10px] text-[#8b93a7]">
            <Terminal className="size-3" /> {SOURCE_PATHS[source] ?? source}
          </span>
          <span className="ml-auto flex items-center gap-1.5 font-mono text-[9px] text-[#5f6678]">
            <span className={cn("size-1.5 rounded-full", logs.isFetching ? "bg-[#f5b44a]" : "bg-[#3ddc97]")} />
            {logs.isFetching ? t("syncing") : t("live tail")}
          </span>
        </div>

        <div
          ref={scrollRef}
          className="h-[58vh] min-h-[360px] overflow-y-auto p-4 font-mono text-[10.5px] leading-[1.75]"
        >
          {logs.isLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 14 }).map((_, index) => (
                <div key={index} className="h-3 animate-pulse rounded bg-[#1a1d28]" style={{ width: `${45 + ((index * 17) % 45)}%` }} />
              ))}
            </div>
          ) : logs.isError ? (
            <div className="rounded-xl border border-[#3a1f26] bg-[#1a0f13] p-4">
              <p className="text-[11px] leading-relaxed text-[#ff8a9b]">{(logs.error as Error).message}</p>
              <Button
                size="sm"
                variant="secondary"
                className="mt-3"
                icon={<RefreshCw className="size-3" />}
                onClick={() => void logs.refetch()}
              >
                {t("Try again")}
              </Button>
            </div>
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={<Search className="size-5" />}
              title={t("No log lines match")}
              description={t("Adjust the level or search query to widen the result set.")}
              className="text-white"
            />
          ) : (
            filtered.map((line, index) => <LogRow key={`${line.time}-${index}`} line={line} />)
          )}
        </div>

        <div className="flex items-center justify-between border-t border-[#1f2230] px-4 py-2 font-mono text-[9px] text-[#5f6678]">
          <span>
            {t(filtered.length === 1 ? "{count} line · buffer 300" : "{count} lines · buffer 300", {
              count: filtered.length,
            })}
          </span>
          <span>{t("Aurora log viewer · refresh 6s")}</span>
        </div>
      </Card>
    </>
  );
}

function LogRow({ line }: { line: LogLine }) {
  const color =
    line.level === "error"
      ? "text-[#ff6b81]"
      : line.level === "warn"
        ? "text-[#f5b44a]"
        : line.level === "debug"
          ? "text-[#7c86a0]"
          : "text-[#5aa9ff]";
  return (
    <div className="flex gap-3 rounded px-1 py-px transition hover:bg-[#11131b]">
      <span className="shrink-0 text-[#4b5263]">{formatClock(line.time)}</span>
      <span className={cn("w-11 shrink-0 uppercase", color)}>{line.level}</span>
      <span className="min-w-0 flex-1 whitespace-pre-wrap break-words text-[#c7cbd6]">{line.message}</span>
    </div>
  );
}

function Stat({
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
  tone?: "warning" | "danger" | "success";
}) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p
        className={cn(
          "num mt-3 text-[21px] font-semibold leading-none",
          tone === "danger" ? "text-[var(--danger)]" : tone === "warning" ? "text-[var(--warning)]" : tone === "success" ? "text-[var(--success)]" : "text-ink",
        )}
      >
        {value}
      </p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}
