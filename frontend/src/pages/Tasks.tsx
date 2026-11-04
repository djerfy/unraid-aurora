import { useQuery } from "@tanstack/react-query";
import { Activity, CheckCircle2, CircleAlert, Clock3, RefreshCw } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Button, Card, EmptyState, Progress } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatDateTime, relativeTime } from "../lib/format";

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
  const queued = values.filter((task) => task.status === "queued").length;
  const failed = values.filter((task) => task.status === "failed").length;

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Tasks")}
        description={t("Background work running on this server: share scans, mover, balance and Trivy.")}
        meta={
          <>
            <Badge tone={running ? "accent" : "success"} dot pulse={running > 0}>{t("{count} running", { count: running })}</Badge>
            <Badge tone="warning">{t("{count} queued", { count: queued })}</Badge>
            {failed ? <Badge tone="danger">{t("{count} failed", { count: failed })}</Badge> : null}
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
          <EmptyState icon={<Activity className="size-5" />} title={t("No background tasks")} description={t("Share scans, mover runs, balance jobs and Trivy operations appear here.")} />
        ) : (
          <div className="overflow-x-auto rounded-xl border border-line">
            <div className="min-w-[860px]">
              <div className="grid grid-cols-[110px_minmax(0,1fr)_120px_150px_150px_90px] gap-3 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[8.5px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <span>{t("Status")}</span>
                <span>{t("Task")}</span>
                <span>{t("Category")}</span>
                <span>{t("Started")}</span>
                <span>{t("Updated")}</span>
                <span className="text-right">{t("Progress")}</span>
              </div>
              <div className="divide-y divide-line/60">
                {values.map((task) => (
                  <div key={task.id} className="grid grid-cols-[110px_minmax(0,1fr)_120px_150px_150px_90px] items-center gap-3 px-3 py-2.5 text-[10px]">
                    <span className="flex items-center gap-1.5">
                      <Badge tone={statusTone(task.status)} dot={task.status === "running"} pulse={task.status === "running"}>
                        <span className="flex items-center gap-1">{statusIcon(task.status)}{statusLabel(t, task.status)}</span>
                      </Badge>
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate font-medium text-soft">{task.name}</span>
                      <span className="block truncate text-[9px] text-subtle">{task.detail || "—"}</span>
                      {task.error ? <span className="mt-0.5 block truncate text-[9px] text-[var(--danger)]">{task.error}</span> : null}
                    </span>
                    <span className="truncate text-[9.5px] capitalize text-muted">{t(task.category)}</span>
                    <span className="truncate text-[9px] text-subtle" title={task.startedAt ? formatDateTime(task.startedAt) : undefined}>
                      {task.startedAt ? relativeTime(task.startedAt) : "—"}
                    </span>
                    <span className="truncate text-[9px] text-subtle" title={task.updatedAt ? formatDateTime(task.updatedAt) : undefined}>
                      {task.updatedAt ? relativeTime(task.updatedAt) : "—"}
                    </span>
                    <span className="flex items-center justify-end gap-2">
                      {task.status === "running" ? <Progress value={task.progress} height={4} className="w-12" /> : null}
                      <span className="num text-[9px] text-subtle">{Math.round(task.progress)}%</span>
                    </span>
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
