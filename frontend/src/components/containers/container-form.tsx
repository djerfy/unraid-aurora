import type { ReactNode } from "react";
import { Plus, Settings2 } from "lucide-react";
import { Button } from "../ui/primitives";
import type { ContainerEnv, ContainerVolume, Port } from "../../lib/types";

export function SectionHeader({
  title,
  hint,
  onAdd,
  addLabel,
}: {
  title: string;
  hint: string;
  onAdd?: () => void;
  addLabel?: string;
}) {
  return (
    <div className="mb-2.5 flex items-end justify-between gap-3">
      <div>
        <h3 className="flex items-center gap-1.5 text-[11.5px] font-semibold uppercase tracking-[0.1em] text-soft">
          <Settings2 className="size-3 text-subtle" /> {title}
        </h3>
        <p className="mt-0.5 text-[9.5px] text-subtle">{hint}</p>
      </div>
      {onAdd && addLabel ? (
        <Button
          size="sm"
          variant="ghost"
          icon={<Plus className="size-3" />}
          onClick={onAdd}
          className="text-[10.5px]"
        >
          {addLabel}
        </Button>
      ) : null}
    </div>
  );
}

export function NumberField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
}) {
  return (
    <input
      type="number"
      min={1}
      max={65535}
      value={value || ""}
      aria-label={label}
      placeholder={label}
      onChange={(event) => onChange(Number(event.target.value))}
      className="h-9 min-w-0 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
    />
  );
}

export function EmptyLine({ icon, text }: { icon: ReactNode; text: string }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border border-dashed border-line px-3 py-2 text-[10px] text-subtle">
      {icon}
      {text}
    </div>
  );
}

export function TextField({
  label,
  value,
  placeholder,
  onChange,
  mono = true,
  className,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
  mono?: boolean;
  className?: string;
}) {
  return (
    <input
      value={value}
      aria-label={label}
      placeholder={placeholder ?? label}
      onChange={(event) => onChange(event.target.value)}
      className={`h-9 min-w-0 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-ink outline-none transition focus:border-[var(--accent-line)] ${
        mono ? "font-mono text-[10.5px]" : ""
      } ${className ?? ""}`}
    />
  );
}

export type ContainerFields = {
  ports: Port[];
  volumes: ContainerVolume[];
  env: ContainerEnv[];
};

export function validateContainerFields(fields: ContainerFields): string {
  for (const volume of fields.volumes) {
    if (!volume.source.trim() || !volume.destination.trim()) {
      return "Every path needs both a host source and a container destination.";
    }
    if (!volume.destination.startsWith("/")) {
      return `Container path “${volume.destination}” must be absolute.`;
    }
  }
  for (const port of fields.ports) {
    if (
      !Number.isInteger(port.host) ||
      !Number.isInteger(port.container) ||
      port.host < 1 ||
      port.host > 65535 ||
      port.container < 1 ||
      port.container > 65535
    ) {
      return "Ports must be integers between 1 and 65535.";
    }
  }
  for (const entry of fields.env) {
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(entry.key)) {
      return `Environment variable “${entry.key}” has an invalid name.`;
    }
  }
  return "";
}
