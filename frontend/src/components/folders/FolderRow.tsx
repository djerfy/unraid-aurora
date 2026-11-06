import { Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";

export function FolderRow({
  name,
  count,
  items,
  onOpen,
  onOpenItem,
  openLabel,
}: {
  name: string;
  count: number;
  items: { id: string; name: string }[];
  onOpen: () => void;
  onOpenItem: (id: string) => void;
  openLabel: string;
}) {
  const { t } = useI18n();
  return (
    <div className="flex flex-wrap items-center gap-3 bg-[var(--surface-2)]/40 px-4 py-2.5">
      <span className="grid size-7 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-[var(--accent)]">
        <FolderIcon className="size-3.5" />
      </span>
      <button
        type="button"
        onClick={onOpen}
        className="min-w-0 truncate text-[12px] font-medium text-ink transition-colors hover:text-[var(--accent)]"
      >
        {name}
      </button>
      <span className="font-mono text-[9px] text-subtle">{t("{count} items", { count })}</span>
      <select
        value=""
        onChange={(event) => {
          if (event.target.value) onOpenItem(event.target.value);
        }}
        disabled={items.length === 0}
        aria-label={openLabel}
        className="ml-auto h-8 max-w-[240px] rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)] disabled:opacity-50"
      >
        <option value="">{items.length ? openLabel : t("No items")}</option>
        {items.map((item) => (
          <option key={item.id} value={item.id}>{item.name}</option>
        ))}
      </select>
    </div>
  );
}
