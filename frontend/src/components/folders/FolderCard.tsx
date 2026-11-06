import type { ReactNode } from "react";
import { ChevronRight, Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";
import { cn } from "../ui/primitives";

export function FolderCard({
  name,
  count,
  open,
  onToggle,
  children,
}: {
  name: string;
  count: number;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  const { t } = useI18n();

  return (
    <section
      className={cn(
        "card flex flex-col p-4",
        open && "md:col-span-2 xl:col-span-3",
      )}
    >
      <button
        type="button"
        aria-expanded={open}
        onClick={onToggle}
        className="flex w-full items-center gap-2.5 text-left"
      >
        <ChevronRight
          className={cn("size-3.5 shrink-0 text-subtle transition-transform", open && "rotate-90")}
        />
        <span className="grid size-9 shrink-0 place-items-center rounded-[9px] border border-[var(--border-accent)] bg-[var(--accent-soft)] text-[var(--accent)]">
          <FolderIcon className="size-4" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-[13px] font-semibold text-ink">{name}</span>
          <span className="font-mono text-[9.5px] text-subtle">{t("{count} items", { count })}</span>
        </span>
      </button>
      {open ? (
        <div className="mt-4">
          {count === 0 ? (
            <p className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-3 text-center text-[10.5px] text-subtle">
              {t("No items")}
            </p>
          ) : (
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">{children}</div>
          )}
        </div>
      ) : null}
    </section>
  );
}
