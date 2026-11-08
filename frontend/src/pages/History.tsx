import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileDiff, History as HistoryIcon, RotateCcw, Search, ShieldAlert } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Drawer, Modal } from "../components/ui/overlays";
import { Badge, Button, Card, EmptyState, InputBase, cn } from "../components/ui/primitives";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatBytes, formatDateTime, relativeTime } from "../lib/format";
import type { ConfigHistory, ConfigFileChange } from "../lib/types";

export function HistoryPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [rollbackTarget, setRollbackTarget] = useState<ConfigHistory | null>(null);

  const history = useQuery({
    queryKey: ["config-history"],
    queryFn: api.configHistory,
    refetchInterval: 60_000,
  });

  const detail = useQuery({
    queryKey: ["config-history", selectedId],
    queryFn: () => api.configHistoryDetail(selectedId ?? ""),
    enabled: Boolean(selectedId),
  });

  const rollback = useMutation({
    mutationFn: (id: string) => api.rollbackConfig(id),
    onSuccess: () => {
      toast({
        kind: "success",
        title: t("Rollback applied"),
        description: t("The previous file contents were restored."),
      });
      setRollbackTarget(null);
      setSelectedId(null);
      void queryClient.invalidateQueries({ queryKey: ["config-history"] });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Rollback failed"), description: error.message });
    },
  });

  const entries = history.data ?? [];
  const filtered = useMemo(() => {
    if (!query.trim()) return entries;
    const needle = query.toLowerCase();
    return entries.filter((entry) =>
      `${entry.action} ${entry.description} ${entry.files.map((file) => file.path).join(" ")}`
        .toLowerCase()
        .includes(needle),
    );
  }, [entries, query]);

  const open = (entry: ConfigHistory) => {
    setSelectedId(entry.id);
  };

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("History")}
        description={t("Configuration and container image changes are recorded here. File-backed entries can be rolled back; image digest changes are informational.")}
        meta={
          <>
            <Badge tone="accent" dot>{t(entries.length === 1 ? "{count} entry" : "{count} entries", { count: entries.length })}</Badge>
            {entries[0] ? (
              <span className="chip">{t("last {time}", { time: relativeTime(entries[0].created) })}</span>
            ) : null}
          </>
        }
        actions={
          <InputBase
            value={query}
            onChange={setQuery}
            placeholder={t("Search actions or files…")}
            icon={<Search className="size-3.5" />}
            className="w-60"
          />
        }
      />

      {entries.length === 0 ? (
        <Card>
          <EmptyState
            icon={<HistoryIcon className="size-5" />}
            title={t("No configuration changes yet")}
            description={t("Editing a container template, toggling autostart, updating a container image or any other setting will create an entry here.")}
          />
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="divide-y divide-line/60">
            {filtered.map((entry) => (
              <div key={entry.id} className="flex flex-wrap items-center gap-3 px-5 py-3.5 transition hover:bg-[var(--surface-2)]/45">
                <span className="grid size-8 shrink-0 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-[var(--violet)]">
                  <FileDiff className="size-3.5" />
                </span>
                <button type="button" onClick={() => open(entry)} className="min-w-0 flex-1 text-left">
                  <p className="truncate text-[12px] font-medium text-ink">{entry.description || entry.action}</p>
                  <p className="mt-0.5 truncate font-mono text-[9.5px] text-subtle">
                    {entry.files.map((file) => file.path).join(" · ") || t("no files")}
                  </p>
                </button>
                <span className="hidden shrink-0 text-right text-[10px] text-subtle sm:block">
                  <span className="block text-muted">{formatDateTime(entry.created)}</span>
                  <span className="font-mono text-[9px]">{entry.action}</span>
                </span>
                <Badge tone={entry.files.length > 0 && entry.files.every((file) => file.virtual) ? "info" : "neutral"}>
                  {entry.files.length > 0 && entry.files.every((file) => file.virtual)
                    ? t("image")
                    : entry.files.length === 0
                      ? t("Event")
                      : t(entry.files.length === 1 ? "{count} file" : "{count} files", { count: entry.files.length })}
                </Badge>
                <Button size="sm" variant="secondary" icon={<FileDiff className="size-3" />} onClick={() => open(entry)}>
                  {t("Details")}
                </Button>
              </div>
            ))}
            {filtered.length === 0 ? (
              <p className="px-5 py-8 text-center text-[11px] text-muted">
                {t("No entry matches “{query}”.", { query })}
              </p>
            ) : null}
          </div>
        </Card>
      )}

      <Drawer
        open={Boolean(selectedId)}
        onClose={() => setSelectedId(null)}
        title={t("Configuration change")}
        subtitle={detail.data?.description ?? t("Loading…")}
        footer={
          detail.data ? (
            <div className="flex flex-wrap gap-2">
              {detail.data.files.length > 0 && detail.data.files.every((file) => !file.virtual) ? (
                <Button
                  variant="danger"
                  icon={<RotateCcw className="size-3.5" />}
                  onClick={() => setRollbackTarget(detail.data ?? null)}
                >
                  {t("Rollback")}
                </Button>
              ) : null}
            </div>
          ) : null
        }
      >
        {detail.isLoading ? (
          <p className="text-[11px] text-muted">{t("Loading history entry…")}</p>
        ) : detail.isError ? (
          <p className="text-[11px] text-[var(--danger)]">{(detail.error as Error).message}</p>
        ) : detail.data ? (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <Meta label={t("Action")} value={detail.data.action} />
              <Meta label={t("Date")} value={formatDateTime(detail.data.created)} />
              <Meta label={t("Files")} value={String(detail.data.files.length)} />
              <Meta label={t("ID")} value={detail.data.id} mono />
            </div>
            {detail.data.files.some((file) => file.virtual) ? (
              <p className="rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] px-3 py-2 text-[10.5px] leading-relaxed text-[var(--warning)]">
                {t("Container image updates are informational. Deploy the previous image tag to roll back.")}
              </p>
            ) : null}
            {detail.data.files.map((file) => (
              <div key={file.path} className="rounded-xl border border-line">
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line px-3 py-2">
                  <span className="min-w-0 truncate font-mono text-[10px] text-soft">{file.path}</span>
                  <span className="flex items-center gap-2">
                    <Badge tone={file.virtual ? "info" : file.existed ? "success" : "warning"}>
                      {file.virtual ? t("image") : file.existed ? t("existed") : t("created")}
                    </Badge>
                    {file.virtual ? null : <span className="text-[9.5px] text-subtle">{formatBytes(file.size)}</span>}
                  </span>
                </div>
                <DiffBlock diff={file.diff} file={file} />
              </div>
            ))}
          </div>
        ) : null}
      </Drawer>

      <Modal
        open={Boolean(rollbackTarget)}
        onClose={() => setRollbackTarget(null)}
        title={t("Rollback this change?")}
        description={rollbackTarget?.description}
        footer={
          <>
            <Button variant="secondary" onClick={() => setRollbackTarget(null)} disabled={rollback.isPending}>
              {t("Cancel")}
            </Button>
            <Button
              variant="danger"
              icon={<RotateCcw className="size-3.5" />}
              loading={rollback.isPending}
              onClick={() => {
                if (rollbackTarget) rollback.mutate(rollbackTarget.id);
              }}
            >
              {t("Rollback")}
            </Button>
          </>
        }
      >
        <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
          <ShieldAlert className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
          {t("The current contents are snapshotted before restoring, so this rollback can itself be reverted from the same page.")}
        </div>
      </Modal>
    </>
  );
}

function DiffBlock({ diff, file }: { diff: string; file: ConfigFileChange }) {
  const { t } = useI18n();
  if (!diff) {
    return <p className="px-3 py-2 text-[10.5px] text-subtle">{t("No textual difference.")}</p>;
  }
  return (
    <div className="max-h-80 overflow-auto bg-[#08090d] px-3 py-2 font-mono text-[10px] leading-relaxed">
      {diff.split("\n").map((line, index) => {
        const added = line.startsWith("+") && !line.startsWith("+++");
        const removed = line.startsWith("-") && !line.startsWith("---");
        const hunk = line.startsWith("@@");
        return (
          <div
            key={`${file.path}-${index}`}
            className={cn(
              "whitespace-pre-wrap",
              added ? "text-[var(--success)]" : removed ? "text-[var(--danger)]" : hunk ? "text-[var(--accent)]" : "text-[#8b93a7]",
            )}
          >
            {line || " "}
          </div>
        );
      })}
    </div>
  );
}

function Meta({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2">
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className={cn("mt-0.5 truncate text-[11px]", mono ? "font-mono text-[10px] text-soft" : "text-soft")}>{value}</p>
    </div>
  );
}
