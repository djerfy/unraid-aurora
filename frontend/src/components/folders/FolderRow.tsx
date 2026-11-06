import { useState } from "react";
import { ChevronRight, Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";
import { cn } from "../ui/primitives";

export function FolderRow({
  name,
  count,
  items,
  onOpenItem,
}: {
  name: string;
  count: number;
  items: { id: string; name: string }[];
  onOpenItem: (id: string) => void;
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
        <div className="border-t border-line/60 bg-[var(--surface-2)]/20 px-4 py-2">
          {items.length ? (
            <ul className="space-y-0.5">
              {items.map((item) => (
                <li key={item.id}>
                  <button
                    type="button"
                    onClick={() => onOpenItem(item.id)}
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-[11px] text-muted transition hover:bg-[var(--surface-2)] hover:text-ink"
                  >
                    <span className="size-1 shrink-0 rounded-full bg-[var(--accent)]" />
                    <span className="truncate">{item.name}</span>
                    <ChevronRight className="ml-auto size-3 shrink-0 text-subtle" />
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="px-2 py-1 text-[10.5px] text-subtle">{t("No items")}</p>
          )}
        </div>
      ) : null}
    </div>
  );
}
