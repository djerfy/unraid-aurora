import { useEffect, useState, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import {
  Apple,
  Clock3,
  Cpu,
  Gauge,
  HardDrive,
  LayoutGrid,
  List,
  MemoryStick,
  Monitor,
  Network,
  Pause,
  Play,
  RefreshCw,
  Square,
  SquareTerminal,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { EntityIcon, vmIconURL } from "../components/ui/entity-icon";
import { Drawer } from "../components/ui/overlays";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  Progress,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useAction } from "../lib/useAction";
import { formatBytes, formatMemory, formatUptime, hexToRgba, relativeTime } from "../lib/format";
import type { VM } from "../lib/types";

export function VMsPage() {
  const { snapshot, refresh } = useApp();
  const action = useAction();
  const [params, setParams] = useSearchParams();
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
        eyebrow="Infrastructure"
        title="Virtual machines"
        description="KVM workloads with passthrough GPUs, live utilisation and direct lifecycle controls."
        meta={
          <>
            <Badge tone="success" dot pulse>{running.length} running</Badge>
            <Badge tone="neutral">{vms.length - running.length} stopped</Badge>
            <span className="chip">{totalVcpu} vCPUs allocated</span>
          </>
        }
        actions={
          <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
            Refresh
          </Button>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Metric label="Machines" value={String(vms.length)} hint={`${running.length} active now`} icon={<Monitor className="size-4" />} />
        <Metric label="vCPU allocated" value={String(totalVcpu)} hint="across all machines" icon={<Cpu className="size-4" />} />
        <Metric label="Memory allocated" value={formatMemory(totalMemory)} hint={`of ${formatMemory(snapshot.system.memory.total)}`} icon={<MemoryStick className="size-4" />} />
        <Metric label="GPU passthrough" value={String(vms.filter((vm) => vm.primaryGpu).length)} hint="dedicated devices" icon={<Gauge className="size-4" />} />
      </div>

      {vms.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Monitor className="size-5" />}
            title="No virtual machines yet"
            description="Create a VM from an ISO or a community template to get started."
          />
        </Card>
      ) : (
        <>
          <div className="mb-4 flex justify-end">
            <div className="flex items-center rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
              <button
                type="button"
                onClick={() => setView("grid")}
                aria-label="Grid view"
                className={cn("grid size-7 place-items-center rounded-md transition", view === "grid" ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink")}
              >
                <LayoutGrid className="size-3.5" />
              </button>
              <button
                type="button"
                onClick={() => setView("list")}
                aria-label="List view"
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
                  busy={action.isPending && action.variables?.target === vm.name}
                />
              ))}
            </div>
          ) : (
            <Card>
              <div className="divide-y divide-line/60">
                {vms.map((vm) => (
                  <VMRow
                    key={vm.id}
                    vm={vm}
                    onOpen={() => setSelected(vm)}
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
              <h2 className="text-[13px] font-semibold text-ink">Hypervisor</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">KVM · libvirt · {vms.length} defined machines</p>
            </div>
          </div>
          <Badge tone="success" dot>KVM active</Badge>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">vCPU overcommit</span>
              <span className="num text-soft">{totalVcpu} / {snapshot.system.cpu.threads}</span>
            </div>
            <Progress value={Math.min(100, (totalVcpu / Math.max(1, snapshot.system.cpu.threads)) * 100)} height={5} className="mt-2" />
            <p className="mt-1.5 text-[9.5px] text-subtle">threads assigned to guests</p>
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">Memory allocation</span>
              <span className="num text-soft">{formatMemory(totalMemory)} / {formatMemory(snapshot.system.memory.total)}</span>
            </div>
            <Progress value={Math.min(100, (totalMemory / Math.max(1, snapshot.system.memory.total)) * 100)} height={5} tone="var(--info)" className="mt-2" />
            <p className="mt-1.5 text-[9.5px] text-subtle">reserved for virtual machines</p>
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">Autostart</span>
              <span className="num text-soft">{vms.filter((vm) => vm.autostart).length} / {vms.length}</span>
            </div>
            <Progress value={(vms.filter((vm) => vm.autostart).length / Math.max(1, vms.length)) * 100} height={5} tone="var(--success)" className="mt-2" />
            <p className="mt-1.5 text-[9.5px] text-subtle">machines started with the array</p>
          </div>
          <div>
            <div className="flex items-center justify-between text-[10px]">
              <span className="text-muted">Passthrough</span>
              <span className="num text-soft">{vms.filter((vm) => vm.primaryGpu).length} GPUs</span>
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
        subtitle={
          selected ? (
            <span className="flex items-center gap-2">
              <Badge tone={selected.state === "running" ? "success" : selected.state === "paused" ? "warning" : "neutral"} dot>
                {selected.state}
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
                        ? `${selected.name} suspended`
                        : selected.state === "paused"
                          ? `${selected.name} resumed`
                          : `${selected.name} started`,
                  })
                }
              >
                {selected.state === "running" ? "Suspend" : selected.state === "paused" ? "Resume" : "Start"}
              </Button>
              {selected.state !== "stopped" ? (
                <Button
                  variant="danger"
                  className="flex-1"
                  onClick={() =>
                    action.mutate({ kind: "vm.stop", target: selected.name, success: `${selected.name} is shutting down` })
                  }
                >
                  Shut down
                </Button>
              ) : null}
            </div>
          ) : null
        }
      >
        {selected ? (
          <div className="space-y-5">
            <div className="grid grid-cols-2 gap-3">
              <Info label="Operating system" value={selected.os} />
              <Info label="Machine type" value={selected.machine} />
              <Info label="vCPUs" value={String(selected.vcpus)} />
              <Info label="Memory" value={formatMemory(selected.memory)} />
              <Info label="IP address" value={selected.ip || "—"} />
              <Info label="Autostart" value={selected.autostart ? "enabled" : "disabled"} />
              <Info label="Uptime" value={selected.state === "running" ? formatUptime(selected.uptime) : "—"} />
              <Info label="Last started" value={relativeTime(selected.lastStarted)} />
              <Info label="Primary GPU" value={selected.primaryGpu || "virtual"} className="col-span-2" />
            </div>

            <div>
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-muted">CPU utilisation</span>
                <span className="num text-soft">{selected.cpuUsage.toFixed(1)}%</span>
              </div>
              <Progress value={selected.cpuUsage} height={5} className="mt-2" />
            </div>
            <div>
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-muted">Memory utilisation</span>
                <span className="num text-soft">{selected.memoryUsage.toFixed(0)}%</span>
              </div>
              <Progress value={selected.memoryUsage} height={5} className="mt-2" tone="var(--info)" />
            </div>

            <div>
              <div className="flex items-center justify-between text-[11px]">
                <span className="text-muted">Virtual disk</span>
                <span className="num text-soft">
                  {formatBytes(selected.diskUsed)} / {formatBytes(selected.diskSize)}
                </span>
              </div>
              <Progress value={selected.diskSize ? (selected.diskUsed / selected.diskSize) * 100 : 0} height={5} className="mt-2" tone="var(--violet)" />
            </div>

            <div className="rounded-xl border border-line bg-[var(--surface-2)]/60 p-3 text-[10.5px] leading-relaxed text-muted">
              Virtual machines run under libvirt/KVM. Changes to hardware, boot order and passthrough devices
              require a graceful shutdown.
            </div>
          </div>
        ) : null}
      </Drawer>
    </>
  );
}

function VMCard({ vm, onOpen, busy }: { vm: VM; onOpen: () => void; busy?: boolean }) {
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
          {vm.state}
        </Badge>
      </div>

      <div className="relative mt-4 grid grid-cols-3 gap-2">
        <Spec icon={<Cpu className="size-3" />} label="vCPU" value={String(vm.vcpus)} />
        <Spec icon={<MemoryStick className="size-3" />} label="RAM" value={formatMemory(vm.memory)} />
        <Spec icon={<HardDrive className="size-3" />} label="Disk" value={formatBytes(vm.diskSize)} />
      </div>

      <div className="relative mt-4 space-y-3">
        <div>
          <div className="flex items-center justify-between text-[9.5px]">
            <span className="text-subtle">CPU</span>
            <span className="num text-soft">{running ? `${vm.cpuUsage.toFixed(1)}%` : "—"}</span>
          </div>
          <Progress value={vm.cpuUsage} height={4} className="mt-1.5" />
        </div>
        <div>
          <div className="flex items-center justify-between text-[9.5px]">
            <span className="text-subtle">Memory</span>
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
          Details
        </Button>
        <button
          type="button"
          disabled={busy}
          onClick={() =>
            action.mutate({
              kind: running ? "vm.pause" : vm.state === "paused" ? "vm.resume" : "vm.start",
              target: vm.name,
              success: running ? `${vm.name} suspended` : vm.state === "paused" ? `${vm.name} resumed` : `${vm.name} started`,
            })
          }
          title={running ? "Suspend" : vm.state === "paused" ? "Resume" : "Start"}
          className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--surface-3)] hover:text-ink disabled:opacity-40"
        >
          {running ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
        </button>
        {vm.state !== "stopped" ? (
          <button
            type="button"
            onClick={() => action.mutate({ kind: "vm.stop", target: vm.name, success: `${vm.name} is shutting down` })}
            title="Shut down"
            className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--danger-soft)] hover:text-[var(--danger)]"
          >
            <Square className="size-3.5" />
          </button>
        ) : null}
        {vm.ip ? (
          <button
            type="button"
            onClick={() => window.open(`http://${vm.ip}`, "_blank", "noopener")}
            title="Open in browser"
            className="grid size-7 place-items-center rounded-md text-muted transition hover:bg-[var(--surface-3)] hover:text-ink"
          >
            <Network className="size-3.5" />
          </button>
        ) : null}
      </div>
    </Card>
  );
}

function VMRow({ vm, onOpen, busy }: { vm: VM; onOpen: () => void; busy?: boolean }) {
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
      <Badge tone={running ? "success" : vm.state === "paused" ? "warning" : "neutral"} dot>{running ? "running" : vm.state}</Badge>
      <span className="hidden w-20 text-right num text-[10px] text-muted sm:block">{vm.vcpus} vCPU</span>
      <span className="hidden w-20 text-right num text-[10px] text-muted sm:block">{formatMemory(vm.memory)}</span>
      <span className="hidden w-16 text-right num text-[10px] text-muted md:block">{running ? `${vm.cpuUsage.toFixed(0)}%` : "—"}</span>
      <span className="hidden w-16 text-right num text-[10px] text-muted md:block">{running ? `${vm.memoryUsage.toFixed(0)}%` : "—"}</span>
      <div className="flex items-center gap-1">
        <IconButton
          label={running ? "Suspend" : vm.state === "paused" ? "Resume" : "Start"}
          size="sm"
          disabled={busy}
          onClick={() =>
            action.mutate({
              kind: running ? "vm.pause" : vm.state === "paused" ? "vm.resume" : "vm.start",
              target: vm.name,
              success: running ? `${vm.name} suspended` : vm.state === "paused" ? `${vm.name} resumed` : `${vm.name} started`,
            })
          }
        >
          {running ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
        </IconButton>
        {vm.state !== "stopped" ? (
          <IconButton
            label="Shut down"
            size="sm"
            onClick={() => action.mutate({ kind: "vm.stop", target: vm.name, success: `${vm.name} is shutting down` })}
          >
            <Square className="size-3.5" />
          </IconButton>
        ) : null}
        <IconButton label="Details" size="sm" onClick={onOpen}>
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
