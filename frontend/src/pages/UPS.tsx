import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BatteryCharging, Save, ShieldAlert, Zap } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Button, Card, Progress } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useI18n } from "../i18n";
import { useToast } from "../app/ToastProvider";
import { api } from "../lib/api";

import type { UPSSettings } from "../lib/types";

function runtimeLabel(minutes: number) {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return `${hours}h ${String(rest).padStart(2, "0")}m`;
}

export function UPSPage() {
  const { snapshot } = useApp();
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ["ups-settings"], queryFn: api.upsSettings });
  const [draft, setDraft] = useState<UPSSettings | null>(null);

  useEffect(() => {
    if (settings.data) setDraft(settings.data);
  }, [settings.data]);

  const save = useMutation({
    mutationFn: () => api.saveUPSSettings(draft as UPSSettings),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["ups-settings"] });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({ kind: "success", title: t("UPS settings saved") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not save UPS settings"), description: error.message }),
  });

  const ups = snapshot?.ups;
  const field = "h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-ink outline-none focus:border-[var(--accent-line)]";
  const label = "mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle";

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("UPS")}
        description={t("Battery status and the built-in apcupsd shutdown policy.")}
        meta={
          <>
            <Badge tone={ups?.present ? "success" : "neutral"} dot>{ups?.present ? t("Detected") : t("Not detected")}</Badge>
            {ups?.model ? <span className="chip">{ups.model}</span> : null}
          </>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
        <Card className="card-pad">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink"><BatteryCharging className="size-4 text-[var(--success)]" /> {t("Battery")}</h2>
              <p className="mt-1 text-[10.5px] text-subtle">{ups?.status || t("No status available")}</p>
            </div>
            <Badge tone={ups?.onBattery ? "warning" : "success"} dot>{ups?.onBattery ? t("On battery") : t("Online")}</Badge>
          </div>
          {ups?.present ? (
            <>
              <div className="mt-5 flex items-end gap-6">
                <div>
                  <p className="num text-[34px] font-semibold leading-none text-ink">{Math.round(ups.battery)}%</p>
                  <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">{t("charge")}</p>
                </div>
                <div>
                  <p className="num text-[22px] font-semibold leading-none text-ink">{runtimeLabel(ups.runtime)}</p>
                  <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">{t("runtime left")}</p>
                </div>
                <div>
                  <p className="num text-[22px] font-semibold leading-none text-ink">{ups.load}%</p>
                  <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">{t("load")}</p>
                </div>
              </div>
              <Progress value={ups.battery} height={7} className="mt-4" />
              <div className="mt-4 grid grid-cols-2 gap-3 text-[10.5px]">
                <span className="text-muted">{t("Input voltage")} <b className="num text-soft">{ups.inputVoltage.toFixed(0)} V</b></span>
                <span className="text-muted">{t("Output voltage")} <b className="num text-soft">{ups.outputVoltage.toFixed(0)} V</b></span>
                <span className="text-muted">{t("Nominal power")} <b className="num text-soft">{ups.nominalPower ? `${ups.nominalPower.toFixed(0)} W` : "—"}</b></span>
                <span className="text-muted">{t("Source")} <b className="text-soft">apcupsd / NUT</b></span>
              </div>
            </>
          ) : (
            <div className="mt-5 flex items-start gap-2 rounded-xl border border-[var(--warning-soft)] bg-[var(--warning-soft)] p-3 text-[10.5px] text-[var(--warning)]">
              <ShieldAlert className="mt-0.5 size-3.5 shrink-0" /> {t("No UPS is detected. Check apcupsd or the NUT plugin on the host.")}
            </div>
          )}
        </Card>

        <Card className="card-pad">
          <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink"><Zap className="size-4 text-[var(--accent)]" /> {t("Shutdown policy")}</h2>
          <p className="mt-1 text-[10px] leading-relaxed text-subtle">{t("These values are written to the Unraid apcupsd plugin and the service is restarted. NUT plugin settings remain in the NUT interface.")}</p>
          {settings.isLoading ? <p className="mt-4 text-[10px] text-muted">{t("Loading UPS settings…")}</p> : settings.isError ? <p className="mt-4 text-[10px] text-[var(--danger)]">{(settings.error as Error).message}</p> : draft ? (
            <div className="mt-4 space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className={label}>{t("Service")}</span>
                  <select value={draft.service} onChange={(event) => setDraft({ ...draft, service: event.target.value })} className={field}>
                    <option value="enable">{t("Enabled")}</option>
                    <option value="disable">{t("Disabled")}</option>
                  </select>
                </label>
                <label className="block">
                  <span className={label}>{t("Cable")}</span>
                  <input value={draft.cable} onChange={(event) => setDraft({ ...draft, cable: event.target.value })} className={field} />
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className={label}>{t("Type")}</span>
                  <input value={draft.type} onChange={(event) => setDraft({ ...draft, type: event.target.value })} className={field} />
                </label>
                <label className="block">
                  <span className={label}>{t("Device")}</span>
                  <input value={draft.device} onChange={(event) => setDraft({ ...draft, device: event.target.value })} className={field} />
                </label>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <label className="block">
                  <span className={label}>{t("Battery level (%)")}</span>
                  <input type="number" min={0} max={100} value={draft.batteryLevel} onChange={(event) => setDraft({ ...draft, batteryLevel: Number(event.target.value) })} className={field} />
                </label>
                <label className="block">
                  <span className={label}>{t("Runtime (min)")}</span>
                  <input type="number" min={0} max={999} value={draft.minutes} onChange={(event) => setDraft({ ...draft, minutes: Number(event.target.value) })} className={field} />
                </label>
                <label className="block">
                  <span className={label}>{t("Timeout (s)")}</span>
                  <input type="number" min={0} max={9999} value={draft.timeout} onChange={(event) => setDraft({ ...draft, timeout: Number(event.target.value) })} className={field} />
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className={label}>{t("Nominal power override")}</span>
                  <input type="number" min={0} value={draft.overrideCapacity} onChange={(event) => setDraft({ ...draft, overrideCapacity: Number(event.target.value) })} className={field} />
                </label>
                <label className="block">
                  <span className={label}>{t("Turn off UPS")}</span>
                  <select value={draft.killUps} onChange={(event) => setDraft({ ...draft, killUps: event.target.value })} className={field}>
                    <option value="no">{t("No")}</option>
                    <option value="yes">{t("Yes")}</option>
                  </select>
                </label>
              </div>
              <Button variant="primary" icon={<Save className="size-3.5" />} loading={save.isPending} onClick={() => save.mutate()}>
                {t("Save settings")}
              </Button>
            </div>
          ) : null}
        </Card>
      </div>
    </>
  );
}
