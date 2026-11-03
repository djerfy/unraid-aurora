import { useState } from "react";
import { Activity, Archive, Download, FileArchive, Server, Upload, Wrench } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Button, Card } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { formatUptime } from "../lib/format";

const INCLUDED = [
  "Host identity: uname, uptime, hostname",
  "Memory, filesystems and kernel devices (df, free, lspci, lsusb)",
  "Network configuration (ip addr, ip route)",
  "Docker containers, images and networks",
  "libvirt virtual machines",
  "Unraid state files (var.ini, disks.ini, shares.ini, …)",
  "Unraid configuration files (disk.cfg, ident.cfg, share.cfg, docker.cfg)",
  "Aurora settings, image update status and mover history",
  "Installed plugin list",
  "Tails of syslog, docker.log and libvirtd.log",
];

export function ToolsPage() {
  const { snapshot } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const [backupBusy, setBackupBusy] = useState<"flash" | "config" | null>(null);
  const [importing, setImporting] = useState(false);

  if (!snapshot) return null;
  const system = snapshot.system;

  const download = async () => {
    setBusy(true);
    try {
      const blob = await api.diagnostics();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
      link.href = url;
      link.download = `aurora-diagnostics-${stamp}.zip`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      toast({
        kind: "success",
        title: t("Diagnostics downloaded"),
        description: t("{size} KB archive.", { size: Math.round(blob.size / 1024) }),
      });
    } catch (error) {
      toast({ kind: "error", title: t("Diagnostics failed"), description: (error as Error).message });
    } finally {
      setBusy(false);
    }
  };

  const saveBlob = async (response: Promise<Response>, filename: string) => {
    const res = await response;
    if (!res.ok) throw new Error(await res.text());
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    return blob.size;
  };

  const flashBackup = async () => {
    setBackupBusy("flash");
    try {
      const size = await saveBlob(api.flashBackup(), "aurora-flash-backup.zip");
      toast({ kind: "success", title: t("Flash backup downloaded"), description: t("{size} MB archive.", { size: (size / 1024 / 1024).toFixed(1) }) });
    } catch (error) {
      toast({ kind: "error", title: t("Flash backup failed"), description: (error as Error).message });
    } finally {
      setBackupBusy(null);
    }
  };

  const exportConfig = async () => {
    setBackupBusy("config");
    try {
      const size = await saveBlob(api.configExport(), "aurora-config.zip");
      toast({ kind: "success", title: t("Aurora configuration exported"), description: t("{size} KB archive.", { size: Math.round(size / 1024) }) });
    } catch (error) {
      toast({ kind: "error", title: t("Configuration export failed"), description: (error as Error).message });
    } finally {
      setBackupBusy(null);
    }
  };

  const importConfig = async (file: File) => {
    if (!window.confirm(t("Import this Aurora configuration? The current settings, layout and histories will be replaced."))) return;
    setImporting(true);
    try {
      const result = await api.configImport(file);
      toast({
        kind: "success",
        title: t("Configuration imported"),
        description: result.restartRequired ? t("Restart Aurora to apply the imported configuration.") : result.files.join(", "),
      });
    } catch (error) {
      toast({ kind: "error", title: t("Configuration import failed"), description: (error as Error).message });
    } finally {
      setImporting(false);
    }
  };

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Tools")}
        description={t("Diagnostics and host information, mirroring the maintenance tools of the official webGUI.")}
        meta={
          <>
            <Badge tone="success" dot>Unraid {system.version}</Badge>
            <span className="chip font-mono">{system.hostname}</span>
            <span className="chip">{formatUptime(system.uptime)} {t("uptime")}</span>
          </>
        }
      />

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card className="card-pad">
          <div className="flex items-start gap-3">
            <span className="grid size-10 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-[var(--violet)]">
              <FileArchive className="size-5" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[14px] font-semibold text-ink">{t("Diagnostics")}</h2>
              <p className="mt-1 text-[11px] leading-relaxed text-muted">
                {t("Build a zip archive with configuration, command output and log tails for troubleshooting, like")}{" "}
                <span className="text-soft">{t("Tools → Diagnostics")}</span> {t("in Unraid.")}
              </p>
            </div>
          </div>
          <div className="mt-5 flex items-center gap-2">
            <Button
              variant="primary"
              icon={<Download className="size-3.5" />}
              loading={busy}
              onClick={() => void download()}
            >
              {t("Download diagnostics")}
            </Button>
            <span className="text-[10px] text-subtle">{t("Generated on demand, never stored on disk.")}</span>
          </div>
          <div className="mt-5 border-t border-line pt-3">
            <p className="mb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Archive contents")}</p>
            <ul className="space-y-1">
              {INCLUDED.map((item) => (
                <li key={item} className="flex items-start gap-2 text-[10.5px] text-muted">
                  <Wrench className="mt-0.5 size-3 shrink-0 text-subtle" />
                  {t(item)}
                </li>
              ))}
            </ul>
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="card-pad">
            <div className="flex items-center gap-2">
              <Server className="size-4 text-[var(--accent)]" />
              <h2 className="text-[13px] font-semibold text-ink">{t("Host")}</h2>
            </div>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <Field label={t("Hostname")} value={system.hostname} />
              <Field label={t("Unraid")} value={system.version} />
              <Field label={t("Kernel")} value={system.kernel} />
              <Field label={t("Motherboard")} value={system.board} />
              <Field label={t("CPU")} value={`${system.cpu.brand} · ${system.cpu.cores}C/${system.cpu.threads}T`} className="col-span-2" />
              <Field label={t("Memory")} value={`${(system.memory.total / 1024 ** 3).toFixed(1)} GiB`} />
              <Field label={t("Uptime")} value={formatUptime(system.uptime)} />
              <Field label={t("License")} value={system.license || "—"} className="col-span-2" />
            </div>
          </Card>

          <Card className="card-pad">
            <div className="flex items-center gap-2">
              <Activity className="size-4 text-[var(--success)]" />
              <h2 className="text-[13px] font-semibold text-ink">{t("Snapshot")}</h2>
            </div>
            <div className="mt-4 grid grid-cols-2 gap-3">
              <Field label={t("Containers")} value={String(snapshot.containers.length)} />
              <Field label={t("Virtual machines")} value={String(snapshot.vms.length)} />
              <Field label={t("Shares")} value={String(snapshot.shares.length)} />
              <Field label={t("Disks")} value={String(snapshot.disks.length)} />
              <Field label={t("Notifications")} value={String(snapshot.notifications.length)} />
              <Field label={t("Plugins")} value={String(snapshot.plugins.length)} />
            </div>
          </Card>
        </div>
      </div>
      <Card className="card-pad mt-4">
        <div className="flex items-center gap-2">
          <Archive className="size-4 text-[var(--accent)]" />
          <h2 className="text-[13px] font-semibold text-ink">{t("Backup and restore")}</h2>
        </div>
        <p className="mt-1 text-[10.5px] leading-relaxed text-muted">
          {t("Download the whole flash drive, export Aurora configuration, or restore a previous Aurora configuration archive.")}
        </p>
        <div className="mt-4 grid gap-3 sm:grid-cols-3">
          <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 p-3">
            <p className="text-[11px] font-medium text-soft">{t("Flash drive")}</p>
            <p className="mt-1 text-[9.5px] leading-relaxed text-subtle">{t("Full /boot archive. It contains secrets such as WireGuard keys and credentials.")}</p>
            <Button size="sm" variant="secondary" className="mt-3" icon={<Download className="size-3.5" />} loading={backupBusy === "flash"} onClick={() => void flashBackup()}>
              {t("Download flash backup")}
            </Button>
          </div>
          <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 p-3">
            <p className="text-[11px] font-medium text-soft">{t("Aurora configuration")}</p>
            <p className="mt-1 text-[9.5px] leading-relaxed text-subtle">{t("Settings, storage layout, histories and audit log.")}</p>
            <Button size="sm" variant="secondary" className="mt-3" icon={<Download className="size-3.5" />} loading={backupBusy === "config"} onClick={() => void exportConfig()}>
              {t("Export configuration")}
            </Button>
          </div>
          <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 p-3">
            <p className="text-[11px] font-medium text-soft">{t("Restore configuration")}</p>
            <p className="mt-1 text-[9.5px] leading-relaxed text-subtle">{t("Import a previously exported Aurora archive. A restart is required.")}</p>
            <label className="mt-3 inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-lg border border-line bg-[var(--surface-2)] px-3 text-xs font-medium text-soft transition hover:border-line-strong hover:bg-[var(--surface-3)]">
              <Upload className="size-3.5" /> {importing ? t("Importing…") : t("Import configuration")}
              <input
                type="file"
                accept=".zip,application/zip"
                className="hidden"
                disabled={importing}
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) void importConfig(file);
                  event.target.value = "";
                }}
              />
            </label>
          </div>
        </div>
      </Card>

    </>
  );
}

function Field({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={`rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2 ${className ?? ""}`}>
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-0.5 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}
