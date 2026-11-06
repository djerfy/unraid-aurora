import { useEffect, useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ListOrdered, RotateCcw } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { Modal } from "../ui/overlays";
import { Button } from "../ui/primitives";
import { EntityIcon, containerIconURL } from "../ui/entity-icon";
import type { Container } from "../../lib/types";

export function ContainerPriorityModal({
  open,
  onClose,
  containers,
}: {
  open: boolean;
  onClose: () => void;
  containers: Container[];
}) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<Record<string, number>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    // Snapshot refreshes must not overwrite a draft the user is editing.
    setDraft(Object.fromEntries(containers.map((container) => [container.name, container.priority ?? 50])));
    setError("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const rows = useMemo(
    () => [...containers].sort((a, b) => a.name.localeCompare(b.name)),
    [containers],
  );
  const startOrder = useMemo(() => {
    const ordered = [...containers].sort((a, b) => {
      const left = draft[a.name] ?? 50;
      const right = draft[b.name] ?? 50;
      if (left !== right) return left - right;
      return a.name.localeCompare(b.name);
    });
    return new Map(ordered.map((container, index) => [container.name, index + 1]));
  }, [containers, draft]);

  const save = useMutation({
    mutationFn: () => api.saveContainerPriorities(draft),
    onSuccess: () => {
      toast({ kind: "success", title: t("Startup priorities saved") });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      onClose();
    },
    onError: (err: Error) => setError(err.message),
  });

  const reset = () => {
    setDraft(Object.fromEntries(containers.map((container) => [container.name, 50])));
    setError("");
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("Startup priorities")}
      description={t("Containers with a lower number start first. 0 is the highest priority, 100 the lowest.")}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>{t("Cancel")}</Button>
          <Button variant="primary" loading={save.isPending} onClick={() => save.mutate()}>
            {t("Save priorities")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 px-3 py-2.5">
          <div className="flex items-center gap-2 text-[10.5px] text-muted">
            <ListOrdered className="size-3.5 text-[var(--accent)]" />
            <span>{t("Bulk starts and the Unraid autostart file follow this order.")}</span>
          </div>
          <Button size="sm" variant="ghost" icon={<RotateCcw className="size-3" />} onClick={reset}>
            {t("Reset to 50")}
          </Button>
        </div>

        {rows.length === 0 ? (
          <p className="py-8 text-center text-[11px] text-muted">{t("No containers.")}</p>
        ) : (
          <div className="max-h-[52vh] space-y-1.5 overflow-y-auto pr-1">
            {rows.map((container) => {
              const value = draft[container.name] ?? 50;
              return (
                <div
                  key={container.id}
                  className="flex flex-wrap items-center gap-3 rounded-lg border border-line bg-[var(--surface-2)]/40 px-3 py-2 sm:flex-nowrap"
                >
                  <span className="w-7 shrink-0 text-center font-mono text-[10px] text-subtle">
                    #{startOrder.get(container.name) ?? "—"}
                  </span>
                  <EntityIcon
                    src={containerIconURL(container.id)}
                    name={container.name}
                    size={26}
                    className="rounded-md"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[11.5px] font-medium text-ink">{container.name}</span>
                    <span className="block truncate text-[9px] text-subtle">{container.image}</span>
                  </span>
                  <input
                    type="range"
                    min={0}
                    max={100}
                    step={1}
                    value={value}
                    aria-label={t("Priority for {name}", { name: container.name })}
                    onChange={(event) => {
                      const next = Number(event.target.value);
                      setDraft((current) => ({ ...current, [container.name]: next }));
                      setError("");
                    }}
                    className="h-2 w-full cursor-pointer sm:w-48"
                    style={{ accentColor: "var(--accent)" }}
                  />
                  <span className="w-9 shrink-0 text-right font-mono text-[10.5px] text-ink">{value}</span>
                </div>
              );
            })}
          </div>
        )}

        {error ? <p className="text-[10.5px] text-[var(--danger)]">{error}</p> : null}
      </div>
    </Modal>
  );
}
