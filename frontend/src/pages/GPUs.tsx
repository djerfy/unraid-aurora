import type { ReactNode } from "react";
import {
  Cpu,
  Fan,
  MemoryStick,
  Microchip,
  Thermometer,
  Zap,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Ring } from "../components/ui/charts";
import { Badge, Card, EmptyState, Progress } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { formatBytes, formatPercent } from "../lib/format";

export function GPUsPage() {
  const { snapshot } = useApp();
  if (!snapshot) return null;
  const gpus = snapshot.gpus;

  const totalVram = gpus.reduce((sum, gpu) => sum + gpu.memoryTotal, 0);
  const usedVram = gpus.reduce((sum, gpu) => sum + gpu.memoryUsed, 0);
  const totalPower = gpus.reduce((sum, gpu) => sum + gpu.power, 0);
  const avgUtil = gpus.length
    ? gpus.reduce((sum, gpu) => sum + gpu.utilization, 0) / gpus.length
    : 0;
  const processes = gpus.reduce((sum, gpu) => sum + gpu.processes.length, 0);

  return (
    <>
      <PageHeader
        eyebrow="Infrastructure"
        title="GPUs"
        description="Live utilisation, memory, temperature and power for the accelerators passed through to this server."
        meta={
          <>
            <Badge tone={gpus.length ? "success" : "neutral"} dot>{gpus.length} detected</Badge>
            {processes > 0 ? <span className="chip">{processes} compute processes</span> : null}
            {gpus.length ? <span className="chip">{formatPercent(avgUtil, 0)} average load</span> : null}
          </>
        }
      />

      {gpus.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Microchip className="size-5" />}
            title="No NVIDIA telemetry available"
            description="Aurora reads GPU data through nvidia-smi. Install the Unraid Nvidia Driver plugin and expose the device to the host to see utilisation, VRAM, temperatures and power."
          />
        </Card>
      ) : (
        <>
          <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
            <Stat icon={<Microchip className="size-4" />} label="Devices" value={String(gpus.length)} hint={`${gpus.filter((gpu) => gpu.vendor).length} NVIDIA`} />
            <Stat icon={<MemoryStick className="size-4" />} label="VRAM" value={formatBytes(usedVram)} hint={`of ${formatBytes(totalVram)} allocated`} />
            <Stat icon={<Cpu className="size-4" />} label="Average load" value={formatPercent(avgUtil, 0)} hint="across all GPUs" />
            <Stat icon={<Zap className="size-4" />} label="Power draw" value={`${totalPower.toFixed(0)} W`} hint="combined instant draw" />
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
            {gpus.map((gpu) => {
              const vram = gpu.memoryTotal ? (gpu.memoryUsed / gpu.memoryTotal) * 100 : 0;
              return (
                <Card key={gpu.uuid || gpu.index} className="card-pad">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-3">
                      <span className="grid size-10 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)]">
                        <VendorLogo vendor={gpu.vendor} />
                      </span>
                      <div className="min-w-0">
                        <h2 className="truncate text-[14px] font-semibold text-ink">{gpu.name}</h2>
                        <p className="mt-0.5 truncate font-mono text-[9.5px] text-subtle">
                          GPU {gpu.index} · driver {gpu.driver || "—"} · {gpu.uuid}
                        </p>
                      </div>
                    </div>
                    <Badge tone={gpu.vendor === "NVIDIA" ? "success" : "neutral"} dot>{gpu.vendor || "GPU"}</Badge>
                  </div>

                  <div className="mt-5 grid gap-5 sm:grid-cols-[auto_1fr]">
                    <Ring value={gpu.utilization} size={126} stroke={8}>
                      <div className="text-center">
                        <p className="num text-[24px] font-semibold leading-none text-ink">{Math.round(gpu.utilization)}%</p>
                        <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">utilisation</p>
                      </div>
                    </Ring>
                    <div className="space-y-3">
                      <div>
                        <div className="flex items-center justify-between text-[10px]">
                          <span className="text-muted">VRAM</span>
                          <span className="num text-soft">{formatBytes(gpu.memoryUsed)} / {formatBytes(gpu.memoryTotal)}</span>
                        </div>
                        <Progress value={vram} height={5} tone="var(--violet)" className="mt-1.5" />
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <Metric label="Temperature" value={`${Math.round(gpu.temperature)}°C`} icon={<Thermometer className="size-3" />} />
                        <Metric label="Power" value={`${gpu.power.toFixed(0)} W`} icon={<Zap className="size-3" />} />
                        <Metric label="Fan" value={gpu.fan ? `${gpu.fan.toFixed(0)}%` : "—"} icon={<Fan className="size-3" />} />
                        <Metric label="Clock" value={gpu.clockMhz ? `${gpu.clockMhz} MHz` : "—"} icon={<Cpu className="size-3" />} />
                      </div>
                      <p className="font-mono text-[9px] text-subtle">Power limit {gpu.powerLimit ? `${gpu.powerLimit.toFixed(0)} W` : "—"} · memory clock {gpu.memoryClockMhz ? `${gpu.memoryClockMhz} MHz` : "—"}</p>
                    </div>
                  </div>

                  <div className="mt-4 border-t border-line pt-3">
                    <p className="mb-2 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">Compute processes</p>
                    {gpu.processes.length === 0 ? (
                      <p className="text-[10.5px] text-muted">No active compute processes.</p>
                    ) : (
                      <div className="space-y-1.5">
                        {gpu.processes.map((process) => (
                          <div key={`${process.pid}-${process.name}`} className="flex items-center justify-between gap-3 text-[10.5px]">
                            <span className="min-w-0 truncate text-soft">{process.name || "process"}</span>
                            <span className="shrink-0 font-mono text-subtle">PID {process.pid} · {formatBytes(process.memory)}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </Card>
              );
            })}
          </div>
        </>
      )}
    </>
  );
}

function VendorLogo({ vendor }: { vendor: string }) {
  const src = vendor === "NVIDIA" ? "/brands/nvidia.svg" : vendor === "Radeon" || vendor === "AMD" ? "/brands/amd.svg" : "";
  if (!src) return <Microchip className="size-5 text-[var(--violet)]" />;
  return <img src={src} alt={vendor} className="h-5 w-auto max-w-7 object-contain" />;
}

function Stat({ icon, label, value, hint }: { icon: ReactNode; label: string; value: string; hint: string }) {
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

function Metric({ label, value, icon }: { label: string; value: string; icon: ReactNode }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-2.5 py-2">
      <p className="flex items-center gap-1 text-[8.5px] uppercase tracking-[0.08em] text-subtle">
        {icon}
        {label}
      </p>
      <p className="num mt-0.5 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}

