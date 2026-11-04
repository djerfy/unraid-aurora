import { useI18n } from "../../i18n";
import { Badge, cn } from "../ui/primitives";
import type { ContainerSecurity } from "../../lib/types";

export type SeverityKey = "critical" | "high" | "medium" | "low" | "unknown";

export const SEVERITY_STYLES: Record<SeverityKey, string> = {
  critical: "border-[color-mix(in_srgb,var(--danger)_28%,transparent)] bg-[var(--danger-soft)] text-[var(--danger)]",
  high: "border-orange-500/30 bg-orange-500/10 text-orange-600 dark:text-orange-400",
  medium: "border-yellow-500/30 bg-yellow-500/10 text-yellow-700 dark:text-yellow-300",
  low: "border-[color-mix(in_srgb,var(--info)_28%,transparent)] bg-[var(--info-soft)] text-[var(--info)]",
  unknown: "border-line bg-[var(--surface-2)] text-subtle",
};

const SECURITY_ZERO_STYLE =
  "border-[color-mix(in_srgb,var(--success)_28%,transparent)] bg-[var(--success-soft)] text-[var(--success)]";

export function severityKey(value: string): SeverityKey {
  switch (value.toUpperCase()) {
    case "CRITICAL":
      return "critical";
    case "HIGH":
      return "high";
    case "MEDIUM":
      return "medium";
    case "LOW":
      return "low";
    default:
      return "unknown";
  }
}

export function SecurityCounts({ security, showLabel = false }: { security?: ContainerSecurity; showLabel?: boolean }) {
  const { t } = useI18n();
  if (!security?.enabled) return <span className="text-[9px] text-subtle">{t("Disabled")}</span>;
  if (security.scanning) return <Badge tone="info" dot pulse>{t("Scanning")}</Badge>;
  if (security.error) return <span title={security.error}><Badge tone="danger" dot>{t("Failed")}</Badge></span>;
  if (!security.available) return <span className="text-[9px] text-subtle">{t("Not scanned")}</span>;
  if (security.total === 0) {
    return (
      <span
        title={t("No known vulnerabilities were found for this image.")}
        className={cn(
          "inline-flex h-[19px] min-w-[20px] items-center justify-center rounded-full border px-1.5 font-mono text-[9.5px] font-semibold leading-none",
          SECURITY_ZERO_STYLE,
        )}
      >
        0
      </span>
    );
  }
  const values = [
    { key: "critical" as const, count: security.critical, label: t("Critical") },
    { key: "high" as const, count: security.high, label: t("High") },
    { key: "medium" as const, count: security.medium, label: t("Medium") },
    { key: "low" as const, count: security.low, label: t("Low") },
    { key: "unknown" as const, count: security.unknown, label: t("Unknown") },
  ];
  return (
    <span className="flex flex-wrap items-center gap-1">
      {values.filter((value) => value.count > 0).map((value) => (
        <span
          key={value.key}
          title={`${value.label}: ${value.count}`}
          className={cn(
            "inline-flex h-[19px] items-center justify-center rounded-full border font-semibold leading-none",
            showLabel ? "gap-1 px-2 text-[9px]" : "min-w-[20px] px-1.5 font-mono text-[9.5px]",
            SEVERITY_STYLES[value.key],
          )}
        >
          {showLabel ? <><span>{value.label}</span><span className="font-mono">{value.count}</span></> : value.count}
        </span>
      ))}
    </span>
  );
}

export function SeverityBadge({ severity }: { severity: string }) {
  const key = severityKey(severity);
  const label = severity ? severity.toUpperCase() : "UNKNOWN";
  return (
    <span
      className={cn(
        "inline-flex h-[19px] shrink-0 items-center rounded-full border px-1.5 font-mono text-[8.5px] font-semibold",
        SEVERITY_STYLES[key],
      )}
    >
      {label}
    </span>
  );
}
