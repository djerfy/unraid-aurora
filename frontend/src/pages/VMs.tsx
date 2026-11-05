import { useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import {
  Apple,
  Archive,
  Camera,
  Check,
  Clock3,
  Copy,
  Cpu,
  FileCode2,
  Gauge,
  HardDrive,
  LayoutGrid,
  List,
  ListOrdered,
  MemoryStick,
  Monitor,
  Network,
  Pause,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  ScrollText,
  Square,
  SquareTerminal,
  Trash2,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { EntityIcon, vmIconURL } from "../components/ui/entity-icon";
import { VMConsoleModal } from "../components/vms/VMConsoleModal";
import { VMLogsModal } from "../components/vms/VMLogsModal";
import { VMBackupModal } from "../components/vms/VMBackupModal";
import { VMAutostartModal } from "../components/vms/VMAutostartModal";
import { VMAdvancedModal } from "../components/vms/VMAdvancedModal";
import { VMUSBHotplug } from "../components/vms/VMUSBHotplug";
import { VMCloneModal, VMResizeModal } from "../components/vms/VMManagementModals";
import { Drawer, Modal } from "../components/ui/overlays";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  Progress,
  Switch,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useToast } from "../app/ToastProvider";
import { useAction } from "../lib/useAction";
import { api } from "../lib/api";
import { formatBytes, formatMemory, formatUptime, hexToRgba, relativeTime } from "../lib/format";
import { useI18n } from "../i18n";
import type { VM } from "../lib/types";

export function VMsPage() {
  const { snapshot, refresh } = useApp();
  const { t } = useI18n();
  const action = useAction();
  const [params, setParams] = useSearchParams();
  const [consoleVm, setConsoleVm] = useState<VM | null>(null);
  const [logsVm, setLogsVm] = useState<VM | null>(null);
  const [backupVm, setBackupVm] = useState<VM | null>(null);
  const [orderOpen, setOrderOpen] = useState(false);
  const [xmlVm, setXmlVm] = useState<VM | null>(null);
  const [advancedVm, setAdvancedVm] = useState<VM | null>(null);
  const [cloneVm, setCloneVm] = useState<VM | null>(null);
  const [resizeVm, setResizeVm] = useState<VM | null>(null);
  const [creating, setCreating] = useState(false);
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const autostart = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) => api.setVMAutostart(id, enabled),
    onSuccess: (_result, variables) => {
      toast({
        kind: "success",
        title: variables.enabled ? t("Autostart enabled") : t("Autostart disabled"),
      });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Autostart update failed"), description: error.message });
    },
  });
  const [view, setView] = useState<"grid" | "list">(() => {
    try {
      return localStorage.getItem("aurora.vms.view") === "list" ? "list" : "grid";
    } catch {
      return "grid";
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem("aurora.vms.view", view);
    } catch {
      // Storage can be unavailable; the view still works for this session.
    }
  }, [view]);

  if (!snapshot) return null;
  const vms = snapshot.vms;
  const running = vms.filter((vm) => vm.state === "running");

  // Keeping the selection in the URL lets the command palette (and a page
  // refresh) open a specific VM directly.
  const selected = vms.find((vm) => vm.name === params.get("vm")) ?? null;
  const setSelected = (vm: VM | null) => {
    const next = new URLSearchParams(params);
    if (vm) next.set("vm", vm.name);
    else next.delete("vm");
    setParams(next, { replace: true });
  };

  const totalVcpu = vms.reduce((sum, vm) => sum + vm.vcpus, 0);
  const totalMemory = vms.reduce((sum, vm) => sum + vm.memory, 0);

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("Virtual machines")}
        description={t(
          "KVM workloads with passthrough GPUs, live utilisation and direct lifecycle controls.",
        )}
        meta={
          <>
            <Badge tone="success" dot pulse>{t("{count} running", { count: running.length })}</Badge>
            <Badge tone="neutral">{t("{count} stopped", { count: vms.length - running.length })}</Badge>
            <span className="chip">{t("{count} vCPUs allocated", { count: totalVcpu })}</span>
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<ListOrdered className="size-3.5" />} onClick={() => setOrderOpen(true)}>
              {t("Autostart order")}
            </Button>
            <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
              {t("Refresh")}
            </Button>
            <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setCreating(true)}>
              {t("New VM")}
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Metric label={t("Machines")} value={String(vms.length)} hint={t("{count} active now", { count: running.length })} icon={<Monitor className="size-4" />} />
        <Metric label={t("vCPU allocated")} value={String(totalVcpu)} hint={t("across all machines")} icon={<Cpu className="size-4" />} />
        <Metric label={t("Memory allocated")} value={formatMemory(totalMemory)} hint={t("of {total}", { total: formatMemory(snapshot.system.memory.total) })} icon={<MemoryStick className="size-4" />} />
        <Metric label={t("GPU passthrough")} value={String(vms.filter((vm) => vm.primaryGpu).length)} hint={t("dedicated devices")} icon={<Gauge className="size-4" />} />
      </div>

      {vms.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Monitor className="size-5" />}
            title={t("No virtual machines yet")}
            description={t("Create a VM from an ISO or a community template to get started.")}
          />
        </Card>
      ) : (
        <>
          <div className="mb-4 flex justify-end">
            <div className="flex items-center rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
              <button
                type="button"
                onClick={() => setView("grid")}
                aria-label={t("Grid view")}
                className={cn("grid size-7 place-items-center rounded-md transition", view === "grid" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
              >
                <LayoutGrid className="size-3.5" />
              </button>
              <button
                type="button"
                onClick={() => setView("list")}
                aria-label={t("List view")}
                className={cn("grid size-7 place-items-center rounded-md transition", view === "list" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
              >
                <List className="size-3.5" />
              </button>
            </div>
          </div>
          {view === "grid" ? (
            <div className="stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
              {vms.map((vm) => (
                <VMCard
                  key={vm.id}
                  vm={vm}
                  onOpen={() => setSelected(vm)}
                  onConsole={() => setConsoleVm(vm)}
                  busy={action.isPending && action.variables?.target === vm.name}
                />
              ))}
            </div>
          ) : (
            <Card>
              <div className="hidden items-center gap-3 border-b border-line bg-[var(--surface-2)]/35 px-4 py-2 sm:flex">
                <span className="min-w-0 flex-1 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Machine")}</span>
                <span className="w-20 shrink-0 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("State")}</span>
                <span className="hidden w-20 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle sm:block">{t("vCPUs")}</span>
                <span className="hidden w-20 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle sm:block">{t("Memory")}</span>
                <span className="hidden w-16 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle md:block">{t("CPU")}</span>
                <span className="hidden w-16 shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle md:block">{t("RAM")}</span>
                <span className="w-[140px] shrink-0 text-right text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Actions")}</span>
              </div>
              <div className="divide-y divide-line/60">
                {vms.map((vm) => (
                  <VMRow
                    key={vm.id}
                    vm={vm}
                    onOpen={() => setSelected(vm)}
                    onConsole={() => setConsoleVm(vm)}
                    busy={action.isPending && action.variables?.target === vm.name}
                  />
                ))}
              </div>
            </Card>
          )}
        </>
      )}

      <Card className="card-pad mt-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">
              <Gauge className="size-3.5" />
            </span>
            <div>
              <h2 className="text-[13px] font-semibold text-ink">{t("Hypervisor")}</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">{t("KVM · libvirt · {count} defined machines", { count: vms.length })}</p>
            </div>
          </div>
          <Badge tone="success" dot>{t("KVM active")}</Badge>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">{t("vCPU overcommit")}</span>
              <span className="num text-soft">{totalVcpu} / {snapshot.system.cpu.threads}</span>
            </div>
            <Progress value={Math.min(100, (totalVcpu / Math.max(1, snapshot.system.cpu.threads)) * 100)} height={5} className="mt-2" />
            <p className="mt-1.5 text-[9.5px] text-subtle">{t("threads assigned to guests")}</p>
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">{t("Memory allocation")}</span>
              <span className="num text-soft">{formatMemory(totalMemory)} / {formatMemory(snapshot.system.memory.total)}</span>
            </div>
            <Progress value={Math.min(100, (totalMemory / Math.max(1, snapshot.system.memory.total)) * 100)} height={5} tone="var(--info)" className="mt-2" />
            <p className="mt-1.5 text-[9.5px] text-subtle">{t("reserved for virtual machines")}</p>
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">{t("Autostart")}</span>
              <span className="num text-soft">{vms.filter((vm) => vm.autostart).length} / {vms.length}</span>
            </div>
            <Progress value={(vms.filter((vm) => vm.autostart).length / Math.max(1, vms.length)) * 100} height={5} tone="var(--success)" className="mt-2" />
            <p className="mt-1.5 text-[9.5px] text-subtle">{t("machines started with the array")}</p>
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">{t("Passthrough")}</span>
              <span className="num text-soft">{t("{count} GPUs", { count: vms.filter((vm) => vm.primaryGpu).length })}</span>
            </div>
            <div className="mt-2 flex flex-wrap gap-1">
              {vms.filter((vm) => vm.primaryGpu).map((vm) => (
                <span key={vm.id} className="chip text-[9px]">{vm.primaryGpu}</span>
              ))}
            </div>
          </div>
        </div>
      </Card>

      <Drawer
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        title={selected?.name ?? ""}
        width="max-w-[1040px]"
        subtitle={
          selected ? (
            <span className="flex items-center gap-2">
              <Badge tone={selected.state === "running" ? "success" : selected.state === "paused" ? "warning" : "neutral"} dot>
                {t(selected.state)}
              </Badge>
              {selected.os}
            </span>
          ) : null
        }
        footer={
          selected ? (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                className="flex-1"
                icon={selected.state === "running" ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
                onClick={() =>
                  action.mutate({
                    kind: selected.state === "running" ? "vm.pause" : selected.state === "paused" ? "vm.resume" : "vm.start",
                    target: selected.name,
                    success:
                      selected.state === "running"
                        ? t("{name} suspended", { name: selected.name })
                        : selected.state === "paused"
                          ? t("{name} resumed", { name: selected.name })
                          : t("{name} started", { name: selected.name }),
                  })
                }
              >
                {selected.state === "running" ? t("Suspend") : selected.state === "paused" ? t("Resume") : t("Start")}
              </Button>
              {selected.state !== "stopped" ? (
                <Button
                  variant="danger"
                  className="flex-1"
                  onClick={() =>
                    action.mutate({ kind: "vm.stop", target: selected.name, success: t("{name} is shutting down", { name: selected.name }) })
                  }
                >
                  {t("Shut down")}
                </Button>
              ) : null}
              <Button
                variant="secondary"
                className="flex-1"
                icon={<Monitor className="size-3.5" />}
                disabled={selected.state !== "running" || !selected.consoleWebSocket}
                onClick={() => setConsoleVm(selected)}
              >
                {t("Console")}
              </Button>
            </div>
          ) : null
        }
      >
        {selected ? (
          <div className="space-y-5">
            <div className="grid grid-cols-2 gap-3">
              <Info label={t("Operating system")} value={selected.os} />
              <Info label={t("Machine type")} value={selected.machine} />
              <Info label={t("vCPUs")} value={String(selected.vcpus)} />
              <Info label={t("Memory")} value={formatMemory(selected.memory)} />
              <Info label={t("IP address")} value={selected.ip || "—"} />
              <Info label={t("Uptime")} value={selected.state === "running" ? formatUptime(selected.uptime) : "—"} />
              <Info label={t("Last started")} value={relativeTime(selected.lastStarted)} />
              <Info label={t("Primary GPU")} value={selected.primaryGpu || t("virtual")} className="col-span-2" />
            </div>

            <div className="flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
              <span>
                <span className="block text-[11px] font-medium text-soft">{t("Start with the array")}</span>
                <span className="block text-[9.5px] text-subtle">{t("libvirt autostart")}</span>
              </span>
              <div className="flex flex-wrap items-center gap-2">
                <Button size="sm" variant="secondary" icon={<FileCode2 className="size-3" />} onClick={() => setXmlVm(selected)}>
                  {t("XML")}
                </Button>
                <Button size="sm" variant="secondary" icon={<Cpu className="size-3" />} onClick={() => setAdvancedVm(selected)}>
                  {t("Hardware")}
                </Button>
                <Button size="sm" variant="secondary" icon={<Copy className="size-3" />} onClick={() => setCloneVm(selected)}>
                  {t("Clone")}
                </Button>
                <Button size="sm" variant="secondary" icon={<HardDrive className="size-3" />} onClick={() => setResizeVm(selected)}>
                  {t("Expand disk")}
                </Button>
                <Button size="sm" variant="secondary" icon={<ScrollText className="size-3" />} onClick={() => setLogsVm(selected)}>
                  {t("Logs")}
                </Button>
                <Button size="sm" variant="secondary" icon={<Archive className="size-3" />} onClick={() => setBackupVm(selected)}>
                  {t("Backup")}
                </Button>
                <Switch
                  checked={selected.autostart}
                  onChange={(enabled) => autostart.mutate({ id: selected.id, enabled })}
                  label={t("Autostart virtual machine")}
                  disabled={autostart.isPending}
                />
              </div>
            </div>

            <div>
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-muted">{t("CPU utilisation")}</span>
                <span className="num text-soft">{selected.cpuUsage.toFixed(1)}%</span>
              </div>
              <Progress value={selected.cpuUsage} height={5} className="mt-2" />
            </div>
            <div>
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-muted">{t("Memory utilisation")}</span>
                <span className="num text-soft">{selected.memoryUsage.toFixed(0)}%</span>
              </div>
              <Progress value={selected.memoryUsage} height={5} className="mt-2" tone="var(--info)" />
            </div>

            <div>
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-muted">{t("Virtual disk")}</span>
                <span className="num text-soft">
                  {formatBytes(selected.diskUsed)} / {formatBytes(selected.diskSize)}
                </span>
              </div>
              <Progress value={selected.diskSize ? (selected.diskUsed / selected.diskSize) * 100 : 0} height={5} className="mt-2" tone="var(--violet)" />
            </div>

            <VMSnapshots vm={selected} />

            <VMUSBHotplug vm={selected} />

            <div className="rounded-xl border border-line bg-[var(--surface-2)]/60 p-3 text-[10.5px] leading-relaxed text-muted">
              {t(
                "Virtual machines run under libvirt/KVM. Changes to hardware, boot order and passthrough devices require a graceful shutdown.",
              )}
            </div>
          </div>
        ) : null}
      </Drawer>

      <VMLogsModal vm={logsVm} onClose={() => setLogsVm(null)} />
      <VMBackupModal vm={backupVm} onClose={() => setBackupVm(null)} />
      <VMAutostartModal open={orderOpen} vms={vms} onClose={() => setOrderOpen(false)} />

      <VMConsoleModal
        vm={consoleVm}
        onClose={() => {
          setConsoleVm(null);
        }}
      />

      <VMXMLModal vm={xmlVm} onClose={() => setXmlVm(null)} />
      <VMAdvancedModal vm={advancedVm} onClose={() => setAdvancedVm(null)} />
      <VMCloneModal vm={cloneVm} onClose={() => setCloneVm(null)} />
      <VMResizeModal vm={resizeVm} onClose={() => setResizeVm(null)} />
      <NewVMModal open={creating} onClose={() => setCreating(false)} />
    </>
  );
}

function VMXMLModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [copied, setCopied] = useState(false);
  const [draft, setDraft] = useState("");
  const query = useQuery({
    queryKey: ["vm-xml", vm?.id],
    queryFn: () => api.vmXML(vm?.id ?? ""),
    enabled: Boolean(vm),
    staleTime: 10_000,
  });

  useEffect(() => {
    if (query.data) setDraft(query.data.xml);
  }, [query.data]);

  const save = useMutation({
    mutationFn: () => api.updateVMXML(vm?.id ?? "", draft),
    onSuccess: () => {
      toast({ kind: "success", title: t("VM definition saved"), description: t("Changes apply to the next boot.") });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["vm-xml", vm?.id] });
      onClose();
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not save the VM definition"), description: error.message });
    },
  });

  const copy = async () => {
    if (!query.data) return;
    await navigator.clipboard.writeText(draft);
    setCopied(true);
    toast({ kind: "info", title: t("XML copied") });
    window.setTimeout(() => setCopied(false), 1600);
  };

  return (
    <Modal
      open={Boolean(vm)}
      onClose={onClose}
      title={t("XML · {name}", { name: vm?.name ?? "" })}
      description={t("Edit the libvirt domain definition. The domain name cannot be changed here; changes apply on the next boot.")}
      size="xl"
      footer={
        <>
          <Button variant="secondary" icon={copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />} onClick={() => void copy()} disabled={!query.data}>
            {copied ? t("Copied") : t("Copy XML")}
          </Button>
          <Button variant="secondary" onClick={onClose}>
            {t("Close")}
          </Button>
          <Button
            variant="primary"
            icon={<FileCode2 className="size-3.5" />}
            loading={save.isPending}
            disabled={!draft.trim() || draft === query.data?.xml}
            onClick={() => save.mutate()}
          >
            {t("Save definition")}
          </Button>
        </>
      }
    >
      {query.isLoading ? (
        <p className="text-[11px] text-muted">{t("Loading domain XML…")}</p>
      ) : query.isError ? (
        <p className="text-[11px] text-[var(--danger)]">{(query.error as Error).message}</p>
      ) : (
        <textarea
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          spellCheck={false}
          aria-label={t("Libvirt XML")}
          className="h-[60vh] w-full resize-none rounded-xl border border-[#1f2230] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed text-[#c7cbd6] outline-none focus:border-[var(--accent-line)]"
        />
      )}
    </Modal>
  );
}

function VMSnapshots({ vm }: { vm: VM }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const snapshots = useQuery({
    queryKey: ["vm-snapshots", vm.id],
    queryFn: () => api.vmSnapshots(vm.id),
    refetchInterval: 10_000,
  });

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ["vm-snapshots", vm.id] });
    void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
  };

  const create = useMutation({
    mutationFn: () => api.createVMSnapshot(vm.id, name.trim(), `${vm.name} · Aurora`),
    onSuccess: () => {
      setName("");
      refresh();
      toast({ kind: "success", title: t("Snapshot created") });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not create the snapshot"), description: error.message });
    },
  });

  const restore = useMutation({
    mutationFn: (snapshot: string) => api.restoreVMSnapshot(vm.id, snapshot),
    onSuccess: () => {
      refresh();
      toast({ kind: "success", title: t("Snapshot restored") });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not restore the snapshot"), description: error.message });
    },
  });

  const remove = useMutation({
    mutationFn: (snapshot: string) => api.deleteVMSnapshot(vm.id, snapshot),
    onSuccess: () => {
      refresh();
      toast({ kind: "success", title: t("Snapshot deleted") });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not delete the snapshot"), description: error.message });
    },
  });

  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
          <Camera className="size-3.5" /> {t("Snapshots")}
        </h3>
        <IconButton label={t("Refresh snapshots")} size="sm" onClick={refresh}>
          <RefreshCw className={cn("size-3.5", snapshots.isFetching && "animate-spin")} />
        </IconButton>
      </div>
      <div className="mt-2 flex gap-2">
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder={t("Snapshot name")}
          aria-label={t("Snapshot name")}
          className="h-9 min-w-0 flex-1 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
        />
        <Button
          size="sm"
          variant="secondary"
          icon={<Plus className="size-3.5" />}
          loading={create.isPending}
          disabled={!name.trim()}
          onClick={() => create.mutate()}
        >
          {t("Create")}
        </Button>
      </div>
      {snapshots.isLoading ? (
        <p className="mt-2 text-[10px] text-muted">{t("Loading snapshots…")}</p>
      ) : snapshots.isError ? (
        <p className="mt-2 text-[10px] text-[var(--danger)]">{(snapshots.error as Error).message}</p>
      ) : snapshots.data?.length ? (
        <div className="mt-2 overflow-hidden rounded-xl border border-line">
          {snapshots.data.map((snapshot) => (
            <div key={snapshot.name} className="flex items-center gap-2 border-b border-line/60 px-3 py-2 last:border-0">
              <span className="min-w-0 flex-1 truncate font-mono text-[10.5px] text-soft">{snapshot.name}</span>
              <IconButton
                label={t("Restore snapshot")}
                size="sm"
                disabled={restore.isPending}
                onClick={() => {
                  if (window.confirm(t("Restore snapshot {name}? The current VM state will be replaced.", { name: snapshot.name }))) {
                    restore.mutate(snapshot.name);
                  }
                }}
              >
                <RotateCcw className="size-3.5" />
              </IconButton>
              <IconButton
                label={t("Delete snapshot")}
                size="sm"
                disabled={remove.isPending}
                onClick={() => {
                  if (window.confirm(t("Delete snapshot {name}?", { name: snapshot.name }))) {
                    remove.mutate(snapshot.name);
                  }
                }}
              >
                <Trash2 className="size-3.5" />
              </IconButton>
            </div>
          ))}
        </div>
      ) : (
        <p className="mt-2 text-[10px] text-subtle">{t("No snapshots for this VM.")}</p>
      )}
    </div>
  );
}

function NewVMModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState({
    name: "",
    vcpus: 2,
    memoryMB: 2048,
    diskGB: 32,
    isoPath: "",
    network: "br0",
    start: true,
  });

  useEffect(() => {
    if (open) {
      setDraft({ name: "", vcpus: 2, memoryMB: 2048, diskGB: 32, isoPath: "", network: "br0", start: true });
    }
  }, [open]);

  const create = useMutation({
    mutationFn: () => api.createVM(draft),
    onSuccess: (vm) => {
      toast({ kind: "success", title: t("Virtual machine created"), description: vm.name });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      onClose();
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not create the virtual machine"), description: error.message });
    },
  });

  const field = "h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-ink outline-none focus:border-[var(--accent-line)]";
  const label = "mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle";

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("New virtual machine")}
      description={t("Create a libvirt/KVM domain with a qcow2 virtual disk and an optional installation ISO.")}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>{t("Cancel")}</Button>
          <Button
            variant="primary"
            icon={<Plus className="size-3.5" />}
            loading={create.isPending}
            disabled={!draft.name.trim()}
            onClick={() => create.mutate()}
          >
            {draft.start ? t("Create and start") : t("Create")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <label className="block">
          <span className={label}>{t("Name")}</span>
          <input
            value={draft.name}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })}
            placeholder="Windows 11"
            className={field}
            autoFocus
          />
          <span className="mt-1 block text-[9.5px] text-subtle">{t("The virtual disk will be created under /mnt/user/domains.")}</span>
        </label>
        <div className="grid gap-3 sm:grid-cols-3">
          <label className="block">
            <span className={label}>{t("vCPUs")}</span>
            <input
              type="number"
              min={1}
              max={128}
              value={draft.vcpus}
              onChange={(event) => setDraft({ ...draft, vcpus: Number(event.target.value) })}
              className={field}
            />
          </label>
          <label className="block">
            <span className={label}>{t("Memory (MiB)")}</span>
            <input
              type="number"
              min={512}
              step={512}
              value={draft.memoryMB}
              onChange={(event) => setDraft({ ...draft, memoryMB: Number(event.target.value) })}
              className={field}
            />
          </label>
          <label className="block">
            <span className={label}>{t("Disk (GiB)")}</span>
            <input
              type="number"
              min={1}
              max={10000}
              value={draft.diskGB}
              onChange={(event) => setDraft({ ...draft, diskGB: Number(event.target.value) })}
              className={field}
            />
          </label>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className={label}>{t("Network bridge")}</span>
            <input
              value={draft.network}
              onChange={(event) => setDraft({ ...draft, network: event.target.value })}
              placeholder="br0"
              className={field}
            />
          </label>
          <label className="block">
            <span className={label}>{t("Installation ISO")}</span>
            <input
              value={draft.isoPath}
              onChange={(event) => setDraft({ ...draft, isoPath: event.target.value })}
              placeholder="/mnt/user/isos/windows.iso"
              className={`${field} font-mono text-[10.5px]`}
            />
          </label>
        </div>
        <label className="flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
          <span>
            <span className="block text-[11px] font-medium text-soft">{t("Start after creation")}</span>
            <span className="block text-[9.5px] text-subtle">{t("Boot the VM as soon as it is defined.")}</span>
          </span>
          <Switch checked={draft.start} onChange={(start) => setDraft({ ...draft, start })} label={t("Start after creation")} />
        </label>
      </div>
    </Modal>
  );
}

function VMCard({ vm, onOpen, onConsole, busy }: { vm: VM; onOpen: () => void; onConsole: () => void; busy?: boolean }) {
  const { t } = useI18n();
  const action = useAction();
  const running = vm.state === "running";
  const Icon = vm.os.toLowerCase().includes("windows") ? Monitor : vm.os.toLowerCase().includes("mac") ? Apple : SquareTerminal;

  return (
    <Card hover className="group relative overflow-hidden card-pad">
      <div
        className="pointer-events-none absolute -right-16 -top-20 size-48 rounded-full blur-3xl"
        style={{ background: hexToRgba(vm.color, 0.14) }}
      />
      <div className="relative flex items-start justify-between gap-3">
        <button type="button" onClick={onOpen} className="flex min-w-0 items-center gap-3 text-left">
          <EntityIcon
            src={vmIconURL(vm.id)}
            name={vm.name}
            size={44}
            fallback={<Icon className="size-5" />}
          />
          <span className="min-w-0">
            <span className="block truncate text-[13px] font-semibold text-ink transition group-hover:text-[var(--accent)]">
              {vm.name}
            </span>
            <span className="mt-0.5 block truncate text-[10px] text-subtle">{vm.os}</span>
          </span>
        </button>
        <Badge tone={running ? "success" : vm.state === "paused" ? "warning" : "neutral"} dot pulse={running}>
          {t(vm.state)}
        </Badge>
      </div>

      <div className="relative mt-4 grid grid-cols-3 gap-2">
        <Spec icon={<Cpu className="size-3" />} label={t("vCPU")} value={String(vm.vcpus)} />
        <Spec icon={<MemoryStick className="size-3" />} label={t("RAM")} value={formatMemory(vm.memory)} />
        <Spec icon={<HardDrive className="size-3" />} label={t("Disk")} value={formatBytes(vm.diskSize)} />
      </div>

      <div className="relative mt-4 space-y-3">
        <div>
          <div className="flex items-center justify-between text-[9.5px]">
            <span className="text-subtle">{t("CPU")}</span>
            <span className="num text-soft">{running ? `${vm.cpuUsage.toFixed(1)}%` : "—"}</span>
          </div>
          <Progress value={vm.cpuUsage} height={4} className="mt-1.5" />
        </div>
        <div>
          <div className="flex items-center justify-between text-[9.5px]">
            <span className="text-subtle">{t("Memory")}</span>
            <span className="num text-soft">{running ? `${vm.memoryUsage.toFixed(0)}%` : "—"}</span>
          </div>
          <Progress value={vm.memoryUsage} height={4} className="mt-1.5" tone="var(--info)" />
        </div>
      </div>

      <div className="relative mt-3 flex flex-wrap gap-1.5">
        {vm.ip ? <span className="chip font-mono text-[9.5px]">{vm.ip}</span> : null}
        {vm.primaryGpu ? <span className="chip text-[9.5px]">{vm.primaryGpu}</span> : null}
        {vm.state === "running" ? <span className="chip text-[9.5px]"><Clock3 className="size-2.5" /> {formatUptime(vm.uptime)}</span> : null}
      </div>

      <div className="relative mt-4 flex items-center gap-1 border-t border-line pt-3">
        <Button size="sm" variant="ghost" onClick={onOpen} className="mr-auto px-2 text-[11px]">
          {t("Details")}
        </Button>
        <button
          type="button"
          disabled={!running || !vm.consoleWebSocket}
          onClick={onConsole}
          title={running ? t("Open console") : t("Start the VM to open its console")}
          className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--surface-3)] hover:text-ink disabled:opacity-30"
        >
          <Monitor className="size-3.5" />
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() =>
            action.mutate({
              kind: running ? "vm.pause" : vm.state === "paused" ? "vm.resume" : "vm.start",
              target: vm.name,
              success: running
                ? t("{name} suspended", { name: vm.name })
                : vm.state === "paused"
                  ? t("{name} resumed", { name: vm.name })
                  : t("{name} started", { name: vm.name }),
            })
          }
          title={running ? t("Suspend") : vm.state === "paused" ? t("Resume") : t("Start")}
          className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--surface-3)] hover:text-ink disabled:opacity-40"
        >
          {running ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
        </button>
        {vm.state !== "stopped" ? (
          <button
            type="button"
            onClick={() => action.mutate({ kind: "vm.stop", target: vm.name, success: t("{name} is shutting down", { name: vm.name }) })}
            title={t("Shut down")}
            className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--danger-soft)] hover:text-[var(--danger)]"
          >
            <Square className="size-3.5" />
          </button>
        ) : (
          <span className="size-7" aria-hidden="true" />
        )}
        {vm.ip ? (
          <button
            type="button"
            onClick={() => window.open(`http://${vm.ip}`, "_blank", "noopener")}
            title={t("Open in browser")}
            className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--surface-3)] hover:text-ink"
          >
            <Network className="size-3.5" />
          </button>
        ) : (
          <span className="size-7" aria-hidden="true" />
        )}
      </div>
    </Card>
  );
}

function VMRow({ vm, onOpen, onConsole, busy }: { vm: VM; onOpen: () => void; onConsole: () => void; busy?: boolean }) {
  const { t } = useI18n();
  const action = useAction();
  const running = vm.state === "running";
  const Icon = vm.os.toLowerCase().includes("windows") ? Monitor : vm.os.toLowerCase().includes("mac") ? Apple : SquareTerminal;
  return (
    <div className="flex flex-wrap items-center gap-3 px-4 py-3 transition hover:bg-[var(--surface-2)]/50">
      <button type="button" onClick={onOpen} className="flex min-w-0 flex-1 items-center gap-3 text-left">
        <EntityIcon src={vmIconURL(vm.id)} name={vm.name} size={32} fallback={<Icon className="size-4" />} />
        <span className="min-w-0">
          <span className="block truncate text-[12px] font-medium text-ink">{vm.name}</span>
          <span className="block truncate text-[9.5px] text-subtle">{vm.os}</span>
        </span>
      </button>
      <span className="w-20 shrink-0">
        <Badge tone={running ? "success" : vm.state === "paused" ? "warning" : "neutral"} dot>{t(vm.state)}</Badge>
      </span>
      <span className="hidden w-20 text-right num text-[10px] text-muted sm:block">{t("{count} vCPU", { count: vm.vcpus })}</span>
      <span className="hidden w-20 text-right num text-[10px] text-muted sm:block">{formatMemory(vm.memory)}</span>
      <span className="hidden w-16 text-right num text-[10px] text-muted md:block">{running ? `${vm.cpuUsage.toFixed(0)}%` : "—"}</span>
      <span className="hidden w-16 text-right num text-[10px] text-muted md:block">{running ? `${vm.memoryUsage.toFixed(0)}%` : "—"}</span>
      <div className="flex w-[140px] shrink-0 items-center justify-end gap-1">
        <IconButton
          label={running ? t("Open console") : t("Start the VM to open its console")}
          size="sm"
          disabled={!running || !vm.consoleWebSocket}
          onClick={onConsole}
        >
          <Monitor className="size-3.5" />
        </IconButton>
        <IconButton
          label={running ? t("Suspend") : vm.state === "paused" ? t("Resume") : t("Start")}
          size="sm"
          disabled={busy}
          onClick={() =>
            action.mutate({
              kind: running ? "vm.pause" : vm.state === "paused" ? "vm.resume" : "vm.start",
              target: vm.name,
              success: running
                ? t("{name} suspended", { name: vm.name })
                : vm.state === "paused"
                  ? t("{name} resumed", { name: vm.name })
                  : t("{name} started", { name: vm.name }),
            })
          }
        >
          {running ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
        </IconButton>
        {vm.state !== "stopped" ? (
          <IconButton
            label={t("Shut down")}
            size="sm"
            onClick={() => action.mutate({ kind: "vm.stop", target: vm.name, success: t("{name} is shutting down", { name: vm.name }) })}
          >
            <Square className="size-3.5" />
          </IconButton>
        ) : (
          <span className="size-8" aria-hidden="true" />
        )}
        <IconButton label={t("Details")} size="sm" onClick={onOpen}>
          <Monitor className="size-3.5" />
        </IconButton>
      </div>
    </div>
  );
}

function Metric({ label, value, hint, icon }: { label: string; value: string; hint: string; icon: ReactNode }) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className="num mt-3 text-[21px] font-semibold leading-none text-ink">{value}</p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}

function Spec({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-2 py-1.5">
      <p className="flex items-center gap-1 text-[8.5px] uppercase tracking-[0.08em] text-subtle">
        {icon}
        {label}
      </p>
      <p className="num mt-0.5 truncate text-[10.5px] text-soft">{value}</p>
    </div>
  );
}

function Info({ label, value, className }: { label: string; value: string; className?: string }) {
  return (
    <div className={cn("rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2", className)}>
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-1 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}
