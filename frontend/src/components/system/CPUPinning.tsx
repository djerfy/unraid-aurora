import { Cpu } from "lucide-react";
import { useApp } from "../../app/AppDataProvider";
import { useI18n } from "../../i18n";
import { Badge, Button, cn } from "../ui/primitives";

function parseCPUset(value: string): Set<number> {
  const selected = new Set<number>();
  for (const rawPart of value.split(",")) {
    const part = rawPart.trim();
    if (!part) continue;
    if (part.includes("-")) {
      const [startText, endText] = part.split("-", 2);
      const start = Number(startText);
      const end = Number(endText);
      if (!Number.isInteger(start) || !Number.isInteger(end)) continue;
      for (let index = Math.min(start, end); index <= Math.max(start, end); index += 1) {
        selected.add(index);
      }
    } else {
      const index = Number(part);
      if (Number.isInteger(index)) selected.add(index);
    }
  }
  return selected;
}

function serializeCPUset(selected: Set<number>): string {
  const values = [...selected].filter((value) => Number.isInteger(value) && value >= 0).sort((a, b) => a - b);
  if (values.length === 0) return "";
  const parts: string[] = [];
  let start = values[0];
  let previous = values[0];
  for (let index = 1; index < values.length; index += 1) {
    const value = values[index];
    if (value === previous + 1) {
      previous = value;
      continue;
    }
    parts.push(start === previous ? String(start) : `${start}-${previous}`);
    start = value;
    previous = value;
  }
  parts.push(start === previous ? String(start) : `${start}-${previous}`);
  return parts.join(",");
}

export function CPUPinning({
  value,
  onChange,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  const { snapshot } = useApp();
  const total = Math.max(
    1,
    snapshot?.system.cpu.threads || snapshot?.system.cpu.perCore.length || 32,
  );
  const selected = parseCPUset(value);
  const selectedCount = [...selected].filter((index) => index < total).length;
  const toggle = (index: number) => {
    const next = new Set(selected);
    if (next.has(index)) next.delete(index);
    else next.add(index);
    onChange(serializeCPUset(next));
  };

  return (
    <div className="rounded-xl border border-line bg-[var(--surface-2)]/35 p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-soft">
            <Cpu className="size-3.5" /> {t("CPU pinning")}
          </p>
          <p className="mt-1 text-[9.5px] leading-relaxed text-subtle">
            {t("Pick the logical CPUs dedicated to this workload. No selection leaves it unpinned across all cores.")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone={selectedCount > 0 ? "accent" : "neutral"}>
            {selectedCount > 0 ? t("{count} / {total} cores", { count: selectedCount, total }) : t("All cores")}
          </Badge>
          <Button
            size="sm"
            variant="ghost"
            disabled={disabled}
            onClick={() => onChange(serializeCPUset(new Set(Array.from({ length: total }, (_, index) => index))))}
          >
            {t("Select all")}
          </Button>
          <Button size="sm" variant="ghost" disabled={disabled} onClick={() => onChange("")}>
            {t("Clear")}
          </Button>
        </div>
      </div>
      <div
        className="mt-3 grid gap-1.5"
        style={{ gridTemplateColumns: "repeat(auto-fill, minmax(30px, 1fr))" }}
        role="group"
        aria-label={t("CPU pinning")}
      >
        {Array.from({ length: total }, (_, index) => {
          const active = selected.has(index);
          return (
            <button
              key={index}
              type="button"
              disabled={disabled}
              aria-pressed={active}
              aria-label={t("CPU {index}", { index })}
              title={t("CPU {index}", { index })}
              onClick={() => toggle(index)}
              className={cn(
                "grid h-8 min-w-0 place-items-center rounded-full border font-mono text-[9px] font-semibold transition-all duration-150",
                active
                  ? "border-[var(--accent)] bg-[var(--accent)] text-[var(--accent-contrast)] shadow-[0_0_0_3px_var(--accent-soft)]"
                  : "border-line bg-[var(--surface-2)] text-subtle hover:border-[var(--accent-line)] hover:text-ink",
                disabled && "cursor-not-allowed opacity-45",
              )}
            >
              {index}
            </button>
          );
        })}
      </div>
    </div>
  );
}
