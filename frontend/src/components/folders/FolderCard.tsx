import { Folder as FolderIcon } from "lucide-react";
import { useI18n } from "../../i18n";

export function FolderCard({
  name,
  count,
  names,
  onClick,
}: {
  name: string;
  count: number;
  names: string[];
  onClick: () => void;
}) {
  const { t } = useI18n();
  return (
    <button
      type="button"
      onClick={onClick}
      className="card card-hover group flex min-h-[150px] flex-col items-start p-4 text-left"
      aria-label={t("Open folder {name}", { name })}
    >
      <span className="grid size-9 place-items-center rounded-[9px] border border-[var(--border-accent)] bg-[var(--accent-soft)] text-[var(--accent)]">
        <FolderIcon className="size-4" />
      </span>
      <span className="mt-3 w-full truncate text-[13px] font-semibold text-ink transition-colors group-hover:text-[var(--accent)]">
        {name}
      </span>
      <span className="mt-1 font-mono text-[9.5px] text-subtle">{t("{count} items", { count })}</span>
      {names.length ? (
        <span className="mt-2 flex flex-wrap gap-1">
          {names.slice(0, 3).map((item) => (
            <span key={item} className="chip !h-5 !text-[8.5px]">{item}</span>
          ))}
          {count > 3 ? <span className="self-center font-mono text-[8.5px] text-subtle">+{count - 3}</span> : null}
        </span>
      ) : null}
      <span className="mt-auto pt-3 text-[10px] font-medium text-[var(--accent)]">{t("Open folder")} →</span>
    </button>
  );
}
