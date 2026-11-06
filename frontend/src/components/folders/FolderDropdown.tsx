import { FolderCog } from "lucide-react";
import { useI18n } from "../../i18n";
import { Button } from "../ui/primitives";
import type { VisualFolder } from "../../lib/types";

export function FolderDropdown({
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
  return (
    <div className="flex items-center gap-1.5">
      <select
        value={selected}
        onChange={(event) => onSelect(event.target.value)}
        aria-label={t("Folder")}
        className="h-8 max-w-[190px] rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
      >
        <option value="all">{t("All folders")} ({total})</option>
        <option value="none">{t("Unfiled")} ({ungrouped})</option>
        {folders.map((folder) => (
          <option key={folder.id} value={folder.id}>
            {folder.name} ({counts[folder.id] ?? 0})
          </option>
        ))}
      </select>
      <Button
        size="sm"
        variant="secondary"
        icon={<FolderCog className="size-3.5" />}
        onClick={onManage}
        className="!px-2.5"
        aria-label={t("Folders")}
        title={t("Folders")}
      >
        <span className="hidden sm:inline">{t("Folders")}</span>
      </Button>
    </div>
  );
}
