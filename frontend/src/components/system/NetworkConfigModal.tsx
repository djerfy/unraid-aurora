import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, ArrowDown, ArrowUp, Cable, Check, RotateCcw, Save, ShieldAlert } from "lucide-react";
import { Badge, Button, Switch, cn } from "../ui/primitives";
import { Modal } from "../ui/overlays";
import { useToast } from "../../app/ToastProvider";
import { useI18n } from "../../i18n";
import { ApiError, api } from "../../lib/api";
import type { NetworkConfiguration, NetworkInterfaceConfig } from "../../lib/types";

export function NetworkConfigModal({
  open,
  onClose,
  initialInterface,
}: {
  open: boolean;
  onClose: () => void;
  initialInterface?: string;
}) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<NetworkConfiguration | null>(null);
  const [dnsText, setDnsText] = useState("");
  const [acknowledge, setAcknowledge] = useState(false);
  const [needsAck, setNeedsAck] = useState(false);
  const [savedBackup, setSavedBackup] = useState("");
  const [initialized, setInitialized] = useState(false);
  const [activeTab, setActiveTab] = useState(0);

  const config = useQuery<NetworkConfiguration>({
    queryKey: ["network-config"],
    queryFn: api.networkConfig,
    enabled: open,
  });

  useEffect(() => {
    if (!open) {
      setInitialized(false);
      return;
    }
    if (config.data && !initialized) {
      const requested = initialInterface
        ? config.data.interfaces.findIndex((iface) =>
            [iface.interface, iface.physicalNic, ...iface.bondNics, ...iface.bridgeNics].includes(initialInterface),
          )
        : 0;
      setDraft(config.data);
      setDnsText((config.data.dns ?? []).join(", "));
      setActiveTab(requested >= 0 ? requested : 0);
      setNeedsAck(false);
      setAcknowledge(false);
      setSavedBackup("");
      setInitialized(true);
    }
  }, [open, config.data, initialized, initialInterface]);

  const save = useMutation({
    mutationFn: () => api.saveNetworkConfig({
      ...(draft as NetworkConfiguration),
      dns: dnsText.split(",").map((item) => item.trim()).filter(Boolean),
    }, acknowledge),
    onSuccess: (result) => {
      setDraft(result);
      setDnsText((result.dns ?? []).join(", "));
      setSavedBackup(result.backup ?? "");
      setNeedsAck(false);
      setAcknowledge(false);
      setActiveTab((current) => Math.min(current, Math.max(0, result.interfaces.length - 1)));
      void queryClient.invalidateQueries({ queryKey: ["network-config"] });
      toast({ kind: "success", title: t("Network configuration saved"), description: t("Reboot the server or apply now to use it.") });
    },
    onError: (error: Error) => {
      if (error instanceof ApiError && error.code === "network_lockout_risk") {
        setNeedsAck(true);
        return;
      }
      toast({ kind: "error", title: t("Could not save the network configuration"), description: error.message });
    },
  });

  const apply = useMutation({
    mutationFn: api.applyNetworkConfig,
    onSuccess: () => toast({ kind: "info", title: t("Networking is restarting"), description: t("The connection may drop for a few seconds.") }),
    onError: (error: Error) => toast({ kind: "error", title: t("Could not restart networking"), description: error.message }),
  });

  const revert = useMutation({
    mutationFn: api.revertNetworkConfig,
    onSuccess: async (result) => {
      setSavedBackup("");
      const next = await api.networkConfig();
      setDraft(next);
      setDnsText((next.dns ?? []).join(", "));
      setActiveTab((current) => Math.min(current, Math.max(0, next.interfaces.length - 1)));
      void config.refetch();
      toast({ kind: "success", title: t("Previous network configuration restored"), description: result.backup });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not restore the previous configuration"), description: error.message }),
  });

  const changeState = useMutation({
    mutationFn: ({ name, action }: { name: string; action: "up" | "down" }) => api.setNetworkInterfaceState(name, action),
    onSuccess: (_result, variables) => {
      setDraft((current) => current ? {
        ...current,
        interfaces: current.interfaces.map((iface) => (iface.interface === variables.name ? { ...iface, state: variables.action } : iface)),
      } : current);
      void config.refetch();
      void refreshInterfaceStates();
      if (variables.action === "down") window.setTimeout(() => void refreshInterfaceStates(), 3000);
      toast({
        kind: "success",
        title: t("Interface state changed"),
        description: t("{name} is now {state}", {
          name: variables.name,
          state: variables.action === "up" ? t("Interface up") : t("Interface down"),
        }),
      });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not change the interface state"), description: error.message }),
  });

  const selected = draft?.interfaces[activeTab];

  const stateLabel = (state?: string) =>
    state === "up" ? t("Interface up") : state === "down" ? t("Interface down") : t("Interface unknown");
  const stateTone = (state?: string): "success" | "danger" | "neutral" =>
    state === "up" ? "success" : state === "down" ? "danger" : "neutral";
  const stateDot = (state?: string) =>
    state === "up" ? "bg-[var(--success)]" : state === "down" ? "bg-[var(--danger)]" : "bg-[var(--muted)]";

  const refreshInterfaceStates = async () => {
    try {
      const next = await api.networkConfig();
      const states = new Map(next.interfaces.map((iface) => [iface.index, iface.state]));
      setDraft((current) => current ? {
        ...current,
        interfaces: current.interfaces.map((iface) => ({ ...iface, state: states.get(iface.index) ?? iface.state })),
      } : current);
      queryClient.setQueryData(["network-config"], next);
    } catch {
      // The action already reported its own result; a stale badge is harmless.
    }
  };

  const update = (patch: Partial<NetworkInterfaceConfig>) => {
    setDraft((current) => {
      if (!current) return current;
      return {
        ...current,
        interfaces: current.interfaces.map((iface, index) => (index === activeTab ? { ...iface, ...patch } : iface)),
      };
    });
  };

  const toggleBonding = (value: boolean) => {
    if (!selected) return;
    const defaultBond = selected.index === 0 ? "bond0" : `bond${selected.index}`;
    const bondName = value ? (selected.bondName || defaultBond) : selected.bondName;
    let bridgeNics = selected.bridgeNics;
    if (value) {
      // A bridge over a bond must contain the bond, not its physical members.
      bridgeNics = selected.bridging ? [bondName] : Array.from(new Set([...bridgeNics, bondName]));
    } else {
      bridgeNics = bridgeNics.filter((nic) => nic !== selected.bondName);
    }
    update({ bonding: value, bondName, bridgeNics });
  };

  const toggleBridging = (value: boolean) => {
    if (!selected) return;
    const defaultBridge = selected.index === 0 ? "br0" : `br${selected.index}`;
    const bridgeNics = value && selected.bonding && selected.bondName ? [selected.bondName] : selected.bridgeNics;
    update({
      bridging: value,
      bridgeName: value && selected.bridgeName === "" ? defaultBridge : selected.bridgeName,
      bridgeNics,
    });
  };

  const toggleList = (key: "bondNics" | "bridgeNics", value: string) => {
    if (!selected) return;
    const list = selected[key];
    update({ [key]: list.includes(value) ? list.filter((item) => item !== value) : [...list, value] });
  };

  const options = draft && selected
    ? Array.from(new Set([...(draft.availableNics ?? []), ...(selected.bonding && selected.bondName ? [selected.bondName] : [])])).sort()
    : [];

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("Network configuration")}
      description={t("Edit every interface stored in /boot/config/network.cfg.")}
      size="lg"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{t("Close")}</Button>
          <Button variant="primary" icon={<Save className="size-3.5" />} loading={save.isPending} disabled={!selected} onClick={() => save.mutate()}>
            {t("Save configuration")}
          </Button>
        </>
      }
    >
      {config.isLoading ? <p className="text-[11px] text-muted">{t("Reading the network configuration…")}</p> : null}
      {config.isError ? <p className="text-[11px] text-[var(--danger)]">{(config.error as Error).message}</p> : null}
      {!config.isLoading && !config.isError && draft && draft.interfaces.length === 0 ? (
        <p className="text-[11px] text-[var(--danger)]">{t("No interface is configured on this flash drive.")}</p>
      ) : null}

      {draft && selected ? (
        <div className="space-y-5">
          <div>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
                <Cable className="size-3" /> {t("Configured interfaces")}
              </span>
              <Badge tone="neutral">{draft.format === "indexed" ? t("Unraid 6.12+ format") : t("Legacy format")}</Badge>
            </div>
            <div className="mt-2 flex gap-1 overflow-x-auto rounded-xl border border-line bg-[var(--surface-2)]/50 p-1">
              {draft.interfaces.map((iface, index) => (
                <button
                  key={`${index}-${iface.index}-${iface.interface}`}
                  type="button"
                  onClick={() => setActiveTab(index)}
                  className={cn(
                    "min-w-[118px] shrink-0 rounded-lg px-2.5 py-1.5 text-left transition",
                    index === activeTab
                      ? "bg-[var(--surface)] text-ink shadow-sm ring-1 ring-line"
                      : "text-muted hover:text-ink",
                  )}
                >
                  <span className="flex items-center gap-1.5">
                    <span className={cn("size-1.5 shrink-0 rounded-full", stateDot(iface.state))} title={stateLabel(iface.state)} />
                    <span className="font-mono text-[10.5px] font-medium">{iface.interface}</span>
                    {iface.index === 0 ? <Badge tone="neutral">{t("Primary")}</Badge> : null}
                    {iface.lockoutRisk ? <ShieldAlert className="size-3 text-[var(--warning)]" /> : null}
                  </span>
                  <span className="mt-0.5 block text-[9px] text-subtle">{iface.useDhcp ? "DHCP" : iface.ipAddr || t("No address")}</span>
                </button>
              ))}
            </div>
            {draft.format === "legacy" ? (
              <p className="mt-1.5 text-[9.5px] leading-relaxed text-subtle">
                {t("This flash drive uses the legacy single-interface format. Only the primary interface can be edited.")}
              </p>
            ) : null}
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2">
            <div className="flex min-w-0 items-center gap-2">
              <span className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Interface state")}</span>
              <Badge tone={stateTone(selected.state)} dot>{stateLabel(selected.state)}</Badge>
              <span className="truncate font-mono text-[9px] text-subtle">{selected.interface}</span>
            </div>
            <div className="flex flex-wrap gap-1.5">
              <Button
                size="sm"
                variant="secondary"
                icon={<ArrowUp className="size-3.5" />}
                loading={changeState.isPending && changeState.variables?.action === "up"}
                disabled={selected.state === "up" || changeState.isPending}
                onClick={() => changeState.mutate({ name: selected.interface, action: "up" })}
              >
                {t("Bring up")}
              </Button>
              <Button
                size="sm"
                variant="danger"
                icon={<ArrowDown className="size-3.5" />}
                loading={changeState.isPending && changeState.variables?.action === "down"}
                disabled={selected.state === "down" || changeState.isPending}
                onClick={() => {
                  if (window.confirm(t("Bring {name} down? Your connection may drop.", { name: selected.interface }))) {
                    changeState.mutate({ name: selected.interface, action: "down" });
                  }
                }}
              >
                {t("Bring down")}
              </Button>
            </div>
          </div>

          <div className="rounded-xl border border-[var(--warning-soft)] bg-[var(--warning-soft)] p-3 text-[10px] leading-relaxed text-[var(--warning)]">
            <p className="flex items-start gap-1.5">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
              {t("Changes are written to the flash drive with a backup. They take effect after a reboot unless you choose Apply now.")}
            </p>
          </div>

          <section>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="flex flex-wrap items-center gap-1.5 text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">
                {t("Addressing")} · <span className="font-mono text-[10px]">{selected.interface}</span>
                {selected.physicalNic && selected.physicalNic !== selected.interface ? (
                  <span className="font-mono text-[9px] normal-case tracking-normal text-subtle">({selected.physicalNic})</span>
                ) : null}
              </h3>
              <div className="inline-flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
                {([true, false] as const).map((value) => (
                  <button
                    key={String(value)}
                    type="button"
                    onClick={() => update({ useDhcp: value })}
                    className={cn(
                      "rounded-md px-3 py-1 text-[11px] font-medium transition",
                      selected.useDhcp === value ? "bg-[var(--surface)] text-ink shadow-sm" : "text-muted hover:text-ink",
                    )}
                  >
                    {value ? t("DHCP") : t("Static")}
                  </button>
                ))}
              </div>
            </div>

            {!selected.useDhcp ? (
              <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                <Field label={t("IP address")} value={selected.ipAddr} onChange={(value) => update({ ipAddr: value })} placeholder="192.168.1.10" />
                <Field label={t("Subnet mask")} value={selected.netmask} onChange={(value) => update({ netmask: value })} placeholder="255.255.255.0" />
                <Field label={t("Gateway")} value={selected.gateway} onChange={(value) => update({ gateway: value })} placeholder="192.168.1.1" />
              </div>
            ) : (
              <div className="mt-3 rounded-lg border border-line bg-[var(--surface-2)]/40 p-3 text-[10px] leading-relaxed text-muted">
                {t("This interface gets its address and gateway from DHCP. Static values are kept for switching back.")}
              </div>
            )}
          </section>

          <section>
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="text-[11.5px] font-medium text-soft">{t("Custom MTU")}</h3>
                <p className="text-[9.5px] text-subtle">{t("Only change this when your switch or storage network requires it.")}</p>
              </div>
              <Switch checked={selected.useMtu} onChange={(value) => update({ useMtu: value })} label={t("Custom MTU")} />
            </div>
            {selected.useMtu ? (
              <div className="mt-2 max-w-40">
                <Field label={t("MTU")} value={String(selected.mtu)} onChange={(value) => update({ mtu: Number(value) || 0 })} placeholder="1500" />
              </div>
            ) : null}
          </section>

          <section className="border-t border-line pt-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="text-[11.5px] font-medium text-soft">{t("Bonding")}</h3>
                <p className="text-[9.5px] text-subtle">{t("Combine physical interfaces into one link.")}</p>
              </div>
              <Switch checked={selected.bonding} onChange={toggleBonding} label={t("Bonding")} />
            </div>
            {selected.bonding ? (
              <div className="mt-3 space-y-3">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Field
                    label={t("Bond name")}
                    value={selected.bondName}
                    onChange={(value) => update({
                      bondName: value,
                      bridgeNics: selected.bridgeNics.map((nic) => (nic === selected.bondName ? value : nic)),
                    })}
                    placeholder={selected.index === 0 ? "bond0" : `bond${selected.index}`}
                  />
                  <label className="block">
                    <span className="mb-1 block text-[9.5px] font-medium text-muted">{t("Bond mode")}</span>
                    <select
                      value={selected.bondMode}
                      onChange={(event) => update({ bondMode: event.target.value })}
                      className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
                    >
                      {[
                        ["0", "balance-rr"],
                        ["1", "active-backup"],
                        ["2", "balance-xor"],
                        ["3", "broadcast"],
                        ["4", "802.3ad"],
                        ["5", "balance-tlb"],
                        ["6", "balance-alb"],
                      ].map(([value, label]) => <option key={value} value={value}>{value} · {label}</option>)}
                    </select>
                  </label>
                </div>
                <NicPicker label={t("Bond members")} options={draft.availableNics ?? []} selected={selected.bondNics} onToggle={(value) => toggleList("bondNics", value)} />
              </div>
            ) : null}
          </section>

          <section className="border-t border-line pt-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h3 className="text-[11.5px] font-medium text-soft">{t("Bridging")}</h3>
                <p className="text-[9.5px] text-subtle">{t("Required for Docker and virtual machines on the primary interface.")}</p>
              </div>
              <Switch checked={selected.bridging} onChange={toggleBridging} label={t("Bridging")} />
            </div>
            {selected.bridging ? (
              <div className="mt-3 space-y-3">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Field label={t("Bridge name")} value={selected.bridgeName} onChange={(value) => update({ bridgeName: value })} placeholder={selected.index === 0 ? "br0" : `br${selected.index}`} />
                  <label className="flex items-end gap-2 pb-2 text-[10.5px] text-muted">
                    <Switch checked={selected.bridgeStp} onChange={(value) => update({ bridgeStp: value })} label={t("Spanning tree")} />
                    {t("Spanning tree protocol")}
                  </label>
                </div>
                <NicPicker label={t("Bridge members")} options={options} selected={selected.bridgeNics} onToggle={(value) => toggleList("bridgeNics", value)} />
              </div>
            ) : null}
          </section>

          <section className="border-t border-line pt-4">
            <h3 className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Global DNS")}</h3>
            <p className="mt-0.5 text-[9.5px] text-subtle">{t("Shared by every interface on this flash drive.")}</p>
            <div className="mt-2 max-w-xl">
              <Field label={t("DNS servers")} value={dnsText} onChange={setDnsText} placeholder="1.1.1.1, 8.8.8.8" />
            </div>
          </section>

          {needsAck ? (
            <div className="rounded-xl border border-[var(--danger)]/40 bg-[var(--danger-soft)] p-3">
              <p className="flex items-start gap-1.5 text-[10.5px] font-medium leading-relaxed text-[var(--danger)]">
                <ShieldAlert className="mt-0.5 size-3.5 shrink-0" />
                {t("This change would move an interface outside the subnet of your current connection. You may lose access until you use the physical console or restore the backup.")}
              </p>
              <label className="mt-2 flex items-start gap-2 text-[10px] leading-relaxed text-[var(--danger)]">
                <input type="checkbox" checked={acknowledge} onChange={(event) => setAcknowledge(event.target.checked)} className="mt-0.5 accent-[var(--danger)]" />
                {t("I understand and want to save this configuration anyway.")}
              </label>
            </div>
          ) : null}

          {savedBackup ? (
            <div className="rounded-xl border border-[var(--success-soft)] bg-[var(--success-soft)] p-3">
              <p className="flex items-center gap-1.5 text-[10.5px] font-medium text-[var(--success)]">
                <Check className="size-3.5" /> {t("Saved. Backup: {name}", { name: savedBackup })}
              </p>
              <div className="mt-2 flex flex-wrap gap-2">
                <Button size="sm" variant="primary" onClick={() => {
                  if (window.confirm(t("Restart networking now? Your connection may drop for a few seconds."))) apply.mutate();
                }}>
                  {t("Apply now")}
                </Button>
                <Button size="sm" variant="secondary" icon={<RotateCcw className="size-3" />} loading={revert.isPending} onClick={() => {
                  if (window.confirm(t("Restore the configuration that was active before this change?"))) revert.mutate();
                }}>
                  {t("Revert")}
                </Button>
              </div>
              <p className="mt-2 text-[9px] leading-relaxed text-subtle">{t("Network settings are also applied on the next reboot.")}</p>
            </div>
          ) : null}
        </div>
      ) : null}
    </Modal>
  );
}

function Field({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder?: string }) {
  return (
    <label className="block min-w-0">
      <span className="mb-1 block text-[9.5px] font-medium text-muted">{label}</span>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-9 w-full min-w-0 rounded-[7px] border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
      />
    </label>
  );
}

function NicPicker({ label, options, selected, onToggle }: { label: string; options: string[]; selected: string[]; onToggle: (value: string) => void }) {
  return (
    <div>
      <p className="mb-1.5 text-[9.5px] font-medium text-muted">{label}</p>
      <div className="flex flex-wrap gap-1.5">
        {options.map((option) => (
          <button
            key={option}
            type="button"
            onClick={() => onToggle(option)}
            className={cn(
              "rounded-lg border px-2.5 py-1 font-mono text-[10px] transition",
              selected.includes(option)
                ? "border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)]"
                : "border-line bg-[var(--surface-2)] text-muted hover:text-ink",
            )}
          >
            {option}
          </button>
        ))}
        {options.length === 0 ? <span className="text-[10px] text-subtle">—</span> : null}
      </div>
    </div>
  );
}
