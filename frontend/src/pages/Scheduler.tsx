import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Clock3, HardDrive, RefreshCw, RotateCcw, Save, ShieldCheck, Sparkles, Timer } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Modal } from "../components/ui/overlays";
import { Badge, Button, Card, Switch, cn } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { useToast } from "../app/ToastProvider";
import type { SchedulerJob, SchedulerPreview, SchedulerSettings, SystemDisplaySettings } from "../lib/types";

type Translate = (key: string, params?: Record<string, string | number>) => string;

const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const MONTHS = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
const DISPLAY_DATES = [
  ["%c", "System setting"],
  ["%A, %Y %B %e", "Day, YYYY Month D"],
  ["%A, %e %B %Y", "Day, D Month YYYY"],
  ["%A, %B %e, %Y", "Day, Month D, YYYY"],
  ["%A, %m/%d/%Y", "Day, MM/DD/YYYY"],
  ["%A, %d-%m-%Y", "Day, DD-MM-YYYY"],
  ["%A, %d.%m.%Y", "Day, DD.MM.YYYY"],
  ["%A, %Y-%m-%d", "Day, YYYY-MM-DD"],
];
const DISPLAY_TIMES = [
  ["%I:%M %p", "12 hours"],
  ["%R", "24 hours"],
];

function cloneSettings(settings: SchedulerSettings): SchedulerSettings {
  return {
    parity: { ...settings.parity },
    mover: { ...settings.mover },
    trim: { ...settings.trim },
  };
}

export function SchedulerPage() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const state = useQuery({ queryKey: ["scheduler-state"], queryFn: api.schedulerState });
  const [draft, setDraft] = useState<SchedulerSettings | null>(null);
  const [display, setDisplay] = useState<SystemDisplaySettings | null>(null);
  const [preview, setPreview] = useState<SchedulerPreview | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!state.data) return;
    setDraft(cloneSettings(state.data.settings));
    setDisplay(state.data.system.display);
    setError("");
  }, [state.data]);

  const previewMutation = useMutation({
    mutationFn: (settings: SchedulerSettings) => api.previewScheduler(settings),
    onSuccess: (value) => setPreview(value),
    onError: (value: Error) => setError(value.message),
  });

  const applyMutation = useMutation({
    mutationFn: (settings: SchedulerSettings) => api.applyScheduler(settings, true),
    onSuccess: () => {
      setPreview(null);
      void queryClient.invalidateQueries({ queryKey: ["scheduler-state"] });
      toast({ kind: "success", title: t("Schedules updated") });
    },
    onError: (value: Error) => setError(value.message),
  });

  const displayMutation = useMutation({
    mutationFn: (settings: SystemDisplaySettings) => api.updateDisplaySettings(settings, true),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["scheduler-state"] });
      toast({ kind: "success", title: t("Display formats updated") });
    },
    onError: (value: Error) => setError(value.message),
  });

  const dirty = useMemo(() => {
    if (!state.data || !draft) return false;
    return JSON.stringify(state.data.settings) !== JSON.stringify(draft);
  }, [draft, state.data]);
  const displayDirty = useMemo(() => {
    if (!state.data || !display) return false;
    return JSON.stringify(state.data.system.display) !== JSON.stringify(display);
  }, [display, state.data]);

  const updateJob = <K extends "parity" | "mover" | "trim">(key: K, patch: Partial<SchedulerSettings[K]>) => {
    setDraft((current) => current ? { ...current, [key]: { ...current[key], ...patch } } : current);
    setPreview(null);
    setError("");
  };

  const reset = () => {
    if (!state.data) return;
    setDraft(cloneSettings(state.data.settings));
    setDisplay(state.data.system.display);
    setPreview(null);
    setError("");
  };

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Scheduler")}
        description={t("Control parity checks, the mover, SSD TRIM and the display formats used by Unraid.")}
        meta={
          <>
            <Badge tone="accent" dot>{t("Safe preview")}</Badge>
            {state.data?.system.timeZone ? <span className="chip">{state.data.system.timeZone}</span> : null}
            {state.data?.system.currentTime ? <span className="chip">{new Date(state.data.system.currentTime).toLocaleString()}</span> : null}
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RotateCcw className="size-3.5" />} onClick={reset} disabled={!dirty && !displayDirty}>{t("Reset")}</Button>
            <Button
              variant="primary"
              icon={preview ? <Check className="size-3.5" /> : <RefreshCw className="size-3.5" />}
              disabled={!draft || !dirty || previewMutation.isPending}
              loading={previewMutation.isPending}
              onClick={() => draft && previewMutation.mutate(draft)}
            >
              {t("Preview changes")}
            </Button>
          </>
        }
      />

      {error ? <div className="mb-4 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-[10.5px] text-[var(--danger)]">{error}</div> : null}

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card className="card-pad">
          <CardHeading icon={<ShieldCheck className="size-4" />} title={t("Parity check")} description={t("Schedule mdcmd check runs and whether errors are corrected.")} />
          {draft ? (
            <div className="mt-4 space-y-3">
              <ModeRow label={t("Schedule")} value={draft.parity.mode} onChange={(mode) => updateJob("parity", { mode })} options={["disabled", "daily", "weekly", "monthly", "yearly"]} t={t} />
              {draft.parity.mode !== "disabled" ? (
                <>
                  <TimeRow job={draft.parity} onChange={(patch) => updateJob("parity", patch)} t={t} />
                  {draft.parity.mode === "weekly" ? <DayRow value={draft.parity.day} onChange={(day) => updateJob("parity", { day })} t={t} /> : null}
                  {draft.parity.mode === "monthly" || draft.parity.mode === "yearly" ? <MonthDayRow value={draft.parity.dayOfMonth} onChange={(dayOfMonth) => updateJob("parity", { dayOfMonth })} t={t} /> : null}
                  {draft.parity.mode === "yearly" ? <MonthRow value={draft.parity.month} onChange={(month) => updateJob("parity", { month })} t={t} /> : null}
                  <ToggleRow label={t("Write corrections to parity")} description={t("Correct parity errors when the check finds them.")} checked={draft.parity.writeCorrections} onChange={(writeCorrections) => updateJob("parity", { writeCorrections })} />
                </>
              ) : null}
            </div>
          ) : <Loading t={t} />}
        </Card>

        <Card className="card-pad">
          <CardHeading icon={<HardDrive className="size-4" />} title={t("Mover")} description={t("Schedule the official mover and tune its logging and progress output.")} />
          {draft ? (
            <div className="mt-4 space-y-3">
              <ModeRow label={t("Schedule")} value={draft.mover.mode} onChange={(mode) => updateJob("mover", { mode })} options={["disabled", "hourly", "daily", "weekly", "monthly"]} t={t} />
              {draft.mover.mode !== "disabled" ? (
                <>
                  {draft.mover.mode === "hourly" ? <IntervalRow job={draft.mover} onChange={(patch) => updateJob("mover", patch)} t={t} /> : <TimeRow job={draft.mover} onChange={(patch) => updateJob("mover", patch)} t={t} />}
                  {draft.mover.mode === "weekly" ? <DayRow value={draft.mover.day} onChange={(day) => updateJob("mover", { day })} t={t} /> : null}
                  {draft.mover.mode === "monthly" ? <MonthDayRow value={draft.mover.dayOfMonth} onChange={(dayOfMonth) => updateJob("mover", { dayOfMonth })} t={t} /> : null}
                  <ToggleRow label={t("Mover logging")} description={t("Write mover activity to the system log.")} checked={draft.mover.logging} onChange={(logging) => updateJob("mover", { logging })} />
                  <ToggleRow label={t("Mover progress")} description={t("Expose byte progress while the mover runs.")} checked={draft.mover.progress} onChange={(progress) => updateJob("mover", { progress })} />
                </>
              ) : null}
            </div>
          ) : <Loading t={t} />}
        </Card>

        <Card className="card-pad">
          <CardHeading icon={<Timer className="size-4" />} title={t("SSD TRIM")} description={t("Schedule the official SSD TRIM maintenance script.")} />
          {draft ? (
            <div className="mt-4 space-y-3">
              <ModeRow label={t("Schedule")} value={draft.trim.mode} onChange={(mode) => updateJob("trim", { mode })} options={["disabled", "hourly", "daily", "weekly", "monthly"]} t={t} />
              {draft.trim.mode !== "disabled" ? (
                <>
                  {draft.trim.mode === "hourly" ? <IntervalRow job={draft.trim} onChange={(patch) => updateJob("trim", patch)} t={t} /> : <TimeRow job={draft.trim} onChange={(patch) => updateJob("trim", patch)} t={t} />}
                  {draft.trim.mode === "weekly" ? <DayRow value={draft.trim.day} onChange={(day) => updateJob("trim", { day })} t={t} /> : null}
                  {draft.trim.mode === "monthly" ? <MonthDayRow value={draft.trim.dayOfMonth} onChange={(dayOfMonth) => updateJob("trim", { dayOfMonth })} t={t} /> : null}
                </>
              ) : null}
              <p className="rounded-xl border border-line bg-[var(--surface-2)]/40 p-3 text-[9.5px] leading-relaxed text-subtle">
                {t("TRIM runs outside parity checks and never touches array data. The upstream SSD TRIM script decides which devices are eligible.")}
              </p>
            </div>
          ) : <Loading t={t} />}
        </Card>

        <Card className="card-pad">
          <CardHeading icon={<Clock3 className="size-4" />} title={t("Date and time display")} description={t("These formats are shared with the official webGUI.")} />
          <div className="mt-4 space-y-3">
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectRow
                label={t("Date format")}
                value={display?.dateFormat ?? "%c"}
                onChange={(dateFormat) => setDisplay((current) => current ? { ...current, dateFormat } : current)}
                options={DISPLAY_DATES}
                t={t}
              />
              <SelectRow
                label={t("Time format")}
                value={display?.timeFormat ?? "%R"}
                onChange={(timeFormat) => setDisplay((current) => current ? { ...current, timeFormat } : current)}
                options={DISPLAY_TIMES}
                t={t}
              />
            </div>
            <div className="rounded-lg border border-line bg-[var(--surface-2)]/45 px-3 py-2 text-[9.5px] text-subtle">
              <p className="font-medium text-soft">{t("Time zone and NTP")}</p>
              <p className="mt-1">{state.data?.system.timeZone || "—"} · {state.data?.system.ntpEnabled ? t("NTP enabled") : t("Manual time")} · {state.data?.system.ntpServers.join(", ") || t("No NTP servers")}</p>
              <p className="mt-1">{t("Time zone and NTP servers are read-only here; change them in the official webGUI to avoid a clock lockout.")}</p>
            </div>
            <Button variant="secondary" icon={<Save className="size-3.5" />} loading={displayMutation.isPending} disabled={!display || !displayDirty} onClick={() => display && displayMutation.mutate(display)}>
              {t("Save display formats")}
            </Button>
          </div>
        </Card>
      </div>

      <Modal
        open={Boolean(preview)}
        onClose={() => setPreview(null)}
        title={t("Review schedule changes")}
        description={t("Aurora writes only the files below. Every write is backed up in History.")}
        size="xl"
        footer={
          <>
            <Button variant="secondary" onClick={() => setPreview(null)} disabled={applyMutation.isPending}>{t("Cancel")}</Button>
            <Button variant="primary" icon={<ShieldCheck className="size-3.5" />} loading={applyMutation.isPending} onClick={() => draft && applyMutation.mutate(draft)}>
              {t("Apply schedules")}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          {preview?.warnings.map((warning) => (
            <p key={warning} className="rounded-lg border border-[var(--warning)]/30 bg-[var(--warning-soft)] p-3 text-[10px] text-[var(--warning)]">{warning}</p>
          ))}
          {preview?.files.map((file) => (
            <div key={file.path} className="overflow-hidden rounded-xl border border-line">
              <div className="flex items-center justify-between gap-2 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2">
                <span className="truncate font-mono text-[9.5px] text-soft">{file.path}</span>
                <Badge tone={file.delete ? "warning" : file.diff?.trim() ? "neutral" : "success"}>{file.delete ? t("Delete") : file.diff?.trim() ? t("Write") : t("Unchanged")}</Badge>
              </div>
              <DiffView diff={file.diff} t={t} />
            </div>
          ))}
          <p className="flex items-start gap-2 rounded-lg border border-line bg-[var(--surface-2)]/45 p-3 text-[9.5px] leading-relaxed text-subtle">
            <Sparkles className="mt-0.5 size-3.5 shrink-0 text-[var(--accent)]" />
            {t("The preview is generated from the values above. Applying also runs update_cron so the new schedule is active immediately.")}
          </p>
        </div>
      </Modal>
    </>
  );
}

function DiffView({ diff, t }: { diff?: string; t: Translate }) {
  if (!diff || !diff.trim()) {
    return <p className="px-3 py-3 text-[10px] text-subtle">{t("No changes.")}</p>;
  }
  const lines = diff.replace(/\n$/, "").split("\n");
  return (
    <div className="max-h-64 overflow-auto bg-[#08090d] py-1 font-mono text-[9px] leading-relaxed">
      {lines.map((line, index) => {
        const kind = line.startsWith("+++") || line.startsWith("---")
          ? "header"
          : line.startsWith("@@")
            ? "hunk"
            : line.startsWith("+")
              ? "add"
              : line.startsWith("-")
                ? "remove"
                : "context";
        return (
          <div
            key={index}
            className={cn(
              "whitespace-pre px-3",
              kind === "header" && "text-[#7d8597]",
              kind === "hunk" && "bg-[var(--info-soft)] text-[var(--info)]",
              kind === "add" && "bg-[color-mix(in_srgb,var(--success)_13%,transparent)] text-[var(--success)]",
              kind === "remove" && "bg-[color-mix(in_srgb,var(--danger)_13%,transparent)] text-[var(--danger)]",
              kind === "context" && "text-[#c7cbd6]",
            )}
          >
            {line || " "}
          </div>
        );
      })}
    </div>
  );
}

function CardHeading({ icon, title, description }: { icon: ReactNode; title: string; description: string }) {
  return (
    <div className="flex items-start gap-2.5">
      <span className="grid size-8 shrink-0 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
      <div>
        <h2 className="text-[12.5px] font-semibold text-ink">{title}</h2>
        <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">{description}</p>
      </div>
    </div>
  );
}

function ModeRow({ label, value, onChange, options, t }: { label: string; value: string; onChange: (value: string) => void; options: string[]; t: Translate }) {
  return (
    <label className="flex flex-wrap items-center justify-between gap-2">
      <span className="text-[10.5px] text-soft">{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
        {options.map((option) => <option key={option} value={option}>{t(option.charAt(0).toUpperCase() + option.slice(1))}</option>)}
      </select>
    </label>
  );
}

function TimeRow({ job, onChange, t }: { job: SchedulerJob; onChange: (patch: Partial<SchedulerJob>) => void; t: Translate }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <span className="text-[10.5px] text-soft">{t("Time")}</span>
      <div className="flex items-center gap-1.5">
        <select value={job.hour} onChange={(event) => onChange({ hour: Number(event.target.value) })} aria-label={t("Hour")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2 font-mono text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
          {Array.from({ length: 24 }, (_, hour) => <option key={hour} value={hour}>{String(hour).padStart(2, "0")}</option>)}
        </select>
        <span className="text-subtle">:</span>
        <select value={job.minute} onChange={(event) => onChange({ minute: Number(event.target.value) })} aria-label={t("Minute")} className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2 font-mono text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
          {Array.from({ length: 12 }, (_, index) => index * 5).map((minute) => <option key={minute} value={minute}>{String(minute).padStart(2, "0")}</option>)}
        </select>
      </div>
    </div>
  );
}

function IntervalRow({ job, onChange, t }: { job: SchedulerJob; onChange: (patch: Partial<SchedulerJob>) => void; t: Translate }) {
  return (
    <label className="flex flex-wrap items-center justify-between gap-2">
      <span className="text-[10.5px] text-soft">{t("Interval")}</span>
      <select value={job.intervalHours} onChange={(event) => onChange({ intervalHours: Number(event.target.value) })} className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
        {[1, 2, 3, 4, 6, 8, 12].map((hours) => <option key={hours} value={hours}>{t("Every {count} hours", { count: hours })}</option>)}
      </select>
    </label>
  );
}

function DayRow({ value, onChange, t }: { value: number; onChange: (value: number) => void; t: Translate }) {
  return (
    <label className="flex flex-wrap items-center justify-between gap-2">
      <span className="text-[10.5px] text-soft">{t("Day of the week")}</span>
      <select value={value} onChange={(event) => onChange(Number(event.target.value))} className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
        {DAYS.map((day, index) => <option key={day} value={index}>{t(day)}</option>)}
      </select>
    </label>
  );
}

function MonthDayRow({ value, onChange, t }: { value: number; onChange: (value: number) => void; t: Translate }) {
  return (
    <label className="flex flex-wrap items-center justify-between gap-2">
      <span className="text-[10.5px] text-soft">{t("Day of the month")}</span>
      <select value={value} onChange={(event) => onChange(Number(event.target.value))} className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
        {Array.from({ length: 31 }, (_, index) => index + 1).map((day) => <option key={day} value={day}>{String(day).padStart(2, "0")}</option>)}
      </select>
    </label>
  );
}

function MonthRow({ value, onChange, t }: { value: number; onChange: (value: number) => void; t: Translate }) {
  return (
    <label className="flex flex-wrap items-center justify-between gap-2">
      <span className="text-[10.5px] text-soft">{t("Month")}</span>
      <select value={value} onChange={(event) => onChange(Number(event.target.value))} className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
        {MONTHS.map((month, index) => <option key={month} value={index + 1}>{t(month)}</option>)}
      </select>
    </label>
  );
}

function ToggleRow({ label, description, checked, onChange }: { label: string; description: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5">
      <span>
        <span className="block text-[10.5px] font-medium text-soft">{label}</span>
        <span className="block text-[9.5px] text-subtle">{description}</span>
      </span>
      <Switch checked={checked} onChange={onChange} label={label} />
    </label>
  );
}

function SelectRow({ label, value, onChange, options, t }: { label: string; value: string; onChange: (value: string) => void; options: string[][]; t: Translate }) {
  return (
    <label className="space-y-1">
      <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]">
        {options.map(([optionValue, optionLabel]) => <option key={optionValue} value={optionValue}>{t(optionLabel)}</option>)}
      </select>
    </label>
  );
}

function Loading({ t }: { t: Translate }) {
  return <p className="mt-4 text-[10.5px] text-muted">{t("Reading scheduler settings…")}</p>;
}
