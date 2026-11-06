import { useEffect, useState } from "react";
import { ArrowDown, ArrowUp, FolderPlus, Trash2 } from "lucide-react";
import { useI18n } from "../../i18n";
import { Button, IconButton } from "../ui/primitives";
import { Modal } from "../ui/overlays";
import { newFolderID } from "./use-visual-folders";
import type { VisualFolder } from "../../lib/types";

export function FolderManager({
  open,
  onClose,
  folders,
  assignments,
  counts,
  onSave,
}: {
  open: boolean;
  onClose: () => void;
  folders: VisualFolder[];
  assignments: Record<string, string>;
  counts: Record<string, number>;
  onSave: (folders: VisualFolder[], assignments: Record<string, string>) => void;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState<VisualFolder[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setDraft(folders.map((folder, order) => ({ ...folder, order })));
    setName("");
    setError("");
  }, [open, folders]);

  const duplicate = (candidate: string, exceptID?: string) =>
    draft.some((folder) => folder.id !== exceptID && folder.name.toLowerCase() === candidate.toLowerCase());

  const add = () => {
    const trimmed = name.trim();
    if (!trimmed) {
      setError(t("Folder name is required."));
      return;
    }
    if (duplicate(trimmed)) {
      setError(t("A folder with this name already exists."));
      return;
    }
    setDraft([...draft, { id: newFolderID(), name: trimmed, order: draft.length }]);
    setName("");
    setError("");
  };

  const move = (index: number, delta: number) => {
    const target = index + delta;
    if (target < 0 || target >= draft.length) return;
    const next = [...draft];
    [next[index], next[target]] = [next[target], next[index]];
    setDraft(next.map((folder, order) => ({ ...folder, order })));
  };

  const save = () => {
    for (const folder of draft) {
      if (!folder.name.trim()) {
        setError(t("Folder name is required."));
        return;
      }
      if (duplicate(folder.name, folder.id)) {
        setError(t("Folder names must be unique."));
        return;
      }
      if (folder.name.trim().length > 64) {
        setError(t("Folder names must be 64 characters or fewer."));
        return;
      }
    }
    const ids = new Set(draft.map((folder) => folder.id));
    const kept = Object.fromEntries(
      Object.entries(assignments).filter(([, folderID]) => ids.has(folderID)),
    );
    onSave(
      draft.map((folder, order) => ({ ...folder, name: folder.name.trim(), order })),
      kept,
    );
    onClose();
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("Folders")}
      description={t("Visual organization only. Containers and VMs are not moved on disk.")}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>{t("Cancel")}</Button>
          <Button variant="primary" onClick={save}>{t("Save folders")}</Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="flex items-end gap-2">
          <label className="min-w-0 flex-1">
            <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("New folder")}</span>
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
              }}
              placeholder={t("Folder name")}
              className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-ink outline-none focus:border-[var(--accent-line)]"
            />
          </label>
          <Button size="sm" variant="secondary" icon={<FolderPlus className="size-3.5" />} onClick={add}>
            {t("Add")}
          </Button>
        </div>

        {draft.length === 0 ? (
          <p className="rounded-xl border border-line bg-[var(--surface-2)]/50 px-3 py-4 text-center text-[11px] text-muted">
            {t("No folders yet.")}
          </p>
        ) : (
          <ol className="space-y-1.5">
            {draft.map((folder, index) => (
              <li key={folder.id} className="flex items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)]/50 px-2.5 py-2">
                <span className="w-5 shrink-0 text-center font-mono text-[10px] text-subtle">{index + 1}</span>
                <input
                  value={folder.name}
                  onChange={(event) => {
                    setDraft(draft.map((item) => (item.id === folder.id ? { ...item, name: event.target.value } : item)));
                    setError("");
                  }}
                  aria-label={t("Folder name")}
                  className="h-7 min-w-0 flex-1 rounded-md border border-transparent bg-transparent px-1.5 text-[11.5px] text-ink outline-none hover:border-line focus:border-[var(--accent-line)]"
                />
                <span className="shrink-0 font-mono text-[9px] text-subtle">
                  {t("{count} items", { count: counts[folder.id] ?? 0 })}
                </span>
                <IconButton label={t("Move up")} size="sm" disabled={index === 0} onClick={() => move(index, -1)}>
                  <ArrowUp className="size-3" />
                </IconButton>
                <IconButton label={t("Move down")} size="sm" disabled={index === draft.length - 1} onClick={() => move(index, 1)}>
                  <ArrowDown className="size-3" />
                </IconButton>
                <IconButton label={t("Delete folder")} size="sm" onClick={() => setDraft(draft.filter((item) => item.id !== folder.id))}>
                  <Trash2 className="size-3" />
                </IconButton>
              </li>
            ))}
          </ol>
        )}

        <p className="text-[10.5px] leading-relaxed text-subtle">
          {t("Deleting a folder moves its items to Unfiled. Renaming is purely visual.")}
        </p>
        {error ? <p className="text-[10.5px] text-[var(--danger)]">{error}</p> : null}
      </div>
    </Modal>
  );
}
