import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Cpu, FolderOpen, HardDrive, Plus, Search, ShieldAlert, Trash2, Usb } from "lucide-react";
import { Modal } from "../ui/overlays";
import { Badge, Button, IconButton } from "../ui/primitives";
import { CPUPinning } from "../system/CPUPinning";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { api } from "../../lib/api";
import type { VM, VMAdvancedConfig, VMHostDevice } from "../../lib/types";

function deviceKey(device: { type: string; address?: string; vendor?: string; product?: string }): string {
  return device.type === "pci" ? `pci:${device.address ?? ""}` : `usb:${device.vendor ?? ""}:${device.product ?? ""}`;
}

export function VMAdvancedModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<VMAdvancedConfig | null>(null);
  const [search, setSearch] = useState("");
  const [error, setError] = useState("");

  const advanced = useQuery({
    queryKey: ["vm-advanced", vm?.id],
    queryFn: () => api.vmAdvanced(vm?.id ?? ""),
    enabled: Boolean(vm),
    staleTime: 0,
  });
  const hostDevices = useQuery({
    queryKey: ["vm-host-devices"],
    queryFn: api.vmHostDevices,
    enabled: Boolean(vm),
    staleTime: 60_000,
  });

  useEffect(() => {
    if (!advanced.data) return;
    setDraft({
      ...advanced.data,
      networks: advanced.data.networks.length ? advanced.data.networks.map((network) => ({ ...network })) : [{ bridge: "br0", model: "virtio", mac: "" }],
      hostDevices: (advanced.data.hostDevices ?? []).map((device) => ({ ...device })),
      sharedFolders: (advanced.data.sharedFolders ?? []).map((folder) => ({ ...folder })),
    });
    setError("");
  }, [advanced.data]);

  const save = useMutation({
    mutationFn: (config: VMAdvancedConfig) => api.updateVMAdvanced(vm?.id ?? "", config),
    onSuccess: () => {
      toast({ kind: "success", title: t("Hardware saved"), description: t("Changes apply to the next boot.") });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["vm-advanced", vm?.id] });
      onClose();
    },
    onError: (value: Error) => {
      setError(value.message);
      toast({ kind: "error", title: t("Could not save hardware"), description: value.message });
    },
  });

  const selectedKeys = useMemo(() => new Set((draft?.hostDevices ?? []).map(deviceKey)), [draft?.hostDevices]);
  const filteredDevices = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return (hostDevices.data ?? []).filter((device) => {
      if (!needle) return true;
      return `${device.name} ${device.address ?? ""} ${device.vendor ?? ""} ${device.product ?? ""} ${device.iommuGroup ?? ""}`.toLowerCase().includes(needle);
    });
  }, [hostDevices.data, search]);

  const update = (patch: Partial<VMAdvancedConfig>) => {
    setDraft((current) => current ? { ...current, ...patch } : current);
    setError("");
  };
  const toggleDevice = (device: VMHostDevice) => {
    if (!draft) return;
    const key = deviceKey(device);
    if (selectedKeys.has(key)) {
      update({ hostDevices: draft.hostDevices.filter((item) => deviceKey(item) !== key) });
    } else {
      update({
        hostDevices: [...draft.hostDevices, {
          type: device.type, address: device.address, vendor: device.vendor, product: device.product,
        }],
      });
    }
  };

  return (
    <Modal
      open={Boolean(vm)}
      onClose={onClose}
      title={t("Hardware · {name}", { name: vm?.name ?? "" })}
      description={t("Edit CPU, memory, network, sound and PCI/USB passthrough. The libvirt XML is rewritten and changes apply at the next boot.")}
      size="xl"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={save.isPending}>{t("Cancel")}</Button>
          <Button variant="primary" icon={<HardDrive className="size-3.5" />} loading={save.isPending} disabled={!draft} onClick={() => draft && save.mutate(draft)}>
            {t("Save hardware")}
          </Button>
        </>
      }
    >
      {advanced.isLoading ? <p className="text-[11px] text-muted">{t("Reading domain XML…")}</p> : advanced.isError ? (
        <p className="rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-[10.5px] text-[var(--danger)]">{(advanced.error as Error).message}</p>
      ) : draft ? (
        <div className="space-y-5">
          {error ? <p className="rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-[10.5px] text-[var(--danger)]">{error}</p> : null}
          <div className="flex items-start gap-2 rounded-xl border border-[var(--warning)]/30 bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
            <ShieldAlert className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
            <span>{t("Only assign devices you can dedicate to this VM. Reusing a device already bound to another VM or the host can fail at boot.")}</span>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <NumericField label={t("vCPUs")} value={draft.vcpus} min={1} max={128} onChange={(vcpus) => update({ vcpus })} />
            <NumericField label={t("Memory (MiB)")} value={draft.memoryMB} min={512} max={1_048_576} onChange={(memoryMB) => update({ memoryMB })} />
          </div>

          <CPUPinning value={draft.cpuSet} onChange={(cpuSet) => update({ cpuSet })} />

          <section>
            <div className="mb-2 flex items-end justify-between gap-2">
              <div>
                <h3 className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-soft"><Cpu className="size-3.5" /> {t("Network interfaces")}</h3>
                <p className="mt-0.5 text-[9.5px] text-subtle">{t("Bridge, model and MAC address. Leave the MAC empty to let libvirt generate one. NAT and user interfaces are preserved.")}</p>
              </div>
              <Button size="sm" variant="ghost" icon={<Plus className="size-3" />} onClick={() => update({ networks: [...draft.networks, { bridge: "br0", model: "virtio", mac: "" }] })}>
                {t("Add interface")}
              </Button>
            </div>
            <div className="space-y-2">
              {draft.networks.map((network, index) => (
                <div key={index} className="grid grid-cols-[1fr_120px_1fr_auto] items-center gap-1.5">
                  <input value={network.bridge} onChange={(event) => update({ networks: draft.networks.map((item, i) => i === index ? { ...item, bridge: event.target.value } : item) })} placeholder="br0" aria-label={t("Bridge")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]" />
                  <select value={network.model} onChange={(event) => update({ networks: draft.networks.map((item, i) => i === index ? { ...item, model: event.target.value } : item) })} aria-label={t("Model")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)]">
                    <option value="virtio">virtio</option>
                    <option value="e1000">e1000</option>
                    <option value="rtl8139">rtl8139</option>
                  </select>
                  <input value={network.mac} onChange={(event) => update({ networks: draft.networks.map((item, i) => i === index ? { ...item, mac: event.target.value } : item) })} placeholder={t("Generated")} aria-label={t("MAC address")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]" />
                  <IconButton label={t("Remove interface")} size="sm" disabled={draft.networks.length <= 1} onClick={() => update({ networks: draft.networks.filter((_, i) => i !== index) })}>
                    <Trash2 className="size-3.5" />
                  </IconButton>
                </div>
              ))}
            </div>
          </section>

          <section>
            <label className="flex flex-wrap items-center justify-between gap-2">
              <span className="text-[10.5px] text-soft">{t("Virtual sound card")}</span>
              <select value={draft.soundModel} onChange={(event) => update({ soundModel: event.target.value })} className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
                <option value="">{t("Disabled")}</option>
                <option value="ich9">ich9</option>
                <option value="ich6">ich6</option>
                <option value="ac97">ac97</option>
                <option value="usb-audio">usb-audio</option>
              </select>
            </label>
          </section>

          <section>
            <div className="mb-2 flex items-end justify-between gap-2">
              <div>
                <h3 className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-soft"><FolderOpen className="size-3.5" /> {t("Shared folders (virtiofs)")}</h3>
                <p className="mt-0.5 text-[9.5px] text-subtle">{t("Share a host directory under /mnt with the guest through virtiofs.")}</p>
              </div>
              <Button size="sm" variant="ghost" icon={<Plus className="size-3" />} onClick={() => update({ sharedFolders: [...(draft.sharedFolders ?? []), { source: "/mnt/user/", target: "share", readOnly: false }] })}>
                {t("Add folder")}
              </Button>
            </div>
            <div className="space-y-2">
              {(draft.sharedFolders ?? []).map((folder, index) => (
                <div key={index} className="grid grid-cols-1 gap-1.5 sm:grid-cols-[1fr_150px_auto_auto] sm:items-center">
                  <input value={folder.source} onChange={(event) => update({ sharedFolders: draft.sharedFolders.map((item, i) => i === index ? { ...item, source: event.target.value } : item) })} placeholder="/mnt/user/media" aria-label={t("Source path")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]" />
                  <input value={folder.target} onChange={(event) => update({ sharedFolders: draft.sharedFolders.map((item, i) => i === index ? { ...item, target: event.target.value } : item) })} placeholder="media" aria-label={t("Target tag")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]" />
                  <label className="flex items-center gap-1.5 text-[9.5px] text-muted">
                    <input type="checkbox" checked={folder.readOnly} onChange={(event) => update({ sharedFolders: draft.sharedFolders.map((item, i) => i === index ? { ...item, readOnly: event.target.checked } : item) })} className="size-3.5 accent-[var(--accent)]" />
                    {t("Read-only")}
                  </label>
                  <IconButton label={t("Remove folder")} size="sm" onClick={() => update({ sharedFolders: draft.sharedFolders.filter((_, i) => i !== index) })}>
                    <Trash2 className="size-3.5" />
                  </IconButton>
                </div>
              ))}
            </div>
          </section>

          <section>
            <div className="mb-2 flex flex-wrap items-end justify-between gap-2">
              <div>
                <h3 className="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-soft"><Usb className="size-3.5" /> {t("PCI and USB passthrough")}</h3>
                <p className="mt-0.5 text-[9.5px] text-subtle">{t("Devices are grouped by IOMMU where available. Prefer whole IOMMU groups for reliable passthrough.")}</p>
              </div>
              <div className="flex h-8 items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)] px-2">
                <Search className="size-3 text-subtle" />
                <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("Filter devices…")} className="w-40 bg-transparent text-[10.5px] text-ink outline-none placeholder:text-subtle" />
              </div>
            </div>
            <div className="max-h-72 overflow-y-auto rounded-xl border border-line">
              {hostDevices.isLoading ? <p className="px-3 py-3 text-[10px] text-muted">{t("Discovering host devices…")}</p> : filteredDevices.length === 0 ? (
                <p className="px-3 py-3 text-[10px] text-subtle">{t("No matching host device.")}</p>
              ) : filteredDevices.map((device) => {
                const selected = selectedKeys.has(deviceKey(device));
                return (
                  <button key={deviceKey(device)} type="button" onClick={() => toggleDevice(device)} className={`flex w-full items-center gap-3 border-b border-line/60 px-3 py-2 text-left transition last:border-0 ${selected ? "bg-[var(--accent-soft)]" : "hover:bg-[var(--surface-2)]/45"}`}>
                    <span className={`grid size-4 place-items-center rounded border ${selected ? "border-[var(--accent)] bg-[var(--accent)] text-[var(--accent-contrast)]" : "border-line"}`}>{selected ? "✓" : ""}</span>
                    <span className="min-w-0 flex-1">
                      <span className="flex flex-wrap items-center gap-1.5">
                        <span className="truncate text-[10.5px] font-medium text-soft">{device.name || device.address || `${device.vendor}:${device.product}`}</span>
                        <Badge tone={device.type === "pci" ? "info" : "violet"}>{device.type.toUpperCase()}</Badge>
                        {device.iommuGroup ? <span className="text-[8.5px] text-subtle">IOMMU {device.iommuGroup}</span> : null}
                      </span>
                      <span className="mt-0.5 block truncate font-mono text-[8.5px] text-subtle">{device.address || `${device.vendor}:${device.product}`}{device.driver ? ` · ${device.driver}` : ""}</span>
                    </span>
                  </button>
                );
              })}
            </div>
          </section>
        </div>
      ) : null}
    </Modal>
  );
}

function NumericField({ label, value, min, max, onChange }: { label: string; value: number; min: number; max: number; onChange: (value: number) => void }) {
  return (
    <label className="space-y-1">
      <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{label}</span>
      <input type="number" min={min} max={max} value={value} onChange={(event) => onChange(Number(event.target.value))} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]" />
    </label>
  );
}
