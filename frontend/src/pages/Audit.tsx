import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ClipboardList, Search, ShieldCheck } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Card, InputBase } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatDateTime } from "../lib/format";
import type { AuditEntry } from "../lib/types";

function statusTone(status: number) {
  if (status >= 200 && status < 300) return "success" as const;
  if (status >= 400 && status < 500) return "warning" as const;
  return "danger" as const;
}

export function AuditPage() {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const audit = useQuery({ queryKey: ["audit"], queryFn: () => api.auditLog(300), refetchInterval: 15_000 });

  const entries = useMemo(() => {
    const list = audit.data ?? [];
    const needle = query.trim().toLowerCase();
    if (!needle) return list;
    return list.filter((entry) =>
      `${entry.user} ${entry.method} ${entry.path} ${entry.status} ${entry.remote}`.toLowerCase().includes(needle),
    );
  }, [audit.data, query]);

  const actions = (audit.data ?? []).filter((entry) => entry.method !== "GET").length;

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Audit log")}
        description={t("Every mutating API call, who made it and whether the host accepted it.")}
        meta={
          <>
            <Badge tone="accent" dot>{t("{count} actions", { count: actions })}</Badge>
            <span className="chip">{t("Last 300 entries")}</span>
          </>
        }
      />
      <Card className="card-pad">
        <div className="flex flex-wrap items-center gap-3">
          <InputBase
            value={query}
            onChange={setQuery}
            placeholder={t("Search actions…")}
            icon={<Search className="size-3.5" />}
            className="w-full sm:w-72"
          />
          <span className="ml-auto flex items-center gap-1.5 text-[9.5px] text-subtle">
            <ShieldCheck className="size-3.5 text-[var(--success)]" />
            {t("Audit records are written to the Aurora config directory.")}
          </span>
        </div>

        {audit.isLoading ? (
          <p className="py-10 text-center text-[11px] text-muted">{t("Loading audit log…")}</p>
        ) : audit.isError ? (
          <p className="py-10 text-center text-[11px] text-[var(--danger)]">{(audit.error as Error).message}</p>
        ) : entries.length === 0 ? (
          <div className="grid min-h-[220px] place-items-center text-center">
            <div>
              <ClipboardList className="mx-auto size-5 text-subtle" />
              <p className="mt-2 text-[11px] font-medium text-soft">{t("No action recorded yet")}</p>
              <p className="mt-1 text-[10px] text-subtle">{t("Mutations will appear here as soon as you use Aurora.")}</p>
            </div>
          </div>
        ) : (
          <div className="mt-4 overflow-x-auto rounded-xl border border-line">
            <div className="min-w-[760px]">
              <div className="grid grid-cols-[150px_90px_100px_minmax(0,1fr)_70px_110px] gap-3 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[8.5px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <span>{t("Time")}</span>
                <span>{t("User")}</span>
                <span>{t("Method")}</span>
                <span>{t("Path")}</span>
                <span>{t("Status")}</span>
                <span>{t("Remote")}</span>
              </div>
              <div className="divide-y divide-line/60">
                {entries.map((entry: AuditEntry, index) => (
                  <div key={`${entry.time}-${entry.path}-${index}`} className="grid grid-cols-[150px_90px_100px_minmax(0,1fr)_70px_110px] items-center gap-3 px-3 py-2 text-[10px]">
                    <span className="num text-subtle">{formatDateTime(entry.time)}</span>
                    <span className="truncate text-soft">{entry.user}</span>
                    <span className="font-mono text-[9.5px] text-muted">{entry.method}</span>
                    <span className="truncate font-mono text-[9.5px] text-soft" title={entry.path}>{entry.path}</span>
                    <span><Badge tone={statusTone(entry.status)}>{entry.status}</Badge></span>
                    <span className="truncate font-mono text-[9px] text-subtle" title={entry.remote}>{entry.remote}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </Card>
    </>
  );
}
