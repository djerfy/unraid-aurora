import { useId } from "react";
import { cn } from "../ui/primitives";
import { useI18n } from "../../i18n";

export function AuroraMark({ size = 30, className }: { size?: number; className?: string }) {
  const gradientID = `aurora-mark-${useId().replace(/:/g, "")}`;
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      fill="none"
      className={cn("shrink-0", className)}
      aria-hidden="true"
    >
      <defs>
        <linearGradient id={gradientID} x1="8" y1="56" x2="56" y2="8" gradientUnits="userSpaceOnUse">
          <stop stopColor="var(--accent-2)" />
          <stop offset="0.55" stopColor="var(--accent)" />
          <stop offset="1" stopColor="var(--violet)" />
        </linearGradient>
      </defs>
      <rect x="1" y="1" width="62" height="62" rx="17" fill="var(--surface-2)" stroke="var(--border-strong)" />
      <path
        d="M15 43c6.5 0 9.5-21 17-21s10.5 21 17 21"
        stroke={`url(#${gradientID})`}
        strokeWidth="4.6"
        strokeLinecap="round"
      />
      <circle cx="32" cy="22" r="3.2" fill="var(--accent)" />
    </svg>
  );
}

export function AuroraWordmark({ compact = false }: { compact?: boolean }) {
  const { t } = useI18n();
  return (
    <div className="flex min-w-0 items-center gap-2.5">
      <AuroraMark size={31} />
      {!compact ? (
        <div className="min-w-0 leading-none">
          <p className="truncate text-[16px] font-semibold tracking-[-0.03em] text-ink">aurora</p>
          <p className="mt-1 text-[8.5px] font-semibold uppercase tracking-[0.24em] text-subtle">{t("Unraid control")}</p>
        </div>
      ) : null}
    </div>
  );
}
