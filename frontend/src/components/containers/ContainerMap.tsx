import { useMemo, useState, type ReactNode } from "react";
import { Boxes, Cpu, HardDrive, MemoryStick, Network } from "lucide-react";
import { useI18n } from "../../i18n";
import { formatBytes, formatMemory, formatPercent } from "../../lib/format";
import { Card, EmptyState, cn } from "../ui/primitives";
import type { Container } from "../../lib/types";

type MapMode = "compute" | "network" | "io";

type Point = {
  container: Container;
  x: number;
  y: number;
  value: number;
  color: string;
};

const MODES: Record<MapMode, { label: string; icon: typeof Cpu; x: string; y: string; size: string }> = {
  compute: { label: "Compute", icon: Cpu, x: "CPU", y: "Memory", size: "Memory" },
  network: { label: "Network", icon: Network, x: "RX rate", y: "TX rate", size: "RX + TX" },
  io: { label: "Disk I/O", icon: HardDrive, x: "Read rate", y: "Write rate", size: "Read + Write" },
};

const VIEW = { width: 860, height: 410, left: 72, right: 22, top: 18, bottom: 52 };

function niceMax(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const exponent = Math.floor(Math.log10(value));
  const magnitude = 10 ** exponent;
  const fraction = value / magnitude;
  const nice = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10;
  return nice * magnitude;
}

function toPoint(container: Container, mode: MapMode): Point {
  switch (mode) {
    case "network":
      return {
        container,
        x: container.networkRxRate,
        y: container.networkTxRate,
        value: container.networkRxRate + container.networkTxRate,
        color: "var(--info)",
      };
    case "io":
      return {
        container,
        x: container.blockReadRate,
        y: container.blockWriteRate,
        value: container.blockReadRate + container.blockWriteRate,
        color: "var(--violet)",
      };
    default:
      return {
        container,
        x: container.cpu,
        y: container.memory,
        value: container.memory,
        color: container.color,
      };
  }
}

function formatRate(value: number): string {
  return `${formatBytes(value, value >= 1_000_000 ? 1 : 0)}/s`;
}

function niceMemory(value: number): number {
  const mib = 1024 ** 2;
  const gib = 1024 ** 3;
  if (value >= gib) {
    return (Math.ceil((value / gib) * 2) / 2) * gib;
  }
  return Math.max(16, Math.ceil(value / (128 * mib)) * 128) * mib;
}

function axisLabel(mode: MapMode, axis: "x" | "y", value: number): string {
  if (mode === "compute" && axis === "x") return formatPercent(value, value < 10 ? 1 : 0);
  if (mode === "compute") return formatMemory(value);
  return formatRate(value);
}

function pointValue(container: Container, mode: MapMode): string {
  if (mode === "compute") {
    return `${formatPercent(container.cpu, 1)} CPU · ${formatMemory(container.memory)}`;
  }
  if (mode === "network") {
    return formatRate(container.networkRxRate + container.networkTxRate);
  }
  return formatRate(container.blockReadRate + container.blockWriteRate);
}

function shortName(name: string, length = 18): string {
  return name.length > length ? `${name.slice(0, length - 1)}…` : name;
}

export function ContainerMap({
  containers,
  onOpen,
}: {
  containers: Container[];
  onOpen: (container: Container) => void;
}) {
  const { t } = useI18n();
  const [mode, setMode] = useState<MapMode>("compute");
  const config = MODES[mode];
  const ModeIcon = config.icon;

  const running = useMemo(() => containers.filter((container) => container.state === "running"), [containers]);
  const points = useMemo(() => running.map((container) => toPoint(container, mode)), [running, mode]);
  const maxX = niceMax(Math.max(...points.map((point) => point.x), 1));
  const maxY = mode === "compute"
    ? niceMemory(Math.max(...points.map((point) => point.y), 1))
    : niceMax(Math.max(...points.map((point) => point.y), 1));
  const maxValue = Math.max(...points.map((point) => point.value), 1);
  const ranking = useMemo(
    () => [...points].sort((a, b) => b.value - a.value).slice(0, 6),
    [points],
  );

  const totalCpu = running.reduce((sum, container) => sum + container.cpu, 0);
  const totalMemory = running.reduce((sum, container) => sum + container.memory, 0);
  const totalNetwork = running.reduce(
    (sum, container) => sum + container.networkRxRate + container.networkTxRate,
    0,
  );
  const totalIO = running.reduce(
    (sum, container) => sum + container.blockReadRate + container.blockWriteRate,
    0,
  );

  if (running.length === 0) {
    return (
      <Card>
        <EmptyState
          icon={<Boxes className="size-5" />}
          title={t("No running containers")}
          description={t("Start a container to see it on the resource map.")}
        />
      </Card>
    );
  }

  const plotWidth = VIEW.width - VIEW.left - VIEW.right;
  const plotHeight = VIEW.height - VIEW.top - VIEW.bottom;
  const scaleX = (value: number) => VIEW.left + Math.min(1, value / maxX) * plotWidth;
  const scaleY = (value: number) => VIEW.top + plotHeight - Math.min(1, value / maxY) * plotHeight;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <div className="inline-flex items-center gap-0.5 rounded-xl border border-line bg-[var(--surface-2)] p-1">
          {(Object.keys(MODES) as MapMode[]).map((value) => {
            const Icon = MODES[value].icon;
            const active = value === mode;
            return (
              <button
                key={value}
                type="button"
                onClick={() => setMode(value)}
                aria-pressed={active}
                className={cn(
                  "inline-flex h-8 items-center gap-1.5 rounded-lg px-3 text-[11.5px] font-medium transition",
                  active ? "bg-[var(--surface)] text-ink shadow-[0_1px_2px_rgba(0,0,0,0.12)] ring-1 ring-line" : "text-muted hover:text-ink",
                )}
              >
                <Icon className="size-3.5" />
                {t(MODES[value].label)}
              </button>
            );
          })}
        </div>
        <span className="text-[10px] text-subtle">
          {t("Bubble size: {metric}", { metric: t(config.size) })}
        </span>
        <span className="ml-auto text-[10px] text-subtle">{t("Click a bubble to open the container details.")}</span>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_270px]">
        <div className="rounded-2xl border border-line bg-[var(--surface-2)]/35 p-3">
          <svg
            viewBox={`0 0 ${VIEW.width} ${VIEW.height}`}
            className="h-[390px] w-full"
            role="img"
            aria-label={t("Container resource map")}
            preserveAspectRatio="xMidYMid meet"
          >
            {Array.from({ length: 5 }, (_, index) => {
              const ratio = index / 4;
              const x = VIEW.left + ratio * plotWidth;
              const y = VIEW.top + plotHeight - ratio * plotHeight;
              return (
                <g key={index}>
                  <line
                    x1={x}
                    y1={VIEW.top}
                    x2={x}
                    y2={VIEW.top + plotHeight}
                    stroke="var(--border)"
                    strokeDasharray="2 5"
                  />
                  <line
                    x1={VIEW.left}
                    y1={y}
                    x2={VIEW.left + plotWidth}
                    y2={y}
                    stroke="var(--border)"
                    strokeDasharray="2 5"
                  />
                  <text
                    x={x}
                    y={VIEW.top + plotHeight + 17}
                    textAnchor="middle"
                    fontSize={9}
                    fill="var(--subtle)"
                  >
                    {axisLabel(mode, "x", maxX * ratio)}
                  </text>
                  <text
                    x={VIEW.left - 9}
                    y={y + 3}
                    textAnchor="end"
                    fontSize={9}
                    fill="var(--subtle)"
                  >
                    {axisLabel(mode, "y", maxY * ratio)}
                  </text>
                </g>
              );
            })}

            <text
              x={VIEW.left + plotWidth / 2}
              y={VIEW.height - 8}
              textAnchor="middle"
              fontSize={9.5}
              fill="var(--muted)"
            >
              {t(config.x)}
            </text>
            <text
              transform={`translate(14 ${VIEW.top + plotHeight / 2}) rotate(-90)`}
              textAnchor="middle"
              fontSize={9.5}
              fill="var(--muted)"
            >
              {t(config.y)}
            </text>

            {points.map((point) => {
              const cx = scaleX(point.x);
              const cy = scaleY(point.y);
              const radius = 9 + Math.sqrt(point.value / maxValue) * 23;
              const showLabel = points.length <= 16 || point.value >= maxValue * 0.25;
              const labelY = cy + radius + 12 > VIEW.top + plotHeight
                ? cy - radius - 7
                : cy + radius + 12;
              const container = point.container;
              return (
                <g
                  key={container.id}
                  role="button"
                  tabIndex={0}
                  aria-label={container.name}
                  onClick={() => onOpen(container)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onOpen(container);
                    }
                  }}
                  className="cursor-pointer outline-none"
                >
                  <circle
                    cx={cx}
                    cy={cy}
                    r={radius}
                    fill={point.color}
                    fillOpacity={0.16}
                    stroke={point.color}
                    strokeWidth={1.5}
                  />
                  <circle cx={cx} cy={cy} r={2.5} fill={point.color} />
                  {showLabel ? (
                    <text
                      x={cx}
                      y={labelY}
                      textAnchor="middle"
                      fontSize={9}
                      fill="var(--muted)"
                    >
                      {shortName(container.name)}
                    </text>
                  ) : null}
                  <title>
                    {`${container.name}\nCPU: ${formatPercent(container.cpu, 1)} · ${t("Memory")}: ${formatMemory(container.memory)}\n${t("Network")}: ${formatRate(container.networkRxRate + container.networkTxRate)}\n${t("Disk I/O")}: ${formatRate(container.blockReadRate + container.blockWriteRate)}`}
                  </title>
                </g>
              );
            })}
          </svg>
        </div>

        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-2">
            <MapStat icon={<Cpu className="size-3.5" />} label={t("CPU")} value={formatPercent(totalCpu, 1)} />
            <MapStat icon={<MemoryStick className="size-3.5" />} label={t("Memory")} value={formatMemory(totalMemory)} />
            <MapStat icon={<Network className="size-3.5" />} label={t("Network")} value={formatRate(totalNetwork)} />
            <MapStat icon={<HardDrive className="size-3.5" />} label={t("Disk I/O")} value={formatRate(totalIO)} />
          </div>

          <Card className="p-3">
            <div className="flex items-center gap-2">
              <ModeIcon className="size-3.5 text-[var(--accent)]" />
              <p className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Top consumers")}</p>
            </div>
            <div className="mt-2.5 space-y-2">
              {ranking.map((point) => {
                const container = point.container;
                const width = Math.max(2, (point.value / maxValue) * 100);
                return (
                  <button
                    key={container.id}
                    type="button"
                    onClick={() => onOpen(container)}
                    className="block w-full text-left transition hover:text-ink"
                  >
                    <div className="flex items-center gap-2">
                      <span className="min-w-0 flex-1 truncate text-[10.5px] font-medium text-soft">{container.name}</span>
                      <span className="num shrink-0 text-[9.5px] text-subtle">{pointValue(container, mode)}</span>
                    </div>
                    <div className="mt-1 h-1 overflow-hidden rounded-full bg-[var(--surface-3)]">
                      <div
                        className="h-full rounded-full"
                        style={{ width: `${width}%`, background: point.color }}
                      />
                    </div>
                  </button>
                );
              })}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

function MapStat({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-xl border border-line bg-[var(--surface-2)]/45 px-2.5 py-2">
      <p className="flex items-center gap-1 text-[8.5px] uppercase tracking-[0.08em] text-subtle">
        {icon}
        {label}
      </p>
      <p className="num mt-1 truncate text-[12px] font-semibold text-soft">{value}</p>
    </div>
  );
}
