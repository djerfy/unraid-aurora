import type { ReactNode } from "react";
import { ChevronRight, Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";
import { cn } from "../ui/primitives";

export function FolderTable({
  name,
  count,
  columns,
  open,
  onToggle,
  children,
}: {
  name: string;
  count: number;
  columns: ReactNode;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  const { t } = useI18n();

  return (
    <section className="card mb-4 overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-line bg-[var(--surface-2)]/50 px-4 py-2.5">
        <button
          type="button"
          aria-expanded={open}
          onClick={onToggle}
          className="flex min-w-0 items-center gap-2 text-left"
        >
          <ChevronRight
            className={cn("size-3.5 shrink-0 text-subtle transition-transform", open && "rotate-90")}
          />
          <span className="grid size-7 shrink-0 place-items-center rounded-md border border-[var(--border-accent)] bg-[var(--accent-soft)] text-[var(--accent)]">
            <FolderIcon className="size-3.5" />
          </span>
          <span className="min-w-0 truncate text-[12px] font-semibold text-ink">{name}</span>
        </button>
        <span className="font-mono text-[9px] text-subtle">{t("{count} items", { count })}</span>
      </div>
      {open ? (
        <>
          {columns}
          <div className="divide-y divide-line/60">{children}</div>
        </>
      ) : null}
    </section>
  );
}
