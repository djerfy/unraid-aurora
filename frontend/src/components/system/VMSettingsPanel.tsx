import { useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Monitor, RefreshCw, Save } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { Badge, Button, Card, Switch, cn } from "../ui/primitives";
import type { VMSettings } from "../../lib/types";

function Row({
  label,
  description,
  children,
}: {
  label: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 py-3.5 first:pt-3 last:pb-0">
      <div className="min-w-0 max-w-md">
        <p className="text-[11.5px] font-medium text-soft">{label}</p>
        <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">{description}</p>
      </div>
      {children}
    </div>
  );
}

export function VMSettingsPanel() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<VMSettings | null>(null);

  const query = useQuery({
    queryKey: ["vm-settings"],
    queryFn: api.vmSettings,
    staleTime: 0,
  });

  useEffect(() => {
    if (query.data) setDraft(query.data);
  }, [query.data]);

  const save = useMutation({
    mutationFn: (value: VMSettings) => api.saveVMSettings(value),
    onSuccess: (saved) => {
      setDraft(saved);
      queryClient.setQueryData(["vm-settings"], saved);
      toast({ kind: "success", title: t("VM settings saved"), description: t("Restart the array to apply VM service changes.") });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("VM settings could not be saved"), description: error.message });
    },
  });

  if (query.isError) {
    return <Card className="card-pad"><p className="text-[10.5px] text-[var(--danger)]">{(query.error as Error).message}</p></Card>;
  }
  if (query.isLoading || !draft) {
    return <Card className="card-pad"><p className="py-8 text-center text-[10.5px] text-muted">{t("Loading VM settings…")}</p></Card>;
  }

  const update = (patch: Partial<VMSettings>) => setDraft((current) => (current ? { ...current, ...patch } : current));

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <Monitor className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("VM Manager")}</h2>
              <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                {t("Unraid libvirt settings from /boot/config/domain.cfg. Service changes apply after the array restarts.")}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={draft.running ? "success" : "neutral"} dot>{draft.running ? t("Running") : t("Stopped")}</Badge>
            <Badge tone={draft.enabled ? "success" : "neutral"}>{draft.enabled ? t("VMs enabled") : t("VMs disabled")}</Badge>
            <Button size="sm" variant="secondary" icon={<RefreshCw className={cn("size-3.5", query.isFetching && "animate-spin")} />} onClick={() => void query.refetch()}>
              {t("Refresh")}
            </Button>
            <Button size="sm" variant="primary" icon={<Save className="size-3.5" />} loading={save.isPending} onClick={() => save.mutate(draft)}>
              {t("Save VM settings")}
            </Button>
          </div>
        </div>
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Service")}</h2>
        <div className="mt-2 divide-y divide-line">
          <Row label={t("Enable VMs")} description={t("Start the libvirt service with the array.")}>
            <Switch checked={draft.enabled} onChange={(enabled) => update({ enabled })} label={t("Enable VMs")} />
          </Row>
          <Row label={t("Disable Autostart/Start option for VMs")} description={t("Hide the autostart and start options for every virtual machine.")}>
            <Switch checked={draft.disableAutostart} onChange={(disableAutostart) => update({ disableAutostart })} label={t("Disable Autostart/Start option for VMs")} />
          </Row>
        </div>
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Windows drivers")}</h2>
        <div className="mt-2 divide-y divide-line">
          <Row label={t("Default Windows VirtIO driver ISO")} description={t("Optional path to the virtio-win ISO mounted when creating Windows VMs.")}>
            <input
              value={draft.virtioIso}
              onChange={(event) => update({ virtioIso: event.target.value })}
              placeholder="/mnt/user/isos/virtio-win.iso"
              className="h-9 w-80 max-w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
            />
          </Row>
        </div>
        <p className="mt-3 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 py-2 text-[10px] leading-relaxed text-subtle">
          {t("Leave empty to pick the VirtIO ISO manually for each Windows VM. The file must be an .iso under /mnt.")}
        </p>
      </Card>
    </div>
  );
}
