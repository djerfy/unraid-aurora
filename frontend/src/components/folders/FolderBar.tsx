import { Folder as FolderIcon, FolderCog } from "lucide-react";
import { useI18n } from "../../i18n";
import { Button, cn } from "../ui/primitives";
import type { VisualFolder } from "../../lib/types";

export function FolderBar({
  folders,
  selected,
  onSelect,
  total,
  ungrouped,
  counts,
  onManage,
}: {
  folders: VisualFolder[];
  selected: string;
  onSelect: (id: string) => void;
  total: number;
  ungrouped: number;
  counts: Record<string, number>;
  onManage: () => void;
}) {
  const { t } = useI18n();
  const chip = (active: boolean) =>
    cn(
      "inline-flex shrink-0 items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] transition",
      active
        ? "border-[var(--accent-line)] bg-[var(--accent-softer)] text-ink"
        : "border-line bg-[var(--surface-2)]/50 text-muted hover:border-line-strong hover:text-ink",
    );

  return (
    <div className="mb-4 flex items-center gap-2">
      <div className="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto pb-0.5">
        <button type="button" className={chip(selected === "all")} onClick={() => onSelect("all")}>
          {t("All")}
          <span className="font-mono text-[9px] text-subtle">{total}</span>
        </button>
        <button type="button" className={chip(selected === "none")} onClick={() => onSelect("none")}>
          <FolderIcon className="size-3" />
          {t("Unfiled")}
          <span className="font-mono text-[9px] text-subtle">{ungrouped}</span>
        </button>
        {folders.map((folder) => (
          <button key={folder.id} type="button" className={chip(selected === folder.id)} onClick={() => onSelect(folder.id)}>
            <FolderIcon className="size-3" />
            {folder.name}
            <span className="font-mono text-[9px] text-subtle">{counts[folder.id] ?? 0}</span>
          </button>
        ))}
      </div>
      <Button size="sm" variant="secondary" icon={<FolderCog className="size-3.5" />} onClick={onManage}>
        {t("Folders")}
      </Button>
    </div>
  );
}
