import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { cn } from "./primitives";

/* -------------------------------------------------------------------------- */
/* Ring gauge                                                                  */
/* -------------------------------------------------------------------------- */

export function Ring({
  value,
  size = 128,
  stroke = 9,
  children,
  trackWidth = 1,
  className,
}: {
  value: number;
  size?: number;
  stroke?: number;
  children?: ReactNode;
  trackWidth?: number;
  className?: string;
}) {
  const id = useId().replace(/:/g, "");
  const radius = (size - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  const safe = Math.min(100, Math.max(0, value));
  const offset = circumference - (safe / 100) * circumference;

  return (
    <div className={cn("relative inline-grid place-items-center", className)} style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90" aria-hidden="true">
        <defs>
          <linearGradient id={`ring-${id}`} x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="var(--accent)" />
            <stop offset="100%" stopColor="var(--accent-2)" />
          </linearGradient>
          <filter id={`glow-${id}`} x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--border)"
          strokeWidth={stroke}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={`url(#ring-${id})`}
          strokeWidth={stroke * trackWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          filter={`url(#glow-${id})`}
          style={{ transition: "stroke-dashoffset 900ms cubic-bezier(0.22,1,0.36,1)" }}
        />
      </svg>
      <div className="absolute inset-0 grid place-items-center">{children}</div>
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Responsive measurement                                                      */
/* -------------------------------------------------------------------------- */

function useWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    const observer = new ResizeObserver((entries) => {
      const next = entries[0]?.contentRect.width ?? 0;
      setWidth(next);
    });
    observer.observe(node);
    setWidth(node.getBoundingClientRect().width);
    return () => observer.disconnect();
  }, []);
  return [ref, width] as const;
}

/* -------------------------------------------------------------------------- */
/* Path helpers                                                                */
/* -------------------------------------------------------------------------- */

type Point = { x: number; y: number };

function smoothPath(points: Point[]): string {
  if (points.length === 0) return "";
  if (points.length === 1) return `M ${points[0].x} ${points[0].y}`;
  let d = `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i - 1] ?? points[i];
    const p1 = points[i];
    const p2 = points[i + 1];
    const p3 = points[i + 2] ?? p2;
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;
    d += ` C ${cp1x.toFixed(2)} ${cp1y.toFixed(2)}, ${cp2x.toFixed(2)} ${cp2y.toFixed(2)}, ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`;
  }
  return d;
}

function downsample(values: number[], max: number): number[] {
  if (values.length <= max) return values;
  const step = (values.length - 1) / (max - 1);
  return Array.from({ length: max }, (_, i) => values[Math.min(values.length - 1, Math.round(i * step))]);
}

/* -------------------------------------------------------------------------- */
/* Area chart                                                                  */
/* -------------------------------------------------------------------------- */

export type Series = { values: number[]; color: string; label?: string; dashed?: boolean };

export function AreaChart({
  series,
  height = 140,
  timestamps,
  formatValue = (v) => String(Math.round(v)),
  domain,
  grid = true,
  fill = true,
  showCursor = true,
  showAxis = false,
  strokeWidth = 2,
  className,
  labelFormat,
}: {
  series: Series[];
  height?: number;
  timestamps?: number[];
  formatValue?: (value: number) => string;
  domain?: [number, number];
  grid?: boolean;
  fill?: boolean;
  showCursor?: boolean;
  showAxis?: boolean;
  strokeWidth?: number;
  className?: string;
  labelFormat?: (timestamp: number) => string;
}) {
  const id = useId().replace(/:/g, "");
  const [ref, width] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);

  const maxPoints = Math.max(40, Math.min(180, Math.floor(width / 4) || 120));
  const seriesPoints = useMemo(
    () => series.map((s) => downsample(s.values, maxPoints)),
    [series, maxPoints],
  );

  const all = seriesPoints.flat();
  const min = domain ? domain[0] : Math.min(...all, 0);
  const max = domain ? domain[1] : Math.max(...all, 1);
  const span = max - min || 1;

  const chartHeight = height;
  const padTop = 6;
  const padBottom = showAxis ? 18 : 4;
  const usableHeight = Math.max(1, chartHeight - padTop - padBottom);

  const toPoints = (values: number[]): Point[] =>
    values.map((value, index) => ({
      x: values.length === 1 ? width / 2 : (index / (values.length - 1)) * width,
      y: padTop + (1 - (value - min) / span) * usableHeight,
    }));

  const hoverIndex = (() => {
    if (!showCursor || hover === null || seriesPoints[0]?.length === undefined) return null;
    const ratio = hover / Math.max(1, width);
    return Math.min(seriesPoints[0].length - 1, Math.round(ratio * (seriesPoints[0].length - 1)));
  })();

  return (
    <div ref={ref} className={cn("relative w-full select-none", className)} style={{ height }}>
      {width > 0 ? (
        <svg
          width={width}
          height={height}
          className="overflow-visible"
          onMouseMove={(event) => {
            const rect = event.currentTarget.getBoundingClientRect();
            setHover(event.clientX - rect.left);
          }}
          onMouseLeave={() => setHover(null)}
        >
          <defs>
            {seriesPoints.map((_, index) => (
              <linearGradient key={index} id={`area-${id}-${index}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={series[index].color} stopOpacity="0.28" />
                <stop offset="100%" stopColor={series[index].color} stopOpacity="0" />
              </linearGradient>
            ))}
          </defs>

          {grid
            ? [0, 0.25, 0.5, 0.75, 1].map((ratio) => (
                <line
                  key={ratio}
                  x1={0}
                  x2={width}
                  y1={padTop + ratio * usableHeight}
                  y2={padTop + ratio * usableHeight}
                  stroke="var(--border)"
                  strokeWidth="1"
                  strokeDasharray={ratio === 1 ? undefined : "3 6"}
                  opacity={ratio === 1 ? 0.9 : 0.55}
                />
              ))
            : null}

          {seriesPoints.map((values, index) => {
            const points = toPoints(values);
            const line = smoothPath(points);
            const area = `${line} L ${width} ${padTop + usableHeight} L 0 ${padTop + usableHeight} Z`;
            return (
              <g key={index}>
                {fill ? <path d={area} fill={`url(#area-${id}-${index})`} /> : null}
                <path
                  d={line}
                  fill="none"
                  stroke={series[index].color}
                  strokeWidth={strokeWidth}
                  strokeLinecap="round"
                  strokeDasharray={series[index].dashed ? "4 5" : undefined}
                  style={{ transition: "d 500ms ease" }}
                />
              </g>
            );
          })}

          {hoverIndex !== null && seriesPoints[0]?.[hoverIndex] !== undefined ? (
            <g>
              <line
                x1={toPoints(seriesPoints[0])[hoverIndex].x}
                x2={toPoints(seriesPoints[0])[hoverIndex].x}
                y1={0}
                y2={padTop + usableHeight}
                stroke="var(--border-strong)"
                strokeWidth="1"
              />
              {seriesPoints.map((values, index) => {
                const point = toPoints(values)[hoverIndex];
                return point ? (
                  <g key={index}>
                    <circle cx={point.x} cy={point.y} r="5" fill="var(--surface)" stroke={series[index].color} strokeWidth="2" />
                  </g>
                ) : null;
              })}
            </g>
          ) : null}

          {showAxis && timestamps
            ? [0, 0.5, 1].map((ratio) => {
                const index = Math.min((timestamps.length || 1) - 1, Math.round(ratio * ((timestamps.length || 1) - 1)));
                const label = timestamps[index]
                  ? (labelFormat ?? ((timestamp: number) => new Date(timestamp * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })))(timestamps[index])
                  : "";
                return (
                  <text
                    key={ratio}
                    x={ratio === 0 ? 0 : ratio === 1 ? width : width / 2}
                    y={height - 2}
                    textAnchor={ratio === 0 ? "start" : ratio === 1 ? "end" : "middle"}
                    fill="var(--subtle)"
                    fontSize="9"
                    fontFamily="var(--font-mono)"
                  >
                    {label}
                  </text>
                );
              })
            : null}
        </svg>
      ) : null}

      {hoverIndex !== null && seriesPoints[0]?.[hoverIndex] !== undefined ? (
        <div
          className="pointer-events-none absolute top-1 z-20 min-w-[104px] rounded-lg border border-line bg-[var(--surface)] px-2.5 py-1.5 shadow-[var(--shadow-pop)]"
          style={{
            left: Math.min(
              Math.max(8, ((hoverIndex / Math.max(1, seriesPoints[0].length - 1)) * width) - 52),
              Math.max(8, width - 112),
            ),
          }}
        >
          {series.map((s, index) => (
            <div key={index} className="flex items-center justify-between gap-3 py-px">
              <span className="flex items-center gap-1.5 text-[10px] text-muted">
                <span className="size-1.5 rounded-full" style={{ background: s.color }} />
                {s.label ?? "Value"}
              </span>
              <span className="num text-[10.5px] font-medium text-ink">
                {formatValue(seriesPoints[index][hoverIndex] ?? 0)}
              </span>
            </div>
          ))}
          {timestamps && timestamps[hoverIndex] ? (
            <p className="mt-1 border-t border-line pt-1 font-mono text-[9px] text-subtle">
              {(labelFormat ?? ((timestamp: number) => new Date(timestamp * 1000).toLocaleTimeString()))(timestamps[hoverIndex])}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Bar chart                                                                   */
/* -------------------------------------------------------------------------- */

export function BarChart({
  values,
  labels,
  height = 160,
  formatValue = (value) => String(Math.round(value)),
  color = "var(--accent)",
  className,
  showValues = false,
}: {
  values: number[];
  labels: string[];
  height?: number;
  formatValue?: (value: number) => string;
  color?: string;
  className?: string;
  showValues?: boolean;
}) {
  const id = useId().replace(/:/g, "");
  const [ref, width] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const max = Math.max(1, ...values) * 1.15;
  const padTop = 12;
  const labelHeight = 20;
  const plotHeight = height - padTop - labelHeight;
  const slot = values.length ? width / values.length : width;
  const barWidth = Math.max(8, slot * 0.42);

  return (
    <div ref={ref} className={cn("relative w-full select-none", className)} style={{ height }}>
      {width > 0 ? (
        <svg width={width} height={height} onMouseLeave={() => setHover(null)}>
          <defs>
            <linearGradient id={`bar-${id}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity="0.95" />
              <stop offset="100%" stopColor={color} stopOpacity="0.35" />
            </linearGradient>
          </defs>
          {[0, 0.5, 1].map((ratio) => (
            <line
              key={ratio}
              x1={0}
              x2={width}
              y1={padTop + ratio * plotHeight}
              y2={padTop + ratio * plotHeight}
              stroke="var(--border)"
              strokeDasharray={ratio === 1 ? undefined : "3 6"}
              opacity={ratio === 1 ? 1 : 0.6}
            />
          ))}
          {values.map((value, index) => {
            const h = Math.max(2, (value / max) * plotHeight);
            const x = index * slot + (slot - barWidth) / 2;
            const y = padTop + plotHeight - h;
            return (
              <g key={index} onMouseEnter={() => setHover(index)}>
                <rect x={index * slot} y={padTop} width={slot} height={plotHeight} fill="transparent" />
                <rect x={x} y={y} width={barWidth} height={h} rx={4} fill={`url(#bar-${id})`} opacity={hover === null || hover === index ? 1 : 0.45} />
                {showValues ? (
                  <text
                    x={index * slot + slot / 2}
                    y={Math.max(10, y - 6)}
                    textAnchor="middle"
                    fill="var(--muted)"
                    fontSize="9.5"
                    fontFamily="var(--font-mono)"
                  >
                    {formatValue(value)}
                  </text>
                ) : null}
                {labels[index] && slot > 48 ? (
                  <text
                    x={index * slot + slot / 2}
                    y={height - 4}
                    textAnchor="middle"
                    fill="var(--subtle)"
                    fontSize="9"
                    fontFamily="var(--font-mono)"
                  >
                    {labels[index]}
                  </text>
                ) : null}
              </g>
            );
          })}
        </svg>
      ) : null}
      {hover !== null ? (
        <div
          className="pointer-events-none absolute top-0 z-20 rounded-lg border border-line bg-[var(--surface)] px-2.5 py-1.5 shadow-[var(--shadow-pop)]"
          style={{ left: Math.min(Math.max(8, hover * slot - 44), Math.max(8, width - 110)) }}
        >
          <p className="text-[9px] text-subtle">{labels[hover]}</p>
          <p className="num text-[11px] font-medium text-ink">{formatValue(values[hover])}</p>
        </div>
      ) : null}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Sparkline                                                                   */
/* -------------------------------------------------------------------------- */

export function Sparkline({
  values,
  color = "var(--accent)",
  height = 36,
  fill = true,
  className,
}: {
  values: number[];
  color?: string;
  height?: number;
  fill?: boolean;
  className?: string;
}) {
  const [ref, width] = useWidth<HTMLDivElement>();
  const id = useId().replace(/:/g, "");
  const points = useMemo(() => downsample(values, 90), [values]);
  const min = Math.min(...points);
  const max = Math.max(...points);
  const span = max - min || 1;
  const coords = points.map((value, index) => ({
    x: points.length === 1 ? width / 2 : (index / (points.length - 1)) * width,
    y: 3 + (1 - (value - min) / span) * (height - 6),
  }));
  const line = smoothPath(coords);
  return (
    <div ref={ref} className={cn("w-full", className)} style={{ height }}>
      {width > 0 ? (
        <svg width={width} height={height} className="overflow-visible">
          <defs>
            <linearGradient id={`spark-${id}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity="0.3" />
              <stop offset="100%" stopColor={color} stopOpacity="0" />
            </linearGradient>
          </defs>
          {fill ? <path d={`${line} L ${width} ${height} L 0 ${height} Z`} fill={`url(#spark-${id})`} /> : null}
          <path d={line} fill="none" stroke={color} strokeWidth="1.6" strokeLinecap="round" />
        </svg>
      ) : null}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Core matrix                                                                 */
/* -------------------------------------------------------------------------- */

export function CoreMatrix({ values, className }: { values: number[]; className?: string }) {
  return (
    <div className={cn("flex h-9 items-end gap-[3px]", className)} aria-label="Per-core CPU usage">
      {values.map((value, index) => {
        const intensity = Math.min(1, Math.max(0.12, value / 100));
        return (
          <span
            key={index}
            title={`Core ${index}: ${Math.round(value)}%`}
            className="min-w-[3px] flex-1 rounded-[2px] transition-all duration-700"
            style={{
              height: `${Math.max(14, value)}%`,
              background:
                value > 75
                  ? "var(--warning)"
                  : `color-mix(in srgb, var(--accent) ${Math.round(intensity * 85 + 15)}%, var(--surface-3))`,
              opacity: 0.55 + intensity * 0.45,
            }}
          />
        );
      })}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Segmented capacity bar                                                      */
/* -------------------------------------------------------------------------- */

export function SegmentBar({
  segments,
  height = 10,
  className,
}: {
  segments: { value: number; color: string; label?: string }[];
  height?: number;
  className?: string;
}) {
  const total = segments.reduce((sum, segment) => sum + segment.value, 0) || 1;
  return (
    <div className={cn("flex w-full overflow-hidden rounded-full bg-[var(--surface-3)]", className)} style={{ height }}>
      {segments.map((segment, index) => (
        <div
          key={index}
          title={segment.label}
          style={{
            width: `${(segment.value / total) * 100}%`,
            background: segment.color,
            opacity: 0.85,
          }}
          className="h-full border-r border-[var(--surface)]/40 last:border-0"
        />
      ))}
    </div>
  );
}
