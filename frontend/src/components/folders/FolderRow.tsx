import { useState, type ReactNode } from "react";
import { ChevronRight, Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";
import { cn } from "../ui/primitives";

export function FolderRow({
  name,
  count,
  children,
}: {
  name: string;
  count: number;
  children: ReactNode;
}) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);

  return (
    <div>
      <div className="flex flex-wrap items-center gap-3 bg-[var(--surface-2)]/40 px-4 py-2.5">
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((value) => !value)}
          className="flex min-w-0 items-center gap-2 text-left"
        >
          <ChevronRight
            className={cn("size-3.5 shrink-0 text-subtle transition-transform", open && "rotate-90")}
          />
          <span className="grid size-7 shrink-0 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-[var(--accent)]">
            <FolderIcon className="size-3.5" />
          </span>
          <span className="min-w-0 truncate text-[12px] font-medium text-ink">{name}</span>
        </button>
        <span className="font-mono text-[9px] text-subtle">{t("{count} items", { count })}</span>
      </div>
      {open ? (
        <div className="divide-y divide-line/60 border-t border-line/60">
          {count === 0 ? (
            <p className="px-4 py-3 text-[10.5px] text-subtle">{t("No items")}</p>
          ) : (
            children
          )}
        </div>
      ) : null}
    </div>
  );
}
