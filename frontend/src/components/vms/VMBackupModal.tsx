import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ArchiveRestore, HardDriveDownload, ShieldAlert } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { formatBytes } from "../../lib/format";
import { Badge, Button, Progress } from "../ui/primitives";
import { Modal } from "../ui/overlays";
import type { VM, VMBackup } from "../../lib/types";

function stateTone(state: string): "success" | "warning" | "danger" | "neutral" {
  if (state === "completed") return "success";
  if (state === "running" || state === "cancelling") return "warning";
  if (state === "failed" || state === "cancelled") return "danger";
  return "neutral";
}

export function VMBackupModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const [dir, setDir] = useState("/mnt/user/backups");
  const [restoreDir, setRestoreDir] = useState("");
  const [confirm, setConfirm] = useState(false);

  useEffect(() => {
    setRestoreDir("");
    setConfirm(false);
  }, [vm?.id]);

  const status = useQuery({
    queryKey: ["vm-backup", vm?.id],
    queryFn: () => api.vmBackupStatus(vm?.id ?? ""),
    enabled: Boolean(vm),
    refetchInterval: (query) => {
      const state = query.state.data?.state;
      return state === "running" || state === "cancelling" ? 1000 : false;
    },
  });

  const job: VMBackup | undefined = status.data;
  const busy = job?.state === "running" || job?.state === "cancelling";
  const vmRunning = vm?.state === "running" || vm?.state === "paused";
  const percent = job && job.totalBytes > 0 ? Math.min(100, (job.doneBytes / job.totalBytes) * 100) : 0;

  const start = useMutation({
    mutationFn: () => api.vmBackup(vm?.id ?? "", dir.trim()),
    onSuccess: () => {
      toast({ kind: "success", title: t("Backup started"), description: dir.trim() });
      void status.refetch();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Backup failed"), description: error.message }),
  });

  const cancel = useMutation({
    mutationFn: () => api.cancelVMBackup(vm?.id ?? ""),
    onSuccess: () => {
      toast({ kind: "info", title: t("Cancelling") });
      void status.refetch();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not cancel the operation"), description: error.message }),
  });

  const restore = useMutation({
    mutationFn: () => api.restoreVMBackup(vm?.id ?? "", restoreDir.trim(), confirm),
    onSuccess: () => {
      toast({ kind: "success", title: t("Restore started"), description: restoreDir.trim() });
      void status.refetch();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Restore failed"), description: error.message }),
  });

  return (
    <Modal
      open={Boolean(vm)}
      onClose={onClose}
      title={vm ? `${vm.name} · ${t("Backup")}` : t("Backup")}
      description={t("Offline backup of the domain XML, NVRAM and every disk.")}
      size="lg"
    >
      <div className="space-y-5">
        {vmRunning ? (
          <div className="flex items-start gap-2 rounded-xl border border-[var(--warning-soft)] bg-[var(--warning-soft)] px-3 py-2.5 text-[11px] leading-relaxed text-[var(--warning)]">
            <ShieldAlert className="mt-0.5 size-3.5 shrink-0" />
            {t("Shut down the virtual machine before backing it up or restoring it.")}
          </div>
        ) : null}

        <section>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
            <HardDriveDownload className="size-3.5" /> {t("Back up")}
          </div>
          <p className="mt-1.5 text-[10.5px] leading-relaxed text-muted">
            {t("Aurora writes a manifest, the domain XML, NVRAM and qcow2 copies of each disk into a new folder under /mnt.")}
          </p>
          <div className="mt-2 flex gap-2">
            <input
              value={dir}
              onChange={(event) => setDir(event.target.value)}
              placeholder="/mnt/user/backups"
              aria-label={t("Backup directory")}
              className="h-8 min-w-0 flex-1 rounded-md border border-line bg-[var(--surface-2)] px-2 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]"
            />
            <Button
              size="sm"
              variant="primary"
              disabled={vmRunning || busy || !dir.trim() || start.isPending}
              loading={start.isPending}
              onClick={() => start.mutate()}
            >
              {t("Start backup")}
            </Button>
          </div>
        </section>

        {job && job.kind === "backup" && job.state !== "idle" ? (
          <section className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone={stateTone(job.state)} dot>{t(job.state === "running" ? "Running" : job.state === "completed" ? "Completed" : job.state === "cancelled" ? "Cancelled" : job.state === "failed" ? "Failed" : "Cancelling")}</Badge>
              <span className="text-[10.5px] text-muted">{job.message || job.phase}</span>
              <span className="ml-auto font-mono text-[10px] text-subtle">
                {formatBytes(job.doneBytes)} / {formatBytes(job.totalBytes)}
              </span>
            </div>
            <Progress value={percent} height={5} className="mt-2.5" tone={job.state === "failed" ? "var(--danger)" : "var(--accent)"} />
            <p className="mt-2 break-all font-mono text-[9.5px] text-subtle">{job.dir}</p>
            {job.error ? <p className="mt-1.5 text-[10.5px] text-[var(--danger)]">{job.error}</p> : null}
            {job.files.length ? (
              <p className="mt-1.5 text-[10px] text-muted">{t("{count} files saved", { count: job.files.length })}</p>
            ) : null}
            {job.state === "running" || job.state === "cancelling" ? (
              <Button size="sm" variant="secondary" className="mt-2" disabled={cancel.isPending} onClick={() => cancel.mutate()}>
                {t("Cancel")}
              </Button>
            ) : null}
          </section>
        ) : null}

        <section>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
            <ArchiveRestore className="size-3.5" /> {t("Restore")}
          </div>
          <p className="mt-1.5 text-[10.5px] leading-relaxed text-muted">
            {t("Restore from an Aurora backup folder. Existing disks are moved aside and restored if the operation fails.")}
          </p>
          <div className="mt-2 flex gap-2">
            <input
              value={restoreDir}
              onChange={(event) => setRestoreDir(event.target.value)}
              placeholder="/mnt/user/backups/aurora-vm-..."
              aria-label={t("Backup folder to restore")}
              className="h-8 min-w-0 flex-1 rounded-md border border-line bg-[var(--surface-2)] px-2 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]"
            />
            <Button
              size="sm"
              variant="danger"
              disabled={vmRunning || busy || !restoreDir.trim() || !confirm || restore.isPending}
              loading={restore.isPending}
              onClick={() => restore.mutate()}
            >
              {t("Restore")}
            </Button>
          </div>
          <label className="mt-2 flex items-start gap-2 text-[10.5px] leading-relaxed text-muted">
            <input
              type="checkbox"
              checked={confirm}
              onChange={(event) => setConfirm(event.target.checked)}
              className="mt-0.5 size-3.5 shrink-0 accent-[var(--accent)]"
            />
            {t("I understand this replaces the current disks and domain definition.")}
          </label>
        </section>

        {job && job.kind === "restore" && job.state !== "idle" ? (
          <section className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone={stateTone(job.state)} dot>{t(job.state === "running" ? "Running" : job.state === "completed" ? "Completed" : job.state === "cancelled" ? "Cancelled" : job.state === "failed" ? "Failed" : "Cancelling")}</Badge>
              <span className="text-[10.5px] text-muted">{job.message || job.phase}</span>
              <span className="ml-auto font-mono text-[10px] text-subtle">{percent.toFixed(0)}%</span>
            </div>
            <Progress value={percent} height={5} className="mt-2.5" tone={job.state === "failed" ? "var(--danger)" : "var(--accent)"} />
            <p className="mt-2 break-all font-mono text-[9.5px] text-subtle">{job.dir}</p>
            {job.error ? <p className="mt-1.5 text-[10.5px] text-[var(--danger)]">{job.error}</p> : null}
            {job.state === "running" || job.state === "cancelling" ? (
              <Button size="sm" variant="secondary" className="mt-2" disabled={cancel.isPending} onClick={() => cancel.mutate()}>
                {t("Cancel")}
              </Button>
            ) : null}
          </section>
        ) : null}

        <p className="text-[10px] leading-relaxed text-subtle">
          {t("Backups require a stopped VM. Restoring keeps the previous disks as *.aurora-pre-restore-* files next to the originals.")}
        </p>
      </div>
    </Modal>
  );
}
