import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowDown, ArrowUp, ListOrdered, Play, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { Badge, Button, Switch } from "../ui/primitives";
import { Modal } from "../ui/overlays";
import type { VM, VMAutostartResult } from "../../lib/types";

export function VMAutostartModal({ open, vms, onClose }: { open: boolean; vms: VM[]; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [order, setOrder] = useState<string[]>([]);
  const [managed, setManaged] = useState(false);
  const [selected, setSelected] = useState("");
  const [result, setResult] = useState<VMAutostartResult | null>(null);

  const orderQuery = useQuery({
    queryKey: ["vm-autostart-order"],
    queryFn: api.vmAutostartOrder,
    enabled: open,
  });

  useEffect(() => {
    if (!orderQuery.data) return;
    setOrder(orderQuery.data.order);
    setManaged(orderQuery.data.managed);
  }, [orderQuery.data]);

  const byName = new Map(vms.map((vm) => [vm.name, vm]));
  const available = vms.filter((vm) => !order.includes(vm.name));
  const saved = orderQuery.data;
  const dirty =
    !saved ||
    managed !== saved.managed ||
    order.length !== saved.order.length ||
    order.some((name, index) => name !== saved.order[index]);

  const move = (index: number, delta: number) => {
    const next = [...order];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    setOrder(next);
  };

  const save = useMutation({
    mutationFn: () => api.setVMAutostartOrder(order, managed),
    onSuccess: (data) => {
      setOrder(data.order);
      setManaged(data.managed);
      toast({ kind: "success", title: t("Order saved") });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void orderQuery.refetch();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not save the order"), description: error.message }),
  });

  const apply = useMutation({
    mutationFn: api.applyVMAutostartOrder,
    onSuccess: (data) => {
      setResult(data);
      toast({
        kind: data.failed.length ? "warning" : "success",
        title: t("{count} started", { count: data.started.length }),
        description: data.failed.length ? t("{count} failed", { count: data.failed.length }) : undefined,
      });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not start the VMs"), description: error.message }),
  });

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("Autostart order")}
      description={t("libvirt has no start order. Aurora stores one, displays it, and can take over autostart at array start.")}
      size="lg"
    >
      <div className="space-y-5">
        <section>
          <div className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
            <ListOrdered className="size-3.5" /> {t("Order")}
          </div>
          {order.length === 0 ? (
            <p className="mt-2 text-[11px] text-muted">{t("No virtual machine in this order yet.")}</p>
          ) : (
            <ol className="mt-2 space-y-1.5">
              {order.map((name, index) => {
                const vm = byName.get(name);
                return (
                  <li key={name} className="flex items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)]/50 px-2.5 py-2">
                    <span className="w-5 shrink-0 text-center font-mono text-[10px] text-subtle">{index + 1}</span>
                    <span className="min-w-0 flex-1 truncate text-[11.5px] text-ink">{name}</span>
                    {vm ? (
                      <Badge tone={vm.state === "running" ? "success" : "neutral"} dot>{t(vm.state)}</Badge>
                    ) : null}
                    <button
                      type="button"
                      onClick={() => move(index, -1)}
                      disabled={index === 0}
                      aria-label={t("Move up")}
                      className="grid size-6 place-items-center rounded text-muted hover:bg-[var(--surface-3)] hover:text-ink disabled:opacity-25"
                    >
                      <ArrowUp className="size-3" />
                    </button>
                    <button
                      type="button"
                      onClick={() => move(index, 1)}
                      disabled={index === order.length - 1}
                      aria-label={t("Move down")}
                      className="grid size-6 place-items-center rounded text-muted hover:bg-[var(--surface-3)] hover:text-ink disabled:opacity-25"
                    >
                      <ArrowDown className="size-3" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setOrder(order.filter((item) => item !== name))}
                      aria-label={t("Remove")}
                      className="grid size-6 place-items-center rounded text-muted hover:bg-[var(--danger-soft)] hover:text-[var(--danger)]"
                    >
                      <Trash2 className="size-3" />
                    </button>
                  </li>
                );
              })}
            </ol>
          )}

          {available.length ? (
            <div className="mt-2 flex gap-2">
              <select
                value={selected}
                onChange={(event) => setSelected(event.target.value)}
                aria-label={t("Add a virtual machine")}
                className="h-8 min-w-0 flex-1 rounded-md border border-line bg-[var(--surface-2)] px-2 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
              >
                <option value="">{t("Add a virtual machine")}</option>
                {available.map((vm) => (
                  <option key={vm.name} value={vm.name}>{vm.name}</option>
                ))}
              </select>
              <Button
                size="sm"
                variant="secondary"
                disabled={!selected}
                onClick={() => {
                  setOrder([...order, selected]);
                  setSelected("");
                }}
              >
                {t("Add")}
              </Button>
            </div>
          ) : null}
        </section>

        <section className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
          <div className="flex items-start justify-between gap-3">
            <span>
              <span className="block text-[11.5px] font-medium text-soft">{t("Manage autostart at array start")}</span>
              <span className="mt-0.5 block text-[10px] leading-relaxed text-subtle">
                {t("Aurora disables libvirt autostart for these VMs and starts them in order after the array starts.")}
              </span>
            </span>
            <Switch checked={managed} onChange={setManaged} label={t("Manage autostart at array start")} />
          </div>
          {managed ? (
            <p className="mt-2 text-[10px] leading-relaxed text-[var(--warning)]">
              {t("Unchecking this restores libvirt autostart for the machines Aurora took over.")}
            </p>
          ) : null}
        </section>

        {result ? (
          <section className="space-y-1.5 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px]">
            <p className="text-[var(--success)]">{t("{count} started", { count: result.started.length })}</p>
            <p className="text-subtle">{t("{count} skipped", { count: result.skipped.length })}</p>
            {result.failed.map((failure) => (
              <p key={failure.name} className="text-[var(--danger)]">{failure.name}: {failure.error}</p>
            ))}
          </section>
        ) : null}

        {dirty ? <p className="text-[10px] text-[var(--warning)]">{t("Save the order before starting the VMs.")}</p> : null}

        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="secondary"
            icon={<Play className="size-3.5" />}
            disabled={apply.isPending || order.length === 0 || dirty}
            loading={apply.isPending}
            onClick={() => apply.mutate()}
          >
            {t("Start in order")}
          </Button>
          <Button variant="primary" disabled={save.isPending} loading={save.isPending} onClick={() => save.mutate()}>
            {t("Save order")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
