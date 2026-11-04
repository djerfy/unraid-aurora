import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Activity, CheckCircle2, CircleAlert, Clock3, Info as InfoIcon, RefreshCw } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Modal } from "../components/ui/overlays";
import { Badge, Button, Card, EmptyState, IconButton, Progress } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatDateTime, formatDuration, relativeTime } from "../lib/format";
import type { Task } from "../lib/types";

function statusTone(status: string): "accent" | "success" | "warning" | "danger" | "neutral" {
  switch (status) {
    case "running": return "accent";
    case "completed": return "success";
    case "queued": return "warning";
    case "failed": return "danger";
    default: return "neutral";
  }
}

function statusLabel(t: (key: string) => string, status: string): string {
  switch (status) {
    case "running": return t("Running");
    case "completed": return t("Completed");
    case "queued": return t("Queued");
    case "failed": return t("Failed");
    default: return status;
  }
}

function statusIcon(status: string) {
  switch (status) {
    case "running": return <Activity className="size-3.5" />;
    case "completed": return <CheckCircle2 className="size-3.5" />;
    case "failed": return <CircleAlert className="size-3.5" />;
    default: return <Clock3 className="size-3.5" />;
  }
}

export function TasksPage() {
  const { t } = useI18n();
  const tasks = useQuery({ queryKey: ["tasks"], queryFn: api.tasks, refetchInterval: 5_000 });
  const values = tasks.data ?? [];
  const running = values.filter((task) => task.status === "running").length;
  const [selected, setSelected] = useState<Task | null>(null);

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Tasks")}
        description={t("Active work only: share scans, mover, balance and Trivy. Finished tasks move to Logs → Tasks.")}
        meta={
          <>
            <Badge tone={running ? "accent" : "success"} dot pulse={running > 0}>{t("{count} running", { count: running })}</Badge>
          </>
        }
        actions={
          <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} loading={tasks.isFetching} onClick={() => void tasks.refetch()}>
            {t("Refresh")}
          </Button>
        }
      />

      <Card className="card-pad">
        {tasks.isLoading ? (
          <p className="py-10 text-center text-[11px] text-muted">{t("Loading tasks…")}</p>
        ) : tasks.isError ? (
          <p className="py-10 text-center text-[11px] text-[var(--danger)]">{(tasks.error as Error).message}</p>
        ) : values.length === 0 ? (
          <EmptyState icon={<Activity className="size-5" />} title={t("No task is running")} description={t("Active share scans, mover runs, balance jobs and Trivy scans appear here. Finished work is listed in Logs → Tasks.")} />
        ) : (
          <div className="overflow-x-auto rounded-xl border border-line">
            <div className="min-w-[940px]">
              <div className="grid grid-cols-[110px_minmax(0,1fr)_110px_80px_130px_130px_100px_32px] gap-3 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[8.5px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <span>{t("Status")}</span>
                <span>{t("Task")}</span>
                <span>{t("Category")}</span>
                <span>{t("PID")}</span>
                <span>{t("Started")}</span>
                <span>{t("Updated")}</span>
                <span className="text-right">{t("Progress")}</span>
                <span />
              </div>
              <div className="divide-y divide-line/60">
                {values.map((task) => (
                  <div key={task.id} className="grid grid-cols-[110px_minmax(0,1fr)_110px_80px_130px_130px_100px_32px] items-center gap-3 px-3 py-2.5 text-[10px]">
                    <span className="flex items-center gap-1.5">
                      <Badge tone={statusTone(task.status)} dot={task.status === "running"} pulse={task.status === "running"}>
                        <span className="flex items-center gap-1">{statusIcon(task.status)}{statusLabel(t, task.status)}</span>
                      </Badge>
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate font-medium text-soft">{task.name}</span>
                      <span className="block truncate text-[9px] text-subtle">{task.description ? `${t(task.description)} · ` : ""}{task.detail ? t(task.detail) : "—"}</span>
                      {task.path ? <span className="block truncate font-mono text-[8.5px] text-subtle">{task.path}</span> : null}
                      {task.error ? <span className="mt-0.5 block truncate text-[9px] text-[var(--danger)]">{task.error}</span> : null}
                    </span>
                    <span className="truncate text-[9.5px] capitalize text-muted">{t(task.category)}</span>
                    <span className="num truncate text-[9px] text-subtle">{task.pid ? task.pid : "—"}</span>
                    <span className="truncate text-[9px] text-subtle" title={task.startedAt ? formatDateTime(task.startedAt) : undefined}>
                      {task.startedAt ? relativeTime(task.startedAt) : "—"}
                    </span>
                    <span className="truncate text-[9px] text-subtle" title={task.updatedAt ? formatDateTime(task.updatedAt) : undefined}>
                      {task.updatedAt ? relativeTime(task.updatedAt) : "—"}
                    </span>
                    <span className="flex items-center justify-end gap-2">
                      {task.progress >= 0 ? <Progress value={task.progress} height={4} className="w-12" /> : null}
                      <span className="num text-[9px] text-subtle">{task.progress >= 0 ? `${Math.round(task.progress)}%` : "—"}</span>
                    </span>
                    <IconButton label={t("Task details")} size="sm" onClick={() => setSelected(task)}>
                      <InfoIcon className="size-3.5" />
                    </IconButton>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </Card>

      <Modal
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        title={selected?.name ?? ""}
        description={selected?.description ?? ""}
        size="lg"
        footer={<Button variant="secondary" onClick={() => setSelected(null)}>{t("Close")}</Button>}
      >
        {selected ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <Info label={t("Status")} value={statusLabel(t, selected.status)} />
            <Info label={t("Category")} value={t(selected.category)} />
            <Info label={t("PID")} value={selected.pid ? String(selected.pid) : "—"} mono />
            <Info label={t("Trigger")} value={selected.trigger ? t(selected.trigger) : "—"} />
            <Info label={t("Duration")} value={selected.duration > 0 ? formatDuration(selected.duration) : "—"} />
            <Info label={t("Started")} value={selected.startedAt ? formatDateTime(selected.startedAt) : "—"} />
            <Info label={t("Updated")} value={selected.updatedAt ? formatDateTime(selected.updatedAt) : "—"} />
            <Info label={t("Progress")} value={selected.progress >= 0 ? `${Math.round(selected.progress)}%` : "—"} />
            <div className="sm:col-span-2">
              <Info label={t("Detail")} value={selected.detail ? t(selected.detail) : "—"} />
            </div>
            {selected.path ? <div className="sm:col-span-2"><Info label={t("Path")} value={selected.path} mono /></div> : null}
            {selected.command ? <div className="sm:col-span-2"><Info label={t("Command")} value={selected.command} mono /></div> : null}
            {selected.error ? (
              <div className="sm:col-span-2 rounded-lg border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-[10px] text-[var(--danger)]">{selected.error}</div>
            ) : null}
          </div>
        ) : null}
      </Modal>
    </>
  );
}

function Info({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/45 px-3 py-2">
      <p className="text-[8.5px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className={mono ? "mt-1 break-all font-mono text-[10px] text-soft" : "mt-1 text-[10.5px] text-soft"}>{value}</p>
    </div>
  );
}
