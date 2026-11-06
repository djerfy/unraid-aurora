import { useState } from "react";
import { useI18n } from "../../i18n";
import { Button } from "../ui/primitives";
import type { VisualFolder } from "../../lib/types";

export function FolderPicker({
  folders,
  value,
  onChange,
  onCreate,
  label,
  disabled,
}: {
  folders: VisualFolder[];
  value: string;
  onChange: (folderID: string) => void;
  onCreate: (name: string) => VisualFolder | null;
  label?: string;
  disabled?: boolean;
}) {
  const { t } = useI18n();
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const current = folders.some((folder) => folder.id === value) ? value : "";

  const add = () => {
    const folder = onCreate(name);
    if (!folder) {
      setError(t("Folder name is required."));
      return;
    }
    onChange(folder.id);
    setCreating(false);
    setName("");
    setError("");
  };

  return (
    <div>
      {label ? (
        <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{label}</span>
      ) : null}
      <select
        value={creating ? "__new" : current}
        aria-label={label ?? t("Folder")}
        disabled={disabled}
        onChange={(event) => {
          if (event.target.value === "__new") {
            setCreating(true);
            setError("");
            return;
          }
          setCreating(false);
          onChange(event.target.value);
        }}
        className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-ink outline-none focus:border-[var(--accent-line)] disabled:opacity-50"
      >
        <option value="">{t("No folder")}</option>
        {folders.map((folder) => (
          <option key={folder.id} value={folder.id}>{folder.name}</option>
        ))}
        <option value="__new">{t("New folder…")}</option>
      </select>
      {creating ? (
        <div className="mt-1.5 flex items-center gap-1.5">
          <input
            value={name}
            onChange={(event) => {
              setName(event.target.value);
              setError("");
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                add();
              }
              if (event.key === "Escape") setCreating(false);
            }}
            placeholder={t("Folder name")}
            aria-label={t("Folder name")}
            autoFocus
            className="h-8 min-w-0 flex-1 rounded-md border border-line bg-[var(--surface-2)] px-2 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
          />
          <Button size="sm" onClick={add}>{t("Add")}</Button>
          <Button size="sm" variant="ghost" onClick={() => setCreating(false)}>{t("Cancel")}</Button>
        </div>
      ) : null}
      {error ? <p className="mt-1 text-[10px] text-[var(--danger)]">{error}</p> : null}
    </div>
  );
}
