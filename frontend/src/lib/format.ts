const STORAGE_UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];
const MEMORY_UNITS = ["B", "KiB", "MiB", "GiB", "TiB"];

/**
 * Storage sizes use decimal units, like Unraid and drive vendors. Memory uses
 * binary units because that is how DIMMs and Docker limits are reported.
 */
export function formatBytes(bytes: number, digits?: number): string {
  return formatScaled(bytes, 1000, STORAGE_UNITS, digits);
}

export function formatMemory(bytes: number, digits?: number): string {
  return formatScaled(bytes, 1024, MEMORY_UNITS, digits);
}

function formatScaled(bytes: number, base: number, units: string[], digits?: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return `0 ${units[0]}`;
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(base)), units.length - 1);
  const value = bytes / Math.pow(base, i);
  const places = digits ?? (i === 0 ? 0 : value >= 100 ? 0 : value >= 10 ? 1 : 2);
  return `${value.toFixed(places)} ${units[i]}`;
}

export function formatSpeed(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) return "0 B/s";
  return `${formatBytes(bytesPerSecond, bytesPerSecond >= 10_000_000 ? 1 : 0)}/s`;
}

export function formatPercent(value: number, digits = 0): string {
  if (!Number.isFinite(value)) return "0%";
  return `${value.toFixed(digits)}%`;
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat("en-US").format(Math.round(value));
}

export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    return `${days}d ${hours % 24}h`;
  }
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

export function relativeTime(input: string | number | Date): string {
  const date = input instanceof Date ? input : new Date(input);
  if (Number.isNaN(date.getTime()) || date.getTime() <= 0) return "never";
  const diff = Date.now() - date.getTime();
  const abs = Math.abs(diff);
  const minute = 60_000;
  const hour = 3_600_000;
  const day = 86_400_000;

  if (abs < 45_000) return "just now";
  if (abs < hour) {
    const n = Math.round(abs / minute);
    return `${n} min ago`;
  }
  if (abs < day) {
    const n = Math.round(abs / hour);
    return `${n}h ago`;
  }
  if (abs < 7 * day) {
    const n = Math.round(abs / day);
    return `${n}d ago`;
  }
  return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

export function formatDateTime(input: string | number | Date): string {
  const date = input instanceof Date ? input : new Date(input);
  if (Number.isNaN(date.getTime()) || date.getTime() <= 0) return "—";
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatClock(input: string | number | Date): string {
  const date = input instanceof Date ? input : new Date(input);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export function greeting(): string {
  const hour = new Date().getHours();
  if (hour < 6) return "Good night";
  if (hour < 12) return "Good morning";
  if (hour < 18) return "Good afternoon";
  return "Good evening";
}

export function titleCase(value: string): string {
  return value.replace(/(^|\s|-)\w/g, (c) => c.toUpperCase());
}

export function initials(name: string): string {
  const cleaned = name.replace(/[^a-zA-Z0-9 ]/g, " ").trim();
  if (!cleaned) return "?";
  const parts = cleaned.split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function formatTemperature(celsius: number, unit = "celsius"): string {
  if (unit === "fahrenheit") return `${Math.round((celsius * 9) / 5 + 32)}°F`;
  return `${Math.round(celsius)}°C`;
}

export function hexToRgba(hex: string, alpha: number): string {
  const clean = hex.replace("#", "");
  const full = clean.length === 3 ? clean.split("").map((c) => c + c).join("") : clean;
  const n = parseInt(full, 16);
  if (Number.isNaN(n)) return `rgba(255,255,255,${alpha})`;
  return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
}

export function pluralize(count: number, singular: string, plural?: string): string {
  return count === 1 ? singular : (plural ?? `${singular}s`);
}

export function timeAgoShort(input: string | number | Date): string {
  const date = input instanceof Date ? input : new Date(input);
  const diff = Math.max(0, Date.now() - date.getTime());
  if (diff < 60_000) return "now";
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h`;
  return `${Math.floor(diff / 86_400_000)}d`;
}
