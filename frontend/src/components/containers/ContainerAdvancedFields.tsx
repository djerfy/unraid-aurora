import { Plus, ShieldAlert, Trash2 } from "lucide-react";
import type { ContainerAdvanced } from "../../lib/types";
import { useI18n } from "../../i18n";
import { Button, IconButton, Switch } from "../ui/primitives";
import { SectionHeader, TextField } from "./container-form";
import { CPUPinning } from "../system/CPUPinning";

export function normalizeContainerAdvanced(value?: Partial<ContainerAdvanced> | null): ContainerAdvanced {
  return {
    ...emptyContainerAdvanced,
    ...value,
    devices: value?.devices ?? [],
    capAdd: value?.capAdd ?? [],
    capDrop: value?.capDrop ?? [],
    sysctls: value?.sysctls ?? {},
    ulimits: value?.ulimits ?? [],
    labels: value?.labels ?? {},
    extraHosts: value?.extraHosts ?? [],
    tmpfs: value?.tmpfs ?? [],
    dns: value?.dns ?? [],
    dnsSearch: value?.dnsSearch ?? [],
    memoryLimit: value?.memoryLimit ?? 0,
    nanoCpus: value?.nanoCpus ?? 0,
    pidsLimit: value?.pidsLimit ?? 0,
    readOnlyRootfs: value?.readOnlyRootfs ?? false,
    macAddress: value?.macAddress ?? "",
    hostname: value?.hostname ?? "",
    cpuSet: value?.cpuSet ?? "",
  };
}

export const emptyContainerAdvanced: ContainerAdvanced = {
  macAddress: "",
  hostname: "",
  devices: [],
  capAdd: [],
  capDrop: [],
  sysctls: {},
  ulimits: [],
  labels: {},
  extraHosts: [],
  tmpfs: [],
  dns: [],
  dnsSearch: [],
  memoryLimit: 0,
  nanoCpus: 0,
  cpuSet: "",
  pidsLimit: 0,
  readOnlyRootfs: false,
};

function listText(values: string[]): string {
  return (values ?? []).join("\n");
}

function parseList(value: string): string[] {
  return value.split(/[\n,]+/).map((item) => item.trim()).filter(Boolean);
}

function mapText(values: Record<string, string>): string {
  return Object.entries(values ?? {}).map(([key, value]) => `${key}=${value}`).join("\n");
}

function parseMap(value: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of value.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const index = trimmed.indexOf("=");
    if (index <= 0) continue;
    result[trimmed.slice(0, index).trim()] = trimmed.slice(index + 1).trim();
  }
  return result;
}

export function ContainerAdvancedFields({
  value: input,
  onChange,
  privileged,
}: {
  value?: Partial<ContainerAdvanced> | null;
  onChange: (value: ContainerAdvanced) => void;
  privileged?: boolean;
}) {
  const { t } = useI18n();
  const value = normalizeContainerAdvanced(input);
  const update = (patch: Partial<ContainerAdvanced>) => onChange({ ...value, ...patch });
  const updateDevice = (index: number, patch: Partial<ContainerAdvanced["devices"][number]>) =>
    update({ devices: value.devices.map((device, i) => (i === index ? { ...device, ...patch } : device)) });
  const updateUlimit = (index: number, patch: Partial<ContainerAdvanced["ulimits"][number]>) =>
    update({ ulimits: value.ulimits.map((ulimit, i) => (i === index ? { ...ulimit, ...patch } : ulimit)) });
  const dangerous = privileged || value.devices.length > 0 || value.capAdd.length > 0;

  return (
    <section>
      <SectionHeader
        title={t("Advanced")}
        hint={t("Devices, capabilities and resource limits. Empty fields keep Docker defaults.")}
      />
      {dangerous ? (
        <div className="mb-3 flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
          <ShieldAlert className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
          <span>{t("This container receives extra host access. Review every device and capability before applying.")}</span>
        </div>
      ) : null}

      <div className="grid gap-2 sm:grid-cols-2">
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Hostname")}</span>
          <TextField label={t("Hostname")} value={value.hostname} placeholder="plex" onChange={(hostname) => update({ hostname })} />
        </label>
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("MAC address")}</span>
          <TextField label={t("MAC address")} value={value.macAddress} placeholder="02:42:ac:11:00:02" onChange={(macAddress) => update({ macAddress })} />
        </label>
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Memory limit (MiB)")}</span>
          <input
            type="number"
            min={0}
            value={value.memoryLimit ? Math.round(value.memoryLimit / (1 << 20)) : ""}
            placeholder={t("Unlimited")}
            aria-label={t("Memory limit (MiB)")}
            onChange={(event) => update({ memoryLimit: Math.max(0, Number(event.target.value)) * (1 << 20) })}
            className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
          />
        </label>
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("CPU limit (cores)")}</span>
          <input
            type="number"
            min={0}
            step="0.1"
            value={value.nanoCpus ? value.nanoCpus / 1_000_000_000 : ""}
            placeholder={t("Unlimited")}
            aria-label={t("CPU limit (cores)")}
            onChange={(event) => update({ nanoCpus: Math.max(0, Number(event.target.value)) * 1_000_000_000 })}
            className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
          />
        </label>
        <label className="space-y-1">
          <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("PID limit")}</span>
          <input
            type="number"
            min={0}
            value={value.pidsLimit || ""}
            placeholder={t("Unlimited")}
            aria-label={t("PID limit")}
            onChange={(event) => update({ pidsLimit: Math.max(0, Number(event.target.value)) })}
            className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
          />
        </label>
      </div>

      <div className="mt-2">
        <CPUPinning value={value.cpuSet} onChange={(cpuSet) => update({ cpuSet })} />
      </div>

      <label className="mt-2 flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
        <span>
          <span className="block text-[11px] font-medium text-soft">{t("Read-only root filesystem")}</span>
          <span className="block text-[9.5px] text-subtle">{t("Prevents writes outside mounted paths.")}</span>
        </span>
        <Switch checked={value.readOnlyRootfs} onChange={(readOnlyRootfs) => update({ readOnlyRootfs })} label={t("Read-only root filesystem")} />
      </label>

      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        <div>
          <div className="mb-1.5 flex items-end justify-between gap-2">
            <span className="text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Devices")}</span>
            <Button size="sm" variant="ghost" icon={<Plus className="size-3" />} onClick={() => update({ devices: [...value.devices, { hostPath: "", containerPath: "", permissions: "rwm" }] })}>
              {t("Add device")}
            </Button>
          </div>
          {value.devices.length === 0 ? (
            <p className="rounded-lg border border-dashed border-line px-3 py-2 text-[10px] text-subtle">{t("No host devices.")}</p>
          ) : (
            <div className="space-y-2">
              {value.devices.map((device, index) => (
                <div key={index} className="grid grid-cols-[1fr_1fr_70px_auto] items-center gap-1.5">
                  <TextField label={t("Host device")} value={device.hostPath} placeholder="/dev/dri" onChange={(hostPath) => updateDevice(index, { hostPath })} />
                  <TextField label={t("Container device")} value={device.containerPath} placeholder="/dev/dri" onChange={(containerPath) => updateDevice(index, { containerPath })} />
                  <TextField label={t("Permissions")} value={device.permissions} placeholder="rwm" onChange={(permissions) => updateDevice(index, { permissions })} />
                  <IconButton label={t("Remove device")} size="sm" onClick={() => update({ devices: value.devices.filter((_, i) => i !== index) })}>
                    <Trash2 className="size-3.5" />
                  </IconButton>
                </div>
              ))}
            </div>
          )}
        </div>

        <div>
          <div className="mb-1.5 flex items-end justify-between gap-2">
            <span className="text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Ulimits")}</span>
            <Button size="sm" variant="ghost" icon={<Plus className="size-3" />} onClick={() => update({ ulimits: [...value.ulimits, { name: "nofile", soft: 1024, hard: 2048 }] })}>
              {t("Add ulimit")}
            </Button>
          </div>
          {value.ulimits.length === 0 ? (
            <p className="rounded-lg border border-dashed border-line px-3 py-2 text-[10px] text-subtle">{t("No custom ulimits.")}</p>
          ) : (
            <div className="space-y-2">
              {value.ulimits.map((ulimit, index) => (
                <div key={index} className="grid grid-cols-[1fr_80px_80px_auto] items-center gap-1.5">
                  <TextField label={t("Name")} value={ulimit.name} placeholder="nofile" onChange={(name) => updateUlimit(index, { name })} />
                  <input type="number" min={0} value={ulimit.soft} aria-label={t("Soft")} onChange={(event) => updateUlimit(index, { soft: Number(event.target.value) })} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]" />
                  <input type="number" min={0} value={ulimit.hard} aria-label={t("Hard")} onChange={(event) => updateUlimit(index, { hard: Number(event.target.value) })} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]" />
                  <IconButton label={t("Remove ulimit")} size="sm" onClick={() => update({ ulimits: value.ulimits.filter((_, i) => i !== index) })}>
                    <Trash2 className="size-3.5" />
                  </IconButton>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        <TextArea label={t("Capabilities to add")} value={listText(value.capAdd)} placeholder={"NET_ADMIN\nSYS_TIME"} onChange={(text) => update({ capAdd: parseList(text) })} />
        <TextArea label={t("Capabilities to drop")} value={listText(value.capDrop)} placeholder="NET_RAW" onChange={(text) => update({ capDrop: parseList(text) })} />
        <TextArea label={t("Sysctls")} value={mapText(value.sysctls)} placeholder="net.ipv4.ip_forward=1" onChange={(text) => update({ sysctls: parseMap(text) })} />
        <TextArea label={t("Labels")} value={mapText(value.labels)} placeholder="com.example.role=media" onChange={(text) => update({ labels: parseMap(text) })} />
        <TextArea label={t("Extra hosts")} value={listText(value.extraHosts)} placeholder="nas.local:192.168.1.10" onChange={(text) => update({ extraHosts: parseList(text) })} />
        <TextArea label={t("Tmpfs mounts")} value={listText(value.tmpfs)} placeholder="/tmp:rw,size=64m" onChange={(text) => update({ tmpfs: parseList(text) })} />
        <TextArea label={t("DNS servers")} value={listText(value.dns)} placeholder="1.1.1.1" onChange={(text) => update({ dns: parseList(text) })} />
        <TextArea label={t("DNS search domains")} value={listText(value.dnsSearch)} placeholder="lan" onChange={(text) => update({ dnsSearch: parseList(text) })} />
      </div>
    </section>
  );
}

function TextArea({
  label,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="space-y-1">
      <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{label}</span>
      <textarea
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        rows={3}
        className="w-full resize-y rounded-lg border border-line bg-[var(--surface-2)] px-2.5 py-2 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
      />
    </label>
  );
}
