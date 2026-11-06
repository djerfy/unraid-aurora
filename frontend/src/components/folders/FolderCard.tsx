import { Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";

export function FolderCard({
  name,
  count,
  names,
}: {
  name: string;
  count: number;
  names: string[];
}) {
  const { t } = useI18n();
  return (
    <div className="card flex min-h-[150px] flex-col items-start p-4">
      <div className="flex w-full items-center gap-2.5">
        <span className="grid size-9 shrink-0 place-items-center rounded-[9px] border border-[var(--border-accent)] bg-[var(--accent-soft)] text-[var(--accent)]">
          <FolderIcon className="size-4" />
        </span>
        <span className="min-w-0">
          <span className="block truncate text-[13px] font-semibold text-ink">{name}</span>
          <span className="font-mono text-[9.5px] text-subtle">{t("{count} items", { count })}</span>
        </span>
      </div>
      <ul className="mt-2.5 max-h-36 w-full space-y-0.5 overflow-y-auto pr-1">
        {names.map((item) => (
          <li key={item} className="truncate rounded px-1 py-0.5 text-[10px] text-muted">
            {item}
          </li>
        ))}
        {count === 0 ? <li className="px-1 py-0.5 text-[10px] text-subtle">{t("No items")}</li> : null}
      </ul>
    </div>
  );
}
