import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowDownToLine,
  ArrowRight,
  ArrowUpFromLine,
  BatteryCharging,
  Boxes,
  CheckCircle2,
  ChevronRight,
  Clock3,
  Cpu,
  FolderTree,
  Fan,
  HardDrive,
  MemoryStick,
  Microchip,
  Network as NetworkIcon,
  RefreshCw,
  ShieldCheck,
  Thermometer,
  Zap,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { EntityIcon, containerIconURL } from "../components/ui/entity-icon";
import { AreaChart, CoreMatrix, Ring, SegmentBar } from "../components/ui/charts";
import {
  Badge,
  Button,
  Card,
  Progress,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { useNetworkInterface } from "../lib/use-network-interface";
import {
  formatBytes,
  formatMemory,
  formatSpeed,
  formatUptime,
  greeting,
  relativeTime,
} from "../lib/format";
import type { Disk } from "../lib/types";

export function DashboardPage() {
  const { snapshot, metrics, history, live, refresh } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const navigate = useNavigate();
  const [range, setRange] = useState<"1h" | "24h" | "7d" | "30d">("1h");
  const networkInterface = useNetworkInterface(snapshot?.network.interfaces ?? []);
  const longHistory = useQuery({
    queryKey: ["metrics-history", range],
    queryFn: () => api.metricsHistory(range),
    enabled: range !== "1h",
    refetchInterval: 30_000,
  });

  if (!snapshot) return null;
  const { system, array, disks, containers, shares, network, ups, notifications, pools, gpus } = snapshot;

  const runningContainers = containers.filter((c) => c.state === "running");
  const updates = containers.filter((c) => c.updateAvailable).length;
  const unread = notifications.filter((n) => !n.read).length;
  const points = history.slice(-240);
  const chartPoints = range === "1h" ? points : (longHistory.data?.length ? longHistory.data : points);
  const cpuHistory = chartPoints.map((point) => point.cpu);
  const selectedInterface = networkInterface.selected;
  const interfaceName = selectedInterface?.name ?? "";
  const hasInterfaceHistory = points.some((point) => Boolean(point.interfaces));
  const netRx = points.map((point) =>
    hasInterfaceHistory ? point.interfaces?.[interfaceName]?.rx ?? 0 : point.netRx,
  );
  const netTx = points.map((point) =>
    hasInterfaceHistory ? point.interfaces?.[interfaceName]?.tx ?? 0 : point.netTx,
  );
  const netTimestamps = points.map((point) => point.t);
  const timestamps = chartPoints.map((point) => point.t);
  const interfaceRate = selectedInterface ? metrics?.interfaces?.[selectedInterface.name] : undefined;
  const downRate = interfaceRate?.rx ?? selectedInterface?.rxBps ?? 0;
  const upRate = interfaceRate?.tx ?? selectedInterface?.txBps ?? 0;
  const cpu = metrics?.cpu ?? system.cpu.usage;
  const memory = metrics?.memory ?? system.memory.usage;
  const packagePower = metrics?.power || system.cpu.power;
  const gpu = gpus[0] ?? null;
  const gpuCount = gpus.length;
  const gpuVram = gpu && gpu.memoryTotal > 0 ? (gpu.memoryUsed / gpu.memoryTotal) * 100 : 0;

  const primaryShare = [...shares].sort((a, b) => b.used - a.used).slice(0, 4);
  const issueCount = system.health.issues.length;
  const arrayDisks = disks
    .filter((disk) => disk.type === "Data" || disk.type === "Parity")
    .sort((a, b) => {
      if (a.type !== b.type) return a.type === "Parity" ? -1 : 1;
      return a.position - b.position;
    });

  return (
    <>
      <PageHeader
        eyebrow={t("Overview")}
        title={`${t(greeting())}.`}
        description={`${system.name} ${t("is online.")} ${
          issueCount > 0
            ? t(
                issueCount > 1
                  ? "{count} items deserve a quick look — nothing critical."
                  : "{count} item deserves a quick look — nothing critical.",
                { count: issueCount },
              )
            : t("Every subsystem is healthy and the array is protected.")
        }`}
        meta={
          <>
            <Badge tone={live ? "success" : "neutral"} dot pulse={live}>
              {live ? t("Live telemetry") : t("Connecting")}
            </Badge>
            <span className="chip">
              <Clock3 className="size-3" /> {formatUptime(system.uptime)} {t("uptime")}
            </span>
          </>
        }
        actions={
          <>
            <Button
              variant="secondary"
              icon={<RefreshCw className="size-3.5" />}
              onClick={() => {
                refresh();
                toast({ kind: "info", title: t("Refreshing server data") });
              }}
            >
              {t("Sync")}
            </Button>
            <Button
              variant="primary"
              icon={<HardDrive className="size-3.5" />}
              onClick={() => navigate("/storage")}
            >
              {t("Manage array")}
            </Button>
          </>
        }
      />

      {issueCount > 0 ? (
        <div className="mb-4 flex flex-col gap-2 rounded-2xl border border-[color-mix(in_srgb,var(--warning)_22%,transparent)] bg-[var(--warning-soft)] px-4 py-3 sm:flex-row sm:items-center">
          <AlertTriangle className="size-4 shrink-0 text-[var(--warning)]" />
          <div className="min-w-0 flex-1 text-[12px]">
            <span className="font-semibold text-ink">
              {t(
                issueCount > 1 ? "{count} items need attention." : "{count} item needs attention.",
                { count: issueCount },
              )}
            </span>{" "}
            <span className="text-muted">{system.health.issues.join(" · ")}</span>
          </div>
          <button
            type="button"
            onClick={() => navigate("/notifications")}
            className="inline-flex shrink-0 items-center gap-1 text-[11px] font-semibold text-[var(--warning)] transition hover:opacity-80"
          >
            {t("Review")} <ArrowRight className="size-3" />
          </button>
        </div>
      ) : null}

      <div className="stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-12">
        {/* Health ---------------------------------------------------------- */}
        <Card className="card-pad md:col-span-2 xl:col-span-3">
          <CardHeader
            title={t("System health")}
            icon={<ShieldCheck className="size-4" />}
            trailing={<Badge tone={system.health.status === "healthy" ? "success" : "warning"}>{system.health.status === "healthy" ? t("Nominal") : t("Attention")}</Badge>}
          />
          <div className="mt-4 flex items-start gap-5">
            <Ring value={system.health.score} size={118} stroke={8}>
              <div className="text-center">
                <p className="num text-[27px] font-semibold leading-none text-ink">{system.health.score}</p>
                <p className="mt-1 text-[9px] font-medium uppercase tracking-[0.14em] text-subtle">Aurora</p>
              </div>
            </Ring>
            <div className="min-w-0 flex-1 pt-1">
              <p className="text-[13px] font-semibold text-ink">
                {system.health.status === "healthy" ? t("Everything is nominal") : t("Operational, with notes")}
              </p>
              <p className="mt-1 text-[11.5px] leading-relaxed text-muted">
                {system.board}
              </p>
              <div className="mt-3 space-y-1.5">
                {system.health.issues.slice(0, 3).map((issue) => (
                  <div key={issue} className="flex items-start gap-2">
                    <span className="mt-[5px] size-1.5 shrink-0 rounded-full bg-[var(--warning)]" />
                    <p className="text-[11px] leading-snug text-soft">{issue}</p>
                  </div>
                ))}
                {system.health.issues.length === 0 ? (
                  <div className="flex items-center gap-2 text-[11px] text-[var(--success)]">
                    <CheckCircle2 className="size-3.5" /> {t("No issues detected")}
                  </div>
                ) : null}
              </div>
            </div>
          </div>
          <div className="mt-5 grid grid-cols-3 divide-x divide-line border-t border-line pt-3.5">
            <MiniStat label={t("Uptime")} value={formatUptime(system.uptime)} />
            <MiniStat label={t("Version")} value={system.version} />
            <MiniStat label={t("Package")} value={`${Math.round(system.cpu.temp)}°C`} tone={system.cpu.temp > 75 ? "warning" : undefined} />
          </div>
        </Card>

        {/* CPU ------------------------------------------------------------- */}
        <Card className="card-pad md:col-span-2 xl:col-span-4">
          <CardHeader
            title={t("Processor")}
            icon={<Cpu className="size-4" />}
            trailing={<span className="chip">{system.cpu.cores}C / {system.cpu.threads}T</span>}
          />
          <div className="mt-3 flex items-end justify-between gap-4">
            <div className="flex items-end gap-2">
              <span className="num text-[38px] font-semibold leading-none tracking-[-0.05em] text-ink">
                {metric(cpu)}
              </span>
              <span className="mb-1 text-sm text-muted">%</span>
              <span className="mb-1.5 ml-1 flex items-center gap-1 text-[11px] text-muted">
                <span className="status-dot live" /> {t("load")}
              </span>
            </div>
            <div className="text-right">
              <p className="text-[11px] text-muted">{system.cpu.brand.replace(" 16-Core Processor", "")}</p>
              <p className="num mt-0.5 text-[10.5px] text-subtle">
                {system.cpu.freqGhz.toFixed(2)} GHz · {(system.load[0] ?? 0).toFixed(2)} {t("load")}
              </p>
            </div>
          </div>
          <div className="mt-2 flex items-center justify-end gap-1">
            {(["1h", "24h", "7d", "30d"] as const).map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => setRange(value)}
                className={cn(
                  "rounded-md px-2 py-0.5 text-[8.5px] font-semibold uppercase tracking-[0.08em] transition",
                  range === value ? "bg-[var(--surface-3)] text-ink" : "text-subtle hover:text-ink",
                )}
              >
                {value}
              </button>
            ))}
          </div>
          <div className="-mx-1 mt-1">
            <AreaChart
              series={[{ values: cpuHistory, color: "var(--accent)", label: "CPU" }]}
              timestamps={timestamps}
              height={78}
              domain={[0, 100]}
              formatValue={(value) => `${value.toFixed(1)}%`}
            />
          </div>
          <div className="mt-2">
            <CoreMatrix values={metrics?.cores ?? system.cpu.perCore} />
          </div>
          <div className="mt-3 grid grid-cols-3 divide-x divide-line border-t border-line pt-3">
            <MiniStat label={t("Temperature")} value={`${Math.round(system.cpu.temp)}°C`} icon={<Thermometer className="size-3" />} />
            <MiniStat label={t("CPU power")} value={`${Math.round(packagePower)} W`} icon={<Zap className="size-3" />} />
            <MiniStat label={t("Load 1m")} value={(system.load[0] ?? 0).toFixed(2)} />
          </div>
        </Card>

        {/* Memory ---------------------------------------------------------- */}
        <Card className="card-pad md:col-span-1 xl:col-span-2">
          <CardHeader
            title={t("Memory")}
            icon={<MemoryStick className="size-4" />}
            trailing={<span className="chip">{formatMemory(system.memory.total)}</span>}
          />
          <div className="mt-4 flex justify-center">
            <Ring value={memory} size={88} stroke={6}>
              <div className="text-center">
                <p className="num text-[23px] font-semibold leading-none text-ink">{Math.round(memory)}%</p>
                <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">{t("used")}</p>
              </div>
            </Ring>
          </div>
          <div className="mt-4 space-y-2">
            <UsageLegend color="var(--accent)" label={t("Applications")} value={formatMemory(system.memory.used)} />
            <UsageLegend color="var(--info)" label={t("Cache")} value={formatMemory(system.memory.cached)} />
            <UsageLegend color="var(--surface-3)" label={t("Free")} value={formatMemory(Math.max(0, system.memory.free))} />
          </div>
          <p className="mt-3 border-t border-line pt-2.5 text-[9.5px] text-subtle">
            {system.memory.type} · {system.memory.speed}
          </p>
        </Card>

        {/* GPU -------------------------------------------------------------- */}
        <Card className="card-pad md:col-span-1 xl:col-span-3">
          <CardHeader
            title={t("GPU")}
            icon={<Microchip className="size-4" />}
            action={
              <button type="button" onClick={() => navigate("/gpus")} className="text-[10.5px] font-medium text-muted transition hover:text-ink">
                {t("Details")}
              </button>
            }
          />
          {gpu ? (
            <>
              <div className="mt-4 flex items-center gap-3">
                <Ring value={gpu.utilization} size={78} stroke={6}>
                  <div className="text-center">
                    <p className="num text-[18px] font-semibold leading-none text-ink">{Math.round(gpu.utilization)}%</p>
                    <p className="mt-1 text-[8px] uppercase tracking-[0.1em] text-subtle">{t("utilisation")}</p>
                  </div>
                </Ring>
                <div className="min-w-0 flex-1 space-y-1.5">
                  <p className="truncate text-[11px] font-semibold text-ink" title={gpu.name}>{gpu.name}</p>
                  <div className="flex items-center justify-between gap-1 text-[9px]">
                    <span className="text-muted">{t("VRAM")}</span>
                    <span className="num text-soft">{formatBytes(gpu.memoryUsed)} / {formatBytes(gpu.memoryTotal)}</span>
                  </div>
                  <Progress value={gpuVram} height={4} tone="var(--violet)" />
                  <div className="grid grid-cols-2 gap-x-2 gap-y-0.5 text-[9px] text-subtle">
                    <span className="flex items-center gap-1">
                      <Thermometer className="size-2.5" /> {gpu.temperature ? `${Math.round(gpu.temperature)}°C` : "—"}
                    </span>
                    <span className="flex items-center gap-1">
                      <Zap className="size-2.5" /> {gpu.power.toFixed(0)} W
                    </span>
                    <span className="flex items-center gap-1">
                      <Fan className="size-2.5" /> {gpu.fan ? `${Math.round(gpu.fan)}%` : "—"}
                    </span>
                    <span className="flex items-center gap-1">
                      <Cpu className="size-2.5" /> {gpu.clockMhz ? `${gpu.clockMhz} MHz` : "—"}
                    </span>
                  </div>
                </div>
              </div>
              <p className="mt-2 truncate border-t border-line pt-2 text-[9px] text-subtle">
                {t("Power limit {limit} · memory clock {clock}", {
                  limit: gpu.powerLimit ? `${gpu.powerLimit.toFixed(0)} W` : "—",
                  clock: gpu.memoryClockMhz ? `${gpu.memoryClockMhz} MHz` : "—",
                })}
              </p>
              {gpuCount > 1 ? (
                <p className="mt-1 text-[9px] text-subtle">{t("+{count} more GPU", { count: gpuCount - 1 })}</p>
              ) : null}
            </>
          ) : (
            <p className="py-10 text-center text-[10px] text-muted">{t("No GPU detected")}</p>
          )}
        </Card>

        {/* Array ------------------------------------------------------------ */}
        <Card className="card-pad md:col-span-2 xl:col-span-5">
          <CardHeader
            title={t("Array")}
            icon={<HardDrive className="size-4" />}
            trailing={
              <div className="flex items-center gap-1.5">
                <Badge tone={array.state === "STARTED" ? "success" : "neutral"} dot>
                  {array.state === "STARTED" ? t("Started") : array.state}
                </Badge>
                <Badge tone={array.status === "protected" ? "violet" : "warning"}>
                  {array.status === "protected" ? t("Protected") : t("Unprotected")}
                </Badge>
              </div>
            }
            action={
              <button type="button" onClick={() => navigate("/storage")} className="text-[10.5px] font-medium text-muted transition hover:text-ink">
                {t("Manage")}
              </button>
            }
          />
          <div className="mt-3 flex items-end justify-between">
            <div>
              <p className="num text-[26px] font-semibold leading-none text-ink">
                {formatBytes(array.used)}
                <span className="ml-1.5 text-[13px] font-normal text-muted">/ {formatBytes(array.total)}</span>
              </p>
              <p className="mt-1.5 text-[10.5px] text-subtle">
                {formatBytes(array.free)} {t("free")} ·{" "}
                {t(array.disks === 1 ? "{count} data disk" : "{count} data disks", { count: array.disks })} ·{" "}
                {pools.length} {pools.length === 1 ? t("pool") : t("pools")}
              </p>
            </div>
            <span className="num text-sm text-soft">{array.usage.toFixed(1)}%</span>
          </div>
          <div className="mt-2.5">
            <SegmentBar
              height={9}
              segments={[
                ...arrayDisks
                  .filter((disk) => disk.type === "Data")
                  .map((disk, index) => ({
                    value: Math.max(1, disk.used),
                    color: disk.status === "standby"
                      ? "var(--surface-3)"
                      : `color-mix(in srgb, var(--accent) ${86 - index * 7}%, var(--surface-3))`,
                    label: `${disk.name}: ${formatBytes(disk.used)}`,
                  })),
                { value: Math.max(1, array.free), color: "var(--surface-3)", label: t("Free") },
              ]}
            />
          </div>
          <div className="mt-4 flex items-end gap-1.5">
            {arrayDisks.map((disk) => (
              <DiskBay key={disk.id} disk={disk} />
            ))}
            {disks
              .filter((disk) => disk.type === "Cache")
              .slice(0, 2)
              .map((disk) => (
                <DiskBay key={disk.id} disk={disk} cache />
              ))}
          </div>
          <div className="mt-4 rounded-xl border border-line bg-[var(--surface-2)]/70 px-3 py-2.5">
            <div className="flex items-center justify-between text-[10.5px]">
              <span className="flex items-center gap-1.5 font-medium text-soft">
                <span className={cn("status-dot", array.parity.paused ? "warning" : "live")} />
                {array.parity.paused ? t("Parity check paused") : array.parity.running ? t("Parity check running") : t("Parity check idle")}
              </span>
              <span className="num text-muted">{array.parity.progress.toFixed(1)}%</span>
            </div>
            <Progress value={array.parity.progress} height={4} className="mt-2" />
            <div className="mt-1.5 flex justify-between font-mono text-[9px] text-subtle">
              <span>{formatSpeed(array.parity.speed)}</span>
              <span>{t(array.parity.errors === 1 ? "{count} sync error" : "{count} sync errors", { count: array.parity.errors })}</span>
            </div>
          </div>
        </Card>

        {/* Network ---------------------------------------------------------- */}
        <Card className="card-pad md:col-span-1 xl:col-span-4">
          <CardHeader
            title={t("Network")}
            icon={<NetworkIcon className="size-4" />}
            trailing={
              <select
                value={selectedInterface?.name ?? ""}
                onChange={(event) => networkInterface.selectInterface(event.target.value)}
                aria-label={t("Network interface")}
                className="h-6 max-w-[140px] rounded-md border border-line bg-[var(--surface-2)] px-1.5 font-mono text-[9px] text-soft outline-none focus:border-[var(--accent-line)]"
              >
                {network.interfaces.map((iface) => (
                  <option key={iface.name} value={iface.name}>
                    {iface.name}{iface.speedMbps ? ` · ${(iface.speedMbps / 1000).toFixed(1)} GbE` : ""}
                  </option>
                ))}
              </select>
            }
            action={
              <button type="button" onClick={() => navigate("/network")} className="text-[10.5px] font-medium text-muted transition hover:text-ink">
                {t("Details")}
              </button>
            }
          />
          <div className="mt-4 grid grid-cols-2 gap-3">
            <div>
              <p className="flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
                <ArrowDownToLine className="size-3 text-[var(--info)]" /> {t("Down")}
              </p>
              <p className="num mt-1 text-[18px] font-semibold text-ink">{formatSpeed(downRate)}</p>
            </div>
            <div>
              <p className="flex items-center gap-1 text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
                <ArrowUpFromLine className="size-3 text-[var(--violet)]" /> {t("Up")}
              </p>
              <p className="num mt-1 text-[18px] font-semibold text-ink">{formatSpeed(upRate)}</p>
            </div>
          </div>
          <AreaChart
            series={[
              { values: netRx, color: "var(--info)", label: t("Down") },
              { values: netTx, color: "var(--violet)", label: t("Up") },
            ]}
            timestamps={netTimestamps}
            height={76}
            formatValue={(value) => formatSpeed(value)}
            className="mt-1"
            domain={[0, Math.max(2_000_000, ...netRx, ...netTx) * 1.15]}
          />
          <div className="mt-3 flex items-center justify-between border-t border-line pt-3 text-[10px] text-subtle">
            <span className="font-mono">{selectedInterface?.ip ?? "—"}</span>
            <span className="flex items-center gap-1">
              <span className="status-dot" />{" "}
              {t(
                network.wireGuard.peers.filter((p) => p.online).length === 1 ? "{count} VPN peer" : "{count} VPN peers",
                { count: network.wireGuard.peers.filter((p) => p.online).length },
              )}
            </span>
          </div>
        </Card>

        {/* UPS ------------------------------------------------------------- */}
        <Card className="card-pad md:col-span-1 xl:col-span-3">
          <CardHeader
            title={t("UPS")}
            icon={<BatteryCharging className="size-4" />}
            trailing={<Badge tone={ups.onBattery ? "warning" : "success"} dot>{ups.onBattery ? t("On battery") : t("Online")}</Badge>}
          />
          {ups.present ? (
            <>
              <div className="mt-4 flex items-center gap-4">
                <Ring
                  value={ups.battery}
                  size={92}
                  stroke={7}
                  color={batteryColor(ups.battery, 55)}
                  color2={batteryColor(ups.battery, 42)}
                >
                  <div className="text-center">
                    <p className="num text-[20px] font-semibold leading-none text-ink">{Math.round(ups.battery)}%</p>
                    <p className="mt-1 text-[8px] uppercase tracking-[0.12em] text-subtle">{t("battery")}</p>
                  </div>
                </Ring>
                <div className="min-w-0 flex-1">
                  <p className="num text-[22px] font-semibold leading-none text-ink">{formatRuntime(ups.runtime)}</p>
                  <p className="mt-1 text-[10px] uppercase tracking-[0.1em] text-subtle">{t("runtime left")}</p>
                  <div className="mt-3 space-y-1.5 text-[11px]">
                    <div className="flex justify-between">
                      <span className="text-muted">{t("Load")}</span>
                      <span className="num text-soft">{ups.load}%</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted">{t("Input")}</span>
                      <span className="num text-soft">{ups.inputVoltage.toFixed(0)} V</span>
                    </div>
                  </div>
                </div>
              </div>
              <p className="mt-3 truncate border-t border-line pt-2.5 text-[9.5px] text-subtle">{ups.model}</p>
            </>
          ) : (
            <p className="py-10 text-center text-xs text-muted">{t("No UPS detected.")}</p>
          )}
        </Card>

        {/* Containers ------------------------------------------------------- */}
        <Card className="card-pad md:col-span-2 xl:col-span-5">
          <CardHeader
            title={t("Containers")}
            icon={<Boxes className="size-4" />}
            trailing={
              <div className="flex items-center gap-1.5">
                <Badge tone="success" dot>
                  {t("{count} running", { count: runningContainers.length })}
                </Badge>
                {updates > 0 ? (
                  <Badge tone="warning">
                    {t(updates === 1 ? "{count} update" : "{count} updates", { count: updates })}
                  </Badge>
                ) : null}
              </div>
            }
            action={
              <button type="button" onClick={() => navigate("/docker")} className="inline-flex items-center gap-0.5 text-[10.5px] font-medium text-muted transition hover:text-ink">
                {t("All containers")} <ChevronRight className="size-3" />
              </button>
            }
          />
          <div className="mt-3 divide-y divide-line/70">
            {runningContainers.slice(0, 5).map((container) => (
              <button
                key={container.id}
                type="button"
                onClick={() => navigate(`/docker?container=${encodeURIComponent(container.name)}`)}
                className="group flex w-full items-center gap-3 py-2 text-left first:pt-0 last:pb-0"
              >
                <EntityIcon
                  src={containerIconURL(container.id)}
                  name={container.name}
                  size={28}
                  className="rounded-lg"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12px] font-medium text-ink transition group-hover:text-accent">{container.name}</p>
                  <p className="truncate text-[9.5px] text-subtle">{container.image}</p>
                </div>
                <div className="hidden w-24 sm:block">
                  <div className="flex items-center justify-between font-mono text-[9px] text-subtle">
                    <span>CPU</span>
                    <span className="text-soft">{container.cpu.toFixed(1)}%</span>
                  </div>
                  <Progress value={Math.min(100, container.cpu * 2)} height={3} className="mt-1" />
                </div>
                <div className="w-20 text-right">
                  <p className="num text-[10.5px] text-soft">{formatMemory(container.memory)}</p>
                  <p className="text-[9px] text-subtle">{formatUptime(container.uptime)}</p>
                </div>
                <span className="status-dot live" />
              </button>
            ))}
          </div>
        </Card>

        {/* Shares ----------------------------------------------------------- */}
        <Card className="card-pad md:col-span-1 xl:col-span-4">
          <CardHeader
            title={t("Shares")}
            icon={<FolderTree className="size-4" />}
            trailing={<span className="chip">{t("{count} mounted", { count: shares.length })}</span>}
            action={
              <button type="button" onClick={() => navigate("/storage")} className="text-[10.5px] font-medium text-muted transition hover:text-ink">
                {t("Manage")}
              </button>
            }
          />
          <div className="mt-3 space-y-3">
            {primaryShare.map((share) => (
              <div key={share.name}>
                <div className="flex items-baseline justify-between gap-3">
                  <span className="truncate text-[12px] font-medium text-soft">{share.name}</span>
                  <span className="shrink-0 font-mono text-[9.5px] text-subtle">
                    {formatBytes(share.used)} / {formatBytes(share.size)}
                  </span>
                </div>
                <Progress value={share.usage} height={4} className="mt-1.5" />
              </div>
            ))}
          </div>
          <p className="mt-3.5 border-t border-line pt-2.5 text-[9.5px] text-subtle">
            {t(
              shares.length === 1
                ? "{size} allocated across {count} share"
                : "{size} allocated across {count} shares",
              {
                size: formatBytes(shares.reduce((sum, share) => sum + share.used, 0)),
                count: shares.length,
              },
            )}
          </p>
        </Card>

        {/* Activity --------------------------------------------------------- */}
        <Card className="card-pad md:col-span-1 xl:col-span-3">
          <CardHeader
            title={t("Activity")}
            icon={<Clock3 className="size-4" />}
            trailing={
              unread > 0 ? (
                <Badge tone="accent">{t("{count} new", { count: unread })}</Badge>
              ) : null
            }
            action={
              <button type="button" onClick={() => navigate("/notifications")} className="text-[10.5px] font-medium text-muted transition hover:text-ink">
                {t("Inbox")}
              </button>
            }
          />
          <div className="mt-3 space-y-0">
            {notifications.slice(0, 4).map((item, index) => {
              const color =
                item.importance === "alert" ? "var(--danger)" : item.importance === "warning" ? "var(--warning)" : "var(--info)";
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => navigate("/notifications")}
                  className="relative flex w-full gap-2.5 pb-3.5 text-left last:pb-0"
                >
                  {index < 3 ? (
                    <span className="absolute left-[5px] top-3 h-full w-px bg-line" aria-hidden="true" />
                  ) : null}
                  <span
                    className="relative z-10 mt-1 size-[11px] shrink-0 rounded-full border-2"
                    style={{ borderColor: color, background: "var(--surface)" }}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="flex items-baseline justify-between gap-2">
                      <span className="truncate text-[11.5px] font-medium text-soft">{item.subject}</span>
                      <span className="shrink-0 font-mono text-[8.5px] text-subtle">{relativeTime(item.timestamp)}</span>
                    </span>
                    <span className="mt-0.5 line-clamp-2 text-[10px] leading-snug text-muted">{item.description}</span>
                  </span>
                </button>
              );
            })}
          </div>
        </Card>
      </div>
    </>
  );
}

/* -------------------------------------------------------------------------- */

function CardHeader({
  title,
  icon,
  trailing,
  action,
}: {
  title: string;
  icon: ReactNode;
  trailing?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">
          {icon}
        </span>
        <h2 className="text-[12.5px] font-semibold text-ink">{title}</h2>
      </div>
      <div className="flex items-center gap-2">
        {trailing}
        {action}
      </div>
    </div>
  );
}

function MiniStat({
  label,
  value,
  icon,
  tone,
}: {
  label: string;
  value: string;
  icon?: ReactNode;
  tone?: "warning";
}) {
  return (
    <div className="px-3 first:pl-0 last:pr-0">
      <p className="flex items-center gap-1 text-[9px] uppercase tracking-[0.1em] text-subtle">
        {icon}
        {label}
      </p>
      <p className={cn("num mt-1 text-[11.5px] font-medium", tone === "warning" ? "text-[var(--warning)]" : "text-soft")}>{value}</p>
    </div>
  );
}

function UsageLegend({ color, label, value }: { color: string; label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-2 text-[10.5px]">
      <span className="flex items-center gap-1.5 text-muted">
        <span className="size-1.5 rounded-full" style={{ background: color }} />
        {label}
      </span>
      <span className="num text-soft">{value}</span>
    </div>
  );
}

function DiskBay({ disk, cache = false }: { disk: Disk; cache?: boolean }) {
  const { t } = useI18n();
  const used = disk.size > 0 ? (disk.used / disk.size) * 100 : 0;
  const color = disk.status === "standby" ? "var(--subtle)" : cache ? "var(--info)" : disk.type === "Parity" ? "var(--violet)" : "var(--accent)";
  return (
    <div className="group/bay min-w-0 flex-1" title={`${disk.name} · ${disk.model} · ${formatBytes(disk.used)} ${t("used")} · ${disk.temp || "—"}°C`}>
      <div className="relative h-9 overflow-hidden rounded-[4px] border border-line bg-[var(--surface-3)]/60">
        <div
          className="absolute inset-x-0 bottom-0 transition-all duration-700"
          style={{
            height: `${Math.max(6, used)}%`,
            background: `linear-gradient(180deg, ${color}, color-mix(in srgb, ${color} 55%, transparent))`,
            opacity: disk.status === "standby" ? 0.25 : 0.85,
          }}
        />
        {disk.smart === "warning" ? (
          <span className="absolute right-0.5 top-0.5 size-1 rounded-full bg-[var(--warning)]" />
        ) : null}
      </div>
      <p className="mt-1 truncate text-center font-mono text-[8px] text-subtle group-hover/bay:text-muted">
        {bayLabel(disk)}
      </p>
    </div>
  );
}

function metric(value: number) {
  return value.toFixed(1);
}

// 100% is green, 20% is red; below 20% stays red.
function batteryColor(percent: number, lightness: number) {
  const ratio = (Math.min(100, Math.max(20, percent)) - 20) / 80;
  return `hsl(${Math.round(ratio * 140)} 72% ${lightness}%)`;
}

function formatRuntime(minutes: number) {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return `${hours}h ${String(rest).padStart(2, "0")}m`;
}

function bayLabel(disk: Disk) {
  if (disk.type === "Parity") return disk.name === "parity" ? "P" : "P2";
  if (disk.type === "Cache") return disk.name === "cache" ? "C1" : disk.name.replace("cache", "C");
  return disk.name.replace("disk", "");
}
