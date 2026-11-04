import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Copy, HardDrive, ShieldAlert } from "lucide-react";
import { Modal } from "../ui/overlays";
import { Button, Switch } from "../ui/primitives";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { api } from "../../lib/api";
import type { VM } from "../../lib/types";

export function VMCloneModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [start, setStart] = useState(false);

  useEffect(() => {
    if (!vm) return;
    setName(`${vm.name}-clone`);
    setStart(false);
  }, [vm]);

  const clone = useMutation({
    mutationFn: () => api.cloneVM(vm?.id ?? "", name.trim(), start),
    onSuccess: () => {
      toast({ kind: "success", title: t("Virtual machine cloned"), description: name.trim() });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      onClose();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not clone the virtual machine"), description: error.message }),
  });

  return (
    <Modal
      open={Boolean(vm)}
      onClose={onClose}
      title={t("Clone {name}", { name: vm?.name ?? "" })}
      description={t("Copy the domain definition, disks and NVRAM into a new libvirt domain. The source VM must be shut down.")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={clone.isPending}>{t("Cancel")}</Button>
          <Button variant="primary" icon={<Copy className="size-3.5" />} loading={clone.isPending} disabled={!name.trim() || vm?.state === "running"} onClick={() => clone.mutate()}>
            {t("Clone virtual machine")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="flex items-start gap-2 rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
          <ShieldAlert className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
          <span>{vm?.state === "running" ? t("Shut down the virtual machine before cloning it.") : t("The clone receives new disk files, a new UUID and new MAC addresses.")}</span>
        </div>
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Clone name")}</span>
          <input value={name} onChange={(event) => setName(event.target.value)} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]" />
        </label>
        <label className="flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
          <span className="text-[10.5px] text-soft">{t("Start the clone after creation")}</span>
          <Switch checked={start} onChange={setStart} label={t("Start the clone after creation")} />
        </label>
      </div>
    </Modal>
  );
}

export function VMResizeModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [sizeGB, setSizeGB] = useState(0);

  useEffect(() => {
    if (!vm) return;
    setSizeGB(Math.max(1, Math.ceil(vm.diskSize / (1 << 30)) + 5));
  }, [vm]);

  const resize = useMutation({
    mutationFn: () => api.resizeVMDisk(vm?.id ?? "", sizeGB),
    onSuccess: () => {
      toast({ kind: "success", title: t("Virtual disk expanded"), description: `${sizeGB} GiB` });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      onClose();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not expand the virtual disk"), description: error.message }),
  });

  const currentGB = vm ? Math.ceil(vm.diskSize / (1 << 30)) : 0;

  return (
    <Modal
      open={Boolean(vm)}
      onClose={onClose}
      title={t("Expand disk · {name}", { name: vm?.name ?? "" })}
      description={t("Expand the first file-backed virtual disk. Shrinking is never allowed.")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={resize.isPending}>{t("Cancel")}</Button>
          <Button variant="primary" icon={<HardDrive className="size-3.5" />} loading={resize.isPending} disabled={sizeGB <= currentGB} onClick={() => resize.mutate()}>
            {t("Expand virtual disk")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <p className="rounded-xl border border-line bg-[var(--surface-2)]/45 p-3 text-[10.5px] text-muted">
          {t("Current capacity: {size} GiB. The guest operating system must still grow its partition after the disk is expanded.", { size: currentGB })}
        </p>
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("New capacity (GiB)")}</span>
          <input type="number" min={currentGB + 1} max={10000} value={sizeGB || ""} onChange={(event) => setSizeGB(Number(event.target.value))} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]" />
        </label>
        {vm?.state === "running" ? <p className="text-[9.5px] text-[var(--warning)]">{t("The disk is resized online with virsh blockresize.")}</p> : <p className="text-[9.5px] text-subtle">{t("The domain is stopped, so the qcow2 file is resized directly.")}</p>}
      </div>
    </Modal>
  );
}
