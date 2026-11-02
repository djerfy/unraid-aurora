import { useEffect, useMemo, useState, type ChangeEvent, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import {
  Activity,
  ArrowDownToLine,
  ArrowUpFromLine,
  CircleAlert,
  ArrowRightLeft,
  ChevronDown,
  Database,
  FolderTree,
  Gauge,
  HardDrive,
  History,
  Info,
  LockKeyhole,
  Pause,
  Play,
  Plus,
  RefreshCw,
  Square,
  Search,
  ShieldCheck,
  Thermometer,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { StorageLocation } from "../components/storage/StorageLocation";
import { AreaChart, BarChart, Ring, SegmentBar } from "../components/ui/charts";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  InputBase,
  Progress,
  Switch,
  Tabs,
  cn,
} from "../components/ui/primitives";
import { Drawer, Menu, Modal } from "../components/ui/overlays";
import { useApp } from "../app/AppDataProvider";
import { useAction } from "../lib/useAction";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import {
  formatBytes,
  formatDateTime,
  formatDuration,
  formatPercent,
  formatShortDate,
  formatSpeed,
  relativeTime,
} from "../lib/format";
import type { Disk, MoverRun, Share, ShareConfig } from "../lib/types";

const TABS = [
  { value: "array", label: "Array" },
  { value: "location", label: "Location" },
  { value: "shares", label: "Shares" },
  { value: "pools", label: "Pools" },
  { value: "parity", label: "Parity" },
];

const MOVER_PHASES = [
  { key: "scanning", label: "Scanning" },
  { key: "calculating", label: "Calculating" },
  { key: "moving", label: "Moving" },
  { key: "finishing", label: "Finishing" },
];

const CACHE_MODES: Record<string, string> = {
  no: "No",
  yes: "Yes",
  prefer: "Prefer",
  only: "Only",
};

function cacheModeLabel(t: (key: string) => string, mode: string) {
  return CACHE_MODES[mode] ? t(CACHE_MODES[mode]) : mode;
}

export function StoragePage() {
  const { snapshot, metrics, history, refresh } = useApp();
  const action = useAction();
  const { toast } = useToast();
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") ?? "array";
  const [selectedDisk, setSelectedDisk] = useState<Disk | null>(null);
  const [selectedShare, setSelectedShare] = useState<Share | null>(null);
  const [showIO, setShowIO] = useState(true);
  const [query, setQuery] = useState("");
  const [arrayModal, setArrayModal] = useState<"start" | "stop" | null>(null);
  const [shareEditor, setShareEditor] = useState<{ share: Share | null } | null>(null);

  if (!snapshot) return null;
  const { array, disks, shares, pools } = snapshot;
  const parity = array.parity;

  const dataDisks = disks.filter((disk) => disk.type === "Data");
  const parityDisks = disks.filter((disk) => disk.type === "Parity");
  const cacheDisks = disks.filter((disk) => disk.type === "Cache");
  const flash = disks.find((disk) => disk.type === "Flash");

  const filteredShares = useMemo(
    () =>
      shares.filter(
        (share) =>
          share.name.toLowerCase().includes(query.toLowerCase()) ||
          share.comment.toLowerCase().includes(query.toLowerCase()),
      ),
    [shares, query],
  );

  const eta = parity.running && parity.speed > 0
    ? ((100 - parity.progress) / 100) * (parity.lastCheck?.size || array.total) / parity.speed
    : 0;

  const tabAction = (value: string) => {
    const next = new URLSearchParams(params);
    next.set("tab", value);
    setParams(next, { replace: true });
  };

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("Storage")}
        description={t(
          "Your array, shares and cache pools — with live health, throughput and parity telemetry.",
        )}
        meta={
          <>
            <Badge tone={array.state === "STARTED" ? "success" : "neutral"} dot>
              {array.state === "STARTED" ? t("Array started") : t("Array stopped")}
            </Badge>
            <Badge tone={array.status === "protected" ? "violet" : "warning"}>
              {array.status === "protected" ? t("Parity protected") : t("Unprotected")}
            </Badge>
            <span className="chip">{t("{count} devices", { count: disks.length })}</span>
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
              {t("Refresh")}
            </Button>
            {array.state === "STARTED" ? (
              <Button
                variant="secondary"
                icon={<Square className="size-3.5" />}
                onClick={() => setArrayModal("stop")}
              >
                {t("Stop array")}
              </Button>
            ) : (
              <Button
                variant="primary"
                icon={<Play className="size-3.5" />}
                onClick={() => setArrayModal("start")}
              >
                {t("Start array")}
              </Button>
            )}
            {array.state === "STARTED" && parity.running ? (
              <Button
                variant="primary"
                icon={<Pause className="size-3.5" />}
                loading={action.isPending}
                onClick={() =>
                  action.mutate({
                    kind: parity.paused ? "parity.resume" : "parity.pause",
                    success: parity.paused ? t("Parity check resumed") : t("Parity check paused"),
                  })
                }
              >
                {parity.paused ? t("Resume check") : t("Pause check")}
              </Button>
            ) : array.state === "STARTED" ? (
              <Button
                variant="primary"
                icon={<Play className="size-3.5" />}
                loading={action.isPending}
                onClick={() =>
                  action.mutate({ kind: "parity.start", success: t("Parity check started") })
                }
              >
                {t("Start parity check")}
              </Button>
            ) : null}
          </>
        }
      />

      <div className="mb-5 grid grid-cols-1 gap-3.5 sm:grid-cols-2 xl:grid-cols-4">
        <SummaryCard
          icon={<HardDrive className="size-4" />}
          label={t("Array capacity")}
          value={formatBytes(array.used)}
          detail={t("of {total} · {free} free", {
            total: formatBytes(array.total),
            free: formatBytes(array.free),
          })}
        >
          <SegmentBar
            height={5}
            segments={[
              ...dataDisks.map((disk, index) => ({
                value: Math.max(1, disk.used),
                color: disk.status === "standby"
                  ? "var(--surface-3)"
                  : `color-mix(in srgb, var(--accent) ${86 - index * 7}%, var(--surface-3))`,
                label: disk.name,
              })),
              { value: Math.max(1, array.free), color: "var(--surface-3)", label: t("free") },
            ]}
            className="mt-3"
          />
        </SummaryCard>

        <SummaryCard
          icon={<ShieldCheck className="size-4" />}
          label={t("Protection")}
          value={parity.configured ? t("{count}× parity", { count: parity.disks.length }) : t("None")}
          detail={
            parity.lastCheck
              ? t(
                  parity.lastCheck.errors === 1
                    ? "Last check {time} · {count} error"
                    : "Last check {time} · {count} errors",
                  {
                    time: relativeTime(parity.lastCheck.date),
                    count: parity.lastCheck.errors,
                  },
                )
              : t("No parity check recorded")
          }
        >
          <div className="mt-3 flex items-center gap-1.5">
            <span className={cn("status-dot", parity.errors > 0 ? "danger" : "live")} />
            <span className="text-[10.5px] text-muted">
              {parity.errors > 0
                ? t(
                    parity.errors === 1 ? "{count} sync error" : "{count} sync errors",
                    { count: parity.errors },
                  )
                : t("No sync errors detected")}
            </span>
          </div>
        </SummaryCard>

        <SummaryCard
          icon={<Database className="size-4" />}
          label={t("Cache pool")}
          value={pools[0] ? formatBytes(pools[0].used) : "—"}
          detail={
            pools[0]
              ? t("{count} devices · {fs} · {free} free", {
                  count: pools[0].devices.length,
                  fs: pools[0].fsType,
                  free: formatBytes(pools[0].free),
                })
              : t("No cache pool")
          }
        >
          <Progress value={pools[0]?.usage ?? 0} height={5} className="mt-3" />
        </SummaryCard>

        <SummaryCard
          icon={<Activity className="size-4" />}
          label={t("Live throughput")}
          value={formatSpeed((metrics?.diskRead ?? 0) + (metrics?.diskWrite ?? 0))}
          detail={t("{read} read · {write} write", {
            read: formatSpeed(metrics?.diskRead ?? 0),
            write: formatSpeed(metrics?.diskWrite ?? 0),
          })}
        >
          <AreaChart
            series={[
              { values: history.slice(-80).map((p) => p.diskRead), color: "var(--info)", label: t("Read") },
              { values: history.slice(-80).map((p) => p.diskWrite), color: "var(--violet)", label: t("Write") },
            ]}
            height={30}
            grid={false}
            formatValue={(v) => formatSpeed(v)}
            className="mt-1"
          />
        </SummaryCard>
      </div>

      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <Tabs
          items={TABS.map((item) => ({
            ...item,
            label: t(item.label),
            count:
              item.value === "array"
                ? disks.length
                : item.value === "shares"
                  ? shares.length
                  : item.value === "pools"
                    ? pools.length
                    : undefined,
          }))}
          value={tab}
          onChange={tabAction}
        />
        {tab === "shares" ? (
          <div className="flex items-center gap-2">
            <InputBase
              value={query}
              onChange={setQuery}
              placeholder={t("Filter shares…")}
              icon={<Search className="size-3.5" />}
              className="w-56"
            />
            <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setShareEditor({ share: null })}>
              {t("New share")}
            </Button>
          </div>
        ) : tab === "array" ? (
          <label className="flex items-center gap-2 text-[11px] text-muted">
            <Switch checked={showIO} onChange={setShowIO} label={t("Show live I/O")} />
            {t("Show live I/O")}
          </label>
        ) : null}
      </div>

      {tab === "location" ? (
        <StorageLocation disks={disks} onOpenDisk={setSelectedDisk} />
      ) : tab === "array" ? (
        <div className="space-y-4">
          <MoverCard arrayRunning={array.state === "STARTED"} />
          <div className="flex flex-wrap items-center gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[10.5px] text-subtle">
            <Info className="size-3.5" />
            <span>
              {t("Spin disks down to save energy. A spun-down disk wakes automatically on first access.")}
            </span>
            <span className="ml-auto flex items-center gap-1.5">
              <span className="status-dot" />{" "}
              {t("{count} active", { count: dataDisks.filter((d) => d.status === "active").length })}
              <span className="status-dot muted" />{" "}
              {t("{count} standby", { count: dataDisks.filter((d) => d.status === "standby").length })}
            </span>
          </div>

          <Card>
            <div className="flex items-center justify-between border-b border-line px-5 py-3.5">
              <div>
                <h2 className="text-[13px] font-semibold text-ink">{t("Array devices")}</h2>
                <p className="mt-0.5 text-[10.5px] text-subtle">
                  {t("Parity, data disks and pool members")}
                </p>
              </div>
              <Badge tone="neutral">{t("{size} usable", { size: formatBytes(array.total) })}</Badge>
            </div>

            <DiskGroup label={t("Parity")} disks={parityDisks} showIO={showIO} onSelect={setSelectedDisk} />
            <DiskGroup label={t("Data")} disks={dataDisks} showIO={showIO} onSelect={setSelectedDisk} />
            {cacheDisks.length ? (
              <DiskGroup label={t("Cache pool")} disks={cacheDisks} showIO={showIO} onSelect={setSelectedDisk} />
            ) : null}
            {flash ? <DiskGroup label={t("Boot")} disks={[flash]} showIO={showIO} onSelect={setSelectedDisk} /> : null}
          </Card>
        </div>
      ) : null}

      {tab === "shares" ? (
        filteredShares.length === 0 ? (
          <Card>
            <EmptyState
              icon={<FolderTree className="size-5" />}
              title={t("No shares match your filter")}
              description={t("Try a different name or clear the search field.")}
            />
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            {filteredShares.map((share) => (
              <ShareCard key={share.name} share={share} onOpen={() => setSelectedShare(share)} />
            ))}
          </div>
        )
      ) : null}

      {tab === "pools" ? (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
          {pools.map((pool) => {
            const members = disks.filter((disk) => pool.devices.includes(disk.name));
            return (
              <Card key={pool.name} className="card-pad xl:col-span-2">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="grid size-7 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-[var(--info)]">
                        <Database className="size-3.5" />
                      </span>
                      <div>
                        <h2 className="text-[13px] font-semibold text-ink">
                          {t("{name} pool", { name: pool.name })}
                        </h2>
                        <p className="text-[10.5px] text-subtle">{t("{fs} · cache pool", { fs: pool.fsType })}</p>
                      </div>
                    </div>
                  </div>
                  <Badge tone={pool.status === "healthy" ? "success" : "warning"} dot>
                    {pool.status === "healthy" ? t("Healthy") : pool.status}
                  </Badge>
                </div>

                <div className="mt-5 grid gap-5 sm:grid-cols-[1fr_auto]">
                  <div>
                    <div className="flex items-end justify-between">
                      <div>
                        <p className="num text-[24px] font-semibold leading-none text-ink">{formatBytes(pool.used)}</p>
                        <p className="mt-1 text-[10.5px] text-subtle">
                          {t("of {size} · {free} free", {
                            size: formatBytes(pool.size),
                            free: formatBytes(pool.free),
                          })}
                        </p>
                      </div>
                      <span className="num text-xs text-muted">{formatPercent(pool.usage, 1)}</span>
                    </div>
                    <Progress value={pool.usage} height={7} className="mt-3" />
                    <div className="mt-4 space-y-2">
                      {members.map((disk) => (
                        <div key={disk.id} className="flex items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/60 px-3 py-2">
                          <HardDrive className="size-4 text-subtle" />
                          <div className="min-w-0 flex-1">
                            <p className="text-[11.5px] font-medium text-soft">{disk.name}</p>
                            <p className="truncate text-[9.5px] text-subtle">{disk.model}</p>
                          </div>
                          <span className="num text-[10px] text-muted">{disk.temp}°C</span>
                          <Badge tone={disk.smart === "passed" ? "success" : "warning"}>
                            {disk.smart === "passed" ? t("Healthy") : t("Check SMART")}
                          </Badge>
                        </div>
                      ))}
                    </div>
                  </div>
                  <Ring value={pool.usage} size={132} stroke={8}>
                    <div className="text-center">
                      <p className="num text-[24px] font-semibold leading-none text-ink">{Math.round(pool.usage)}%</p>
                      <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">{t("used")}</p>
                    </div>
                  </Ring>
                </div>
              </Card>
            );
          })}

          <Card className="card-pad">
            <div className="flex items-center gap-2">
              <span className="grid size-7 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-[var(--violet)]">
                <Gauge className="size-3.5" />
              </span>
              <h2 className="text-[13px] font-semibold text-ink">{t("Cache strategy")}</h2>
            </div>
            <p className="mt-3 text-[11.5px] leading-relaxed text-muted">
              {t("Shares configured with")}{" "}
              <span className="font-medium text-soft">{t("Only")}</span>{" "}
              {t("keep data on the cache pool permanently.")}{" "}
              <span className="font-medium text-soft">{t("Prefer")}</span>{" "}
              {t("uses the pool as a fast landing zone before the mover runs.")}
            </p>
            <div className="mt-4 space-y-2.5">
              {shares
                .filter((share) => share.cacheMode && share.cacheMode !== "no")
                .map((share) => (
                  <div key={share.name} className="flex items-center justify-between gap-2 text-[11px]">
                    <span className="text-soft">{share.name}</span>
                    <span className="chip">{cacheModeLabel(t, share.cacheMode)}</span>
                  </div>
                ))}
            </div>
          </Card>
        </div>
      ) : null}

      {tab === "parity" ? (
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
          <Card className="card-pad xl:col-span-7">
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-[13px] font-semibold text-ink">{t("Parity check")}</h2>
                <p className="mt-0.5 text-[10.5px] text-subtle">
                  {parity.running
                    ? t("Checking all data disks against parity")
                    : t("No check running")}
                </p>
              </div>
              <Badge tone={parity.running ? (parity.paused ? "warning" : "accent") : "neutral"} dot pulse={parity.running && !parity.paused}>
                {parity.paused ? t("Paused") : parity.running ? t("Running") : t("Idle")}
              </Badge>
            </div>
            <div className="mt-5 flex flex-col items-center gap-6 sm:flex-row sm:items-center">
              <Ring value={parity.progress} size={150} stroke={10}>
                <div className="text-center">
                  <p className="num text-[30px] font-semibold leading-none text-ink">{parity.progress.toFixed(1)}%</p>
                  <p className="mt-1 text-[9px] uppercase tracking-[0.12em] text-subtle">{t("complete")}</p>
                </div>
              </Ring>
              <div className="flex-1 space-y-3">
                <DataRow label={t("Speed")} value={formatSpeed(parity.speed)} />
                <DataRow label={t("Position")} value={formatBytes(parity.position)} />
                <DataRow label={t("Elapsed")} value={formatDuration(parity.elapsed)} />
                <DataRow label={t("Remaining")} value={eta > 0 ? formatDuration(eta) : "—"} />
                <DataRow label={t("Sync errors")} value={String(parity.errors)} tone={parity.errors > 0 ? "warning" : "success"} />
              </div>
            </div>
            <div className="mt-5 flex flex-wrap gap-2 border-t border-line pt-4">
              {parity.running ? (
                <>
                  <Button
                    variant="secondary"
                    icon={parity.paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
                    loading={action.isPending}
                    onClick={() =>
                      action.mutate({
                        kind: parity.paused ? "parity.resume" : "parity.pause",
                        success: parity.paused ? t("Parity check resumed") : t("Parity check paused"),
                      })
                    }
                  >
                    {parity.paused ? t("Resume") : t("Pause")}
                  </Button>
                  <Button
                    variant="danger"
                    loading={action.isPending}
                    onClick={() => action.mutate({ kind: "parity.stop", success: t("Parity check stopped") })}
                  >
                    {t("Stop check")}
                  </Button>
                </>
              ) : (
                <Button
                  variant="primary"
                  icon={<Play className="size-3.5" />}
                  loading={action.isPending}
                  onClick={() => action.mutate({ kind: "parity.start", success: t("Parity check started") })}
                >
                  {t("Start check")}
                </Button>
              )}
              <Button variant="ghost" onClick={() => toast({ kind: "info", title: t("Parity scheduler"), description: t("Configure schedules from Settings → Scheduler.") })}>
                {t("Schedule")}
              </Button>
            </div>
          </Card>

          <Card className="card-pad xl:col-span-5">
            <h2 className="text-[13px] font-semibold text-ink">{t("Protection overview")}</h2>
            <div className="mt-4 space-y-3">
              <DataRow label={t("Parity devices")} value={parity.disks.join(", ") || t("None")} />
              <DataRow label={t("Data disks")} value={String(dataDisks.length)} />
              <DataRow label={t("Last check")} value={parity.lastCheck ? formatDateTime(parity.lastCheck.date) : "—"} />
              <DataRow label={t("Last duration")} value={parity.lastCheck ? formatDuration(parity.lastCheck.duration) : "—"} />
              <DataRow label={t("Last speed")} value={parity.lastCheck ? formatSpeed(parity.lastCheck.speed) : "—"} />
              <DataRow
                label={t("Result")}
                value={
                  parity.lastCheck
                    ? `${t(
                        parity.lastCheck.errors === 1 ? "{count} error" : "{count} errors",
                        { count: parity.lastCheck.errors },
                      )} · ${
                        parity.lastCheck.status === "completed"
                          ? t("Completed")
                          : parity.lastCheck.status === "failed"
                            ? t("Failed")
                            : parity.lastCheck.status
                      }`
                    : "—"
                }
                tone={parity.lastCheck?.errors ? "warning" : "success"}
              />
            </div>
            <div className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/60 p-3 text-[10.5px] leading-relaxed text-muted">
              <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
              {t("Parity protects against a single disk failure. Keep a current flash backup and verify SMART status regularly.")}
            </div>
          </Card>

          <Card className="card-pad xl:col-span-12">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-[13px] font-semibold text-ink">{t("Check history")}</h2>
                <p className="mt-0.5 text-[10.5px] text-subtle">
                  {t("Speed across the last {count} checks", { count: parity.history.length })}
                </p>
              </div>
              <Badge tone="neutral">
                {t("{size} scanned", { size: formatBytes(parity.lastCheck?.size ?? 0) })}
              </Badge>
            </div>
            <BarChart
              values={[...(parity.history ?? [])].reverse().map((check) => check.speed)}
              labels={[...(parity.history ?? [])].reverse().map((check) =>
                formatShortDate(check.date),
              )}
              height={170}
              formatValue={(value) => formatSpeed(value)}
              showValues
              className="mt-4"
            />
            <div className="mt-4 overflow-x-auto">
              <table className="w-full min-w-[620px] text-left">
                <thead>
                  <tr className="border-b border-line text-[9.5px] uppercase tracking-[0.1em] text-subtle">
                    <th className="pb-2 font-medium">{t("Date")}</th>
                    <th className="pb-2 font-medium">{t("Duration")}</th>
                    <th className="pb-2 font-medium">{t("Speed")}</th>
                    <th className="pb-2 font-medium">{t("Errors")}</th>
                    <th className="pb-2 text-right font-medium">{t("Status")}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line/60">
                  {parity.history.map((check, index) => (
                    <tr key={`${check.date}-${index}`} className="text-[11.5px]">
                      <td className="py-2.5 text-soft">{formatDateTime(check.date)}</td>
                      <td className="num py-2.5 text-muted">{formatDuration(check.duration)}</td>
                      <td className="num py-2.5 text-muted">{formatSpeed(check.speed)}</td>
                      <td className="num py-2.5 text-muted">{check.errors}</td>
                      <td className="py-2.5 text-right">
                        <Badge tone={check.status === "completed" ? "success" : "warning"}>
                          {check.status === "completed"
                            ? t("Completed")
                            : check.status === "failed"
                              ? t("Failed")
                              : check.status}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </div>
      ) : null}

      <ArrayStartModal open={arrayModal === "start"} onClose={() => setArrayModal(null)} />
      <ArrayStopModal open={arrayModal === "stop"} onClose={() => setArrayModal(null)} />

      <Drawer
        open={Boolean(selectedDisk)}
        onClose={() => setSelectedDisk(null)}
        title={selectedDisk ? `${selectedDisk.name} · ${selectedDisk.device}` : ""}
        subtitle={selectedDisk?.model}
        footer={
          selectedDisk && selectedDisk.type !== "Parity" ? (
            <Button
              variant="secondary"
              className="w-full"
              onClick={() =>
                action.mutate({
                  kind: selectedDisk.status === "active" ? "disk.spindown" : "disk.spinup",
                  target: selectedDisk.name,
                  success: selectedDisk.status === "active"
                    ? t("{name} spinning down", { name: selectedDisk.name })
                    : t("{name} spinning up", { name: selectedDisk.name }),
                })
              }
            >
              {selectedDisk.status === "active" ? t("Spin down") : t("Spin up")}
            </Button>
          ) : null
        }
      >
        {selectedDisk ? (
          <div className="space-y-5">
            <div className="flex items-center justify-between rounded-xl border border-line bg-[var(--surface-2)]/60 p-4">
              <div>
                <p className="num text-[22px] font-semibold text-ink">{formatBytes(selectedDisk.size)}</p>
                <p className="mt-1 text-[10.5px] text-subtle">
                  {selectedDisk.fsType} · {t(selectedDisk.type)}
                </p>
              </div>
              <Badge tone={selectedDisk.smart === "passed" ? "success" : "warning"} dot>
                {t("SMART {status}", { status: selectedDisk.smart })}
              </Badge>
            </div>
            <Progress value={selectedDisk.size > 0 ? (selectedDisk.used / selectedDisk.size) * 100 : 0} height={7} />
            <div className="grid grid-cols-2 gap-3">
              <Detail label={t("Used")} value={formatBytes(selectedDisk.used)} />
              <Detail label={t("Free")} value={formatBytes(selectedDisk.free)} />
              <Detail label={t("Temperature")} value={`${selectedDisk.temp}°C`} icon={<Thermometer className="size-3" />} />
              <Detail label={t("Errors")} value={String(selectedDisk.errors)} />
              <Detail label={t("Serial")} value={selectedDisk.serial} />
              <Detail label={t("Firmware")} value={selectedDisk.firmware} />
              <Detail label={t("Spinup group")} value={selectedDisk.spinupGroup || "—"} />
              <Detail label={t("Encrypted")} value={selectedDisk.luks ? t("Yes") : t("No")} />
            </div>
            <DiskSmartSection device={selectedDisk.device} />

            {(selectedDisk.warnings ?? []).length > 0 ? (
              <div className="rounded-xl border border-[color-mix(in_srgb,var(--warning)_25%,transparent)] bg-[var(--warning-soft)] p-3">
                <p className="flex items-center gap-1.5 text-[11px] font-semibold text-[var(--warning)]">
                  <CircleAlert className="size-3.5" /> {t("SMART warnings")}
                </p>
                <ul className="mt-1.5 space-y-1 text-[10.5px] text-muted">
                  {(selectedDisk.warnings ?? []).map((warning) => (
                    <li key={warning}>• {warning}</li>
                  ))}
                </ul>
              </div>
            ) : null}
            <div className="rounded-xl border border-line p-3">
              <p className="text-[10px] uppercase tracking-[0.1em] text-subtle">{t("Live I/O")}</p>
              <div className="mt-2 flex gap-6">
                <span className="num text-[11px] text-soft">
                  {t("{speed} read", { speed: formatSpeed(selectedDisk.readBps) })}
                </span>
                <span className="num text-[11px] text-soft">
                  {t("{speed} write", { speed: formatSpeed(selectedDisk.writeBps) })}
                </span>
              </div>
            </div>
          </div>
        ) : null}
      </Drawer>

      <Drawer
        open={Boolean(selectedShare)}
        onClose={() => setSelectedShare(null)}
        title={selectedShare?.name ?? ""}
        subtitle={selectedShare?.comment}
        footer={
          selectedShare ? (
            <Button variant="primary" className="w-full" onClick={() => setShareEditor({ share: selectedShare })}>
              {t("Edit settings")}
            </Button>
          ) : null
        }
      >
        {selectedShare ? (
          <div className="space-y-5">
            <div className="flex items-center justify-between rounded-xl border border-line bg-[var(--surface-2)]/60 p-4">
              <div>
                <p className="num text-[22px] font-semibold text-ink">{formatBytes(selectedShare.used)}</p>
                <p className="mt-1 text-[10.5px] text-subtle">{t("of {size}", { size: formatBytes(selectedShare.size) })}</p>
              </div>
              <span className="num text-sm text-soft">{formatPercent(selectedShare.usage, 1)}</span>
            </div>
            <Progress value={selectedShare.usage} height={7} />
            <div className="flex flex-wrap gap-1.5">
              <Badge tone={selectedShare.smb ? "info" : "neutral"}>
                {selectedShare.smb ? t("SMB on") : t("SMB off")}
              </Badge>
              <Badge tone={selectedShare.nfs ? "violet" : "neutral"}>
                {selectedShare.nfs ? t("NFS on") : t("NFS off")}
              </Badge>
              <Badge tone="accent">{t("Cache {mode}", { mode: cacheModeLabel(t, selectedShare.cacheMode) })}</Badge>
              {selectedShare.exclusive ? <Badge tone="success">{t("Exclusive")}</Badge> : null}
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Detail label={t("Allocator")} value={selectedShare.allocator} />
              <Detail label={t("Split level")} value={selectedShare.splitLevel} />
              <Detail label={t("Pool")} value={selectedShare.pool || t("Array")} />
              <Detail label={t("Security")} value={selectedShare.security} />
              <Detail label={t("Free")} value={formatBytes(selectedShare.free)} />
              <Detail label={t("Last modified")} value={relativeTime(selectedShare.lastModified)} />
            </div>
            <div className="rounded-xl border border-line p-3 text-[10.5px] leading-relaxed text-muted">
              {t("Shares are virtual folders that span one or more disks. Allocation happens automatically and can be tuned per share.")}
            </div>
          </div>
        ) : null}
      </Drawer>

      {shareEditor ? (
        <ShareEditModal
          share={shareEditor.share}
          onClose={() => {
            setShareEditor(null);
            setSelectedShare(null);
          }}
        />
      ) : null}
    </>
  );
}

/* -------------------------------------------------------------------------- */

function SummaryCard({
  icon,
  label,
  value,
  detail,
  children,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  detail: string;
  children?: ReactNode;
}) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">
          {icon}
        </span>
        <p className="text-[10.5px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className="num mt-3 text-[21px] font-semibold leading-none text-ink">{value}</p>
      <p className="mt-1.5 text-[10.5px] text-muted">{detail}</p>
      {children}
    </Card>
  );
}

function DiskGroup({
  label,
  disks,
  showIO,
  onSelect,
}: {
  label: string;
  disks: Disk[];
  showIO: boolean;
  onSelect: (disk: Disk) => void;
}) {
  const action = useAction();
  const { t } = useI18n();
  return (
    <div>
      <div className="flex items-center gap-2 border-b border-line bg-[var(--surface-2)]/45 px-5 py-1.5">
        <span className="text-[9px] font-semibold uppercase tracking-[0.14em] text-subtle">{label}</span>
        <span className="text-[9px] text-subtle/70">{disks.length}</span>
      </div>
      <div className="divide-y divide-line/60">
        {disks.map((disk) => {
          const usage = disk.size > 0 ? (disk.used / disk.size) * 100 : 0;
          return (
            <div
              key={disk.id}
              className="group grid grid-cols-[auto_1fr_auto] items-center gap-3 px-5 py-3 transition hover:bg-[var(--surface-2)]/45 sm:grid-cols-[auto_minmax(0,1.4fr)_minmax(0,1fr)_auto_auto_auto]"
            >
              <button
                type="button"
                onClick={() => onSelect(disk)}
                className="flex items-center gap-3 text-left"
              >
                <span className="relative grid size-9 place-items-center rounded-lg border border-line bg-[var(--surface-2)]">
                  <HardDrive className="size-4" style={{ color: disk.color }} />
                  <span
                    className={cn(
                      "absolute -right-0.5 -top-0.5 size-2 rounded-full border-2 border-[var(--surface)]",
                      disk.status === "standby" ? "bg-[var(--subtle)]" : "bg-[var(--success)]",
                    )}
                  />
                </span>
                <span className="min-w-0">
                  <span className="flex items-center gap-1.5">
                    <span className="text-[12px] font-medium text-ink">{disk.name}</span>
                    <span className="font-mono text-[9px] text-subtle">/dev/{disk.device}</span>
                  </span>
                  <span className="block max-w-[230px] truncate text-[9.5px] text-subtle">{disk.model}</span>
                </span>
              </button>

              <div className="hidden min-w-0 sm:block">
                <div className="flex items-center justify-between text-[9.5px]">
                  <span className="text-subtle">{formatBytes(disk.used)} / {formatBytes(disk.size)}</span>
                  <span className="num text-muted">{usage > 0 ? `${usage.toFixed(0)}%` : "—"}</span>
                </div>
                <Progress value={usage} height={4} className="mt-1" tone={disk.status === "standby" ? "var(--subtle)" : undefined} />
              </div>

              <div className="hidden text-right sm:block">
                <p className="num text-[10.5px] text-soft">{disk.temp > 0 ? `${disk.temp}°C` : "—"}</p>
                <p className="text-[8.5px] uppercase tracking-[0.1em] text-subtle">
                  {disk.status === "active"
                    ? t("Active")
                    : disk.status === "standby"
                      ? t("Standby")
                      : disk.status}
                </p>
              </div>

              <div className="hidden w-[76px] text-right md:block">
                {showIO ? (
                  <>
                    <p className="num text-[9.5px] text-[var(--info)]">{formatSpeed(disk.readBps)}</p>
                    <p className="num text-[9.5px] text-[var(--violet)]">{formatSpeed(disk.writeBps)}</p>
                  </>
                ) : (
                  <Badge tone={disk.smart === "passed" ? "success" : "warning"}>
                    {disk.smart === "passed" ? t("SMART") : t("Check")}
                  </Badge>
                )}
              </div>

              <div className="flex items-center gap-1">
                <IconButton
                  label={disk.status === "active" ? t("Spin down") : t("Spin up")}
                  size="sm"
                  onClick={() =>
                    action.mutate({
                      kind: disk.status === "active" ? "disk.spindown" : "disk.spinup",
                      target: disk.name,
                      success: disk.status === "active"
                        ? t("{name} spinning down", { name: disk.name })
                        : t("{name} spinning up", { name: disk.name }),
                    })
                  }
                >
                  {disk.status === "active" ? <ArrowDownToLine className="size-3.5" /> : <ArrowUpFromLine className="size-3.5" />}
                </IconButton>
                <Menu
                  trigger={
                    <IconButton label={t("Disk actions")} size="sm">
                      <span className="text-[14px] leading-none">⋯</span>
                    </IconButton>
                  }
                  items={[
                    { label: t("View details"), onSelect: () => onSelect(disk) },
                    {
                      label: disk.status === "active" ? t("Spin down") : t("Spin up"),
                      onSelect: () =>
                        action.mutate({
                          kind: disk.status === "active" ? "disk.spindown" : "disk.spinup",
                          target: disk.name,
                          success: disk.status === "active"
                            ? t("{name} spinning down", { name: disk.name })
                            : t("{name} spinning up", { name: disk.name }),
                        }),
                    },
                    { label: t("SMART report"), onSelect: () => onSelect(disk) },
                  ]}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ShareEditModal({ share, onClose }: { share: Share | null; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { snapshot } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const [draft, setDraft] = useState<ShareConfig | null>(null);
  const [error, setError] = useState("");
  const [includeText, setIncludeText] = useState("");
  const [excludeText, setExcludeText] = useState("");
  const [saving, setSaving] = useState(false);

  const config = useQuery({
    queryKey: ["share-config", share?.name ?? "new"],
    queryFn: () => api.shareConfig(share?.name ?? ""),
    enabled: Boolean(share),
    staleTime: 0,
  });

  useEffect(() => {
    if (share) return;
    setDraft({
      name: "", comment: "", floor: "0", allocator: "highwater", splitLevel: "",
      include: [], exclude: [], useCache: "no", cachePool: "", cachePool2: "",
      cow: "auto", exclusive: false, smbExport: "-", smbSecurity: "public",
      smbCaseSensitive: "auto", smbVolumeSizeLimit: "", nfsExport: "-",
      nfsSecurity: "public", nfsHostList: "",
    });
    setIncludeText("");
    setExcludeText("");
    setError("");
  }, [share]);

  useEffect(() => {
    if (!config.data) return;
    setDraft({ ...config.data });
    setIncludeText(config.data.include.join(", "));
    setExcludeText(config.data.exclude.join(", "));
    setError("");
  }, [config.data]);

  const update = (patch: Partial<ShareConfig>) => {
    setDraft((current) => (current ? { ...current, ...patch } : current));
    setError("");
  };

  const save = async () => {
    if (!draft) return;
    if (!/^[A-Za-z][A-Za-z0-9._-]{0,39}$/.test(draft.name)) {
      setError(t("Share name must start with a letter and contain only letters, digits, '.', '_' or '-'."));
      return;
    }
    const next: ShareConfig = {
      ...draft,
      include: includeText.split(",").map((item) => item.trim()).filter(Boolean),
      exclude: excludeText.split(",").map((item) => item.trim()).filter(Boolean),
    };
    setSaving(true);
    setError("");
    try {
      if (share) {
        await api.updateShare(share.name, next);
      } else {
        await api.createShare(next);
      }
      await api.updateShareSMB(next.name, next);
      await api.updateShareNFS(next.name, next);
      toast({
        kind: "success",
        title: share
          ? share.name === next.name
            ? t("Share updated")
            : t("Share renamed")
          : t("Share created"),
        description: t("A snapshot is available in History for rollback."),
      });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["config-history"] });
      onClose();
    } catch (saveError) {
      setError((saveError as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const pools = snapshot?.pools ?? [];

  return (
    <Modal
      open
      onClose={onClose}
      title={share ? t("Settings · {name}", { name: share.name }) : t("New share")}
      description={t("Share properties, SMB export and NFS export. The current configuration file is snapshotted before applying.")}
      size="xl"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            {t("Cancel")}
          </Button>
          <Button variant="primary" loading={saving} disabled={!draft} onClick={() => void save()}>
            {t("Apply changes")}
          </Button>
        </>
      }
    >
      {config.isLoading ? (
        <p className="text-[11px] text-muted">{t("Loading share configuration…")}</p>
      ) : config.isError ? (
        <ErrorLine message={(config.error as Error).message} />
      ) : draft ? (
        <div className="space-y-5">
          <section>
            <SectionTitle title={t("Share")} hint={t("Name, description and free-space floor")} />
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <TextInput label={t("Name")} value={draft.name} onChange={(value) => update({ name: value })} placeholder="media" />
              <TextInput label={t("Comment")} value={draft.comment} onChange={(value) => update({ comment: value })} />
              <TextInput label={t("Minimum free space")} value={draft.floor} onChange={(value) => update({ floor: value })} placeholder="0 or 10G" />
            </div>
            {share ? (
              <p className="mt-1 text-[9.5px] text-subtle">
                {t("Renaming moves the share configuration; existing data stays on disk.")}
              </p>
            ) : null}
          </section>

          <section>
            <SectionTitle title={t("Storage")} hint={t("Primary/secondary storage, cache mode and allocation")} />
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <SelectInput label={t("Primary storage")} value={draft.cachePool} onChange={(value) => update({ cachePool: value })}>
                <option value="">{t("Array")}</option>
                {pools.map((pool) => (
                  <option key={pool.name} value={pool.name}>{pool.name}</option>
                ))}
              </SelectInput>
              <SelectInput label={t("Secondary storage")} value={draft.cachePool2} onChange={(value) => update({ cachePool2: value })}>
                <option value="">{t("None / Array")}</option>
                {pools.map((pool) => (
                  <option key={pool.name} value={pool.name}>{pool.name}</option>
                ))}
              </SelectInput>
              <SelectInput label={t("Cache mode")} value={draft.useCache} onChange={(value) => update({ useCache: value })}>
                <option value="no">{t("No")}</option>
                <option value="yes">{t("Yes")}</option>
                <option value="prefer">{t("Prefer")}</option>
                <option value="only">{t("Only")}</option>
              </SelectInput>
              <SelectInput label={t("Allocation method")} value={draft.allocator} onChange={(value) => update({ allocator: value })}>
                <option value="highwater">{t("High-water")}</option>
                <option value="fillup">{t("Fill-up")}</option>
                <option value="mostfree">{t("Most-free")}</option>
              </SelectInput>
              <SelectInput label={t("Split level")} value={draft.splitLevel} onChange={(value) => update({ splitLevel: value })}>
                <option value="">{t("Automatic")}</option>
                {["1", "2", "3", "4", "5", "0"].map((level) => (
                  <option key={level} value={level}>
                    {level === "0" ? t("Manual") : t("Split at level {level}", { level })}
                  </option>
                ))}
              </SelectInput>
              <SelectInput label={t("Copy-on-write")} value={draft.cow} onChange={() => undefined} disabled>
                <option value="auto">{t("Auto")}</option>
                <option value="no">{t("No")}</option>
              </SelectInput>
            </div>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              <TextInput label={t("Included disks (comma separated)")} value={includeText} onChange={setIncludeText} placeholder="disk1, disk2" />
              <TextInput label={t("Excluded disks (comma separated)")} value={excludeText} onChange={setExcludeText} placeholder="disk3" />
            </div>
            <p className="mt-2 text-[9.5px] text-subtle">
              {t("Copy-on-write can only be chosen when creating a share.")}
            </p>
          </section>

          <section>
            <SectionTitle title={t("SMB")} hint={t("Windows/Samba export and security")} />
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
              <SelectInput label={t("Export")} value={draft.smbExport} onChange={(value) => update({ smbExport: value })}>
                <option value="-">{t("No")}</option>
                <option value="e">{t("Yes")}</option>
                <option value="eh">{t("Yes (hidden)")}</option>
                <option value="et">{t("Yes / Time Machine")}</option>
                <option value="eth">{t("Yes / Time Machine (hidden)")}</option>
              </SelectInput>
              <SelectInput label={t("Security")} value={draft.smbSecurity} onChange={(value) => update({ smbSecurity: value })}>
                <option value="public">{t("Public")}</option>
                <option value="secure">{t("Secure")}</option>
                <option value="private">{t("Private")}</option>
              </SelectInput>
              <SelectInput label={t("Case-sensitive names")} value={draft.smbCaseSensitive} onChange={(value) => update({ smbCaseSensitive: value })}>
                <option value="auto">{t("Auto")}</option>
                <option value="yes">{t("Yes")}</option>
                <option value="forced">{t("Force lower")}</option>
              </SelectInput>
              <TextInput label={t("Time Machine limit (MB)")} value={draft.smbVolumeSizeLimit} onChange={(value) => update({ smbVolumeSizeLimit: value })} placeholder="0" />
            </div>
          </section>

          <section>
            <SectionTitle title={t("NFS")} hint={t("Network file system export and rules")} />
            <div className="grid gap-2 sm:grid-cols-2">
              <SelectInput label={t("Export")} value={draft.nfsExport} onChange={(value) => update({ nfsExport: value })}>
                <option value="-">{t("No")}</option>
                <option value="e">{t("Yes")}</option>
              </SelectInput>
              <SelectInput label={t("Security")} value={draft.nfsSecurity} onChange={(value) => update({ nfsSecurity: value })}>
                <option value="public">{t("Public")}</option>
                <option value="secure">{t("Secure")}</option>
                <option value="private">{t("Private")}</option>
              </SelectInput>
            </div>
            {draft.nfsSecurity === "private" ? (
              <label className="mt-2 block space-y-1">
                <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">
                  {t("NFS rules")}
                </span>
                <textarea
                  value={draft.nfsHostList}
                  onChange={(event) => update({ nfsHostList: event.target.value })}
                  rows={4}
                  placeholder={t("*(rw,sec=sys,insecure,anongid=100,anonuid=99,no_root_squash,lock)")}
                  className="w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 py-2 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
                />
              </label>
            ) : null}
          </section>

          <div className="flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
            <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
            {t("The share configuration is snapshotted before the change. Use")}{" "}
            <span className="text-soft">{t("History")}</span>{" "}
            {t("to review the diff or roll back.")}
          </div>

          {error ? <ErrorLine message={error} /> : null}
        </div>
      ) : null}
    </Modal>
  );
}

function SectionTitle({ title, hint }: { title: string; hint: string }) {
  return (
    <div className="mb-2.5">
      <h3 className="text-[11.5px] font-semibold uppercase tracking-[0.1em] text-soft">{title}</h3>
      <p className="mt-0.5 text-[9.5px] text-subtle">{hint}</p>
    </div>
  );
}

function TextInput({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  return (
    <label className="block space-y-1">
      <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{label}</span>
      <input
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]"
      />
    </label>
  );
}

function SelectInput({
  label,
  value,
  onChange,
  children,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  children: ReactNode;
  disabled?: boolean;
}) {
  return (
    <label className="block space-y-1">
      <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{label}</span>
      <select
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)] disabled:opacity-50"
      >
        {children}
      </select>
    </label>
  );
}

function ErrorLine({ message }: { message: string }) {
  return (
    <p className="rounded-lg border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2 text-[10.5px] text-[var(--danger)]">
      {message}
    </p>
  );
}

function DiskSmartSection({ device }: { device: string }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const smart = useQuery({
    queryKey: ["smart", device],
    queryFn: () => api.diskSmart(device),
    staleTime: 30_000,
  });
  const test = useMutation({
    mutationFn: () => api.runSmartTest(device),
    onSuccess: () => {
      toast({
        kind: "success",
        title: t("SMART test started"),
        description: t("The short self-test runs in the background."),
      });
      void queryClient.invalidateQueries({ queryKey: ["smart", device] });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("SMART test failed"), description: error.message });
    },
  });

  const report = smart.data;

  return (
    <div className="rounded-xl border border-line">
      <div className="flex items-center justify-between gap-3 border-b border-line px-3 py-2.5">
        <p className="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-[0.1em] text-subtle">
          <ShieldCheck className="size-3.5" /> {t("SMART")}
        </p>
        {report ? (
          <Badge tone={report.passed ? "success" : report.available ? "danger" : "neutral"} dot>
            {report.available
              ? report.passed
                ? t("passed")
                : t("failing")
              : t("unavailable")}
          </Badge>
        ) : null}
      </div>
      <div className="space-y-3 p-3">
        {smart.isLoading ? (
          <p className="text-[10.5px] text-muted">{t("Reading SMART data…")}</p>
        ) : smart.isError ? (
          <p className="text-[10.5px] text-[var(--danger)]">{(smart.error as Error).message}</p>
        ) : report ? (
          <>
            {!report.available ? (
              <p className="text-[10.5px] text-muted">
                {report.message || t("SMART data is not available for this disk.")}
              </p>
            ) : (
              <>
                <div className="grid grid-cols-3 gap-2">
                  <Detail label={t("Temperature")} value={report.temperature ? `${report.temperature}°C` : "—"} />
                  <Detail label={t("Power-on")} value={report.powerOnHours ? `${report.powerOnHours} h` : "—"} />
                  <Detail label={t("Self-test")} value={report.selfTest || "—"} />
                </div>
                {report.attributes.length > 0 ? (
                  <div className="max-h-60 overflow-y-auto rounded-lg border border-line/70">
                    <table className="w-full text-left">
                      <thead className="sticky top-0 bg-[var(--surface-2)] text-[8.5px] uppercase tracking-[0.08em] text-subtle">
                        <tr>
                          <th className="px-2 py-1.5 font-medium">{t("Attribute")}</th>
                          <th className="px-2 py-1.5 text-right font-medium">{t("Value")}</th>
                          <th className="px-2 py-1.5 text-right font-medium">{t("Worst")}</th>
                          <th className="px-2 py-1.5 text-right font-medium">{t("Thr.")}</th>
                          <th className="px-2 py-1.5 text-right font-medium">{t("Raw")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {report.attributes.map((attribute, index) => (
                          <tr
                            key={`${attribute.id}-${attribute.name}-${index}`}
                            className={cn("border-t border-line/50 text-[10px]", attribute.failing && "bg-[var(--danger-soft)] text-[var(--danger)]")}
                          >
                            <td className="px-2 py-1.5 text-soft">{attribute.name}</td>
                            <td className="px-2 py-1.5 text-right num text-muted">{attribute.value || "—"}</td>
                            <td className="px-2 py-1.5 text-right num text-muted">{attribute.worst || "—"}</td>
                            <td className="px-2 py-1.5 text-right num text-muted">{attribute.threshold || "—"}</td>
                            <td className="px-2 py-1.5 text-right font-mono text-[9.5px] text-subtle">{attribute.raw}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : null}
              </>
            )}
            <div className="flex items-center justify-between gap-2">
              <p className="text-[9.5px] text-subtle">
                {t("Serial {serial} · firmware {firmware}", {
                  serial: report.serial || "—",
                  firmware: report.firmware || "—",
                })}
              </p>
              <Button
                size="sm"
                variant="secondary"
                icon={<Play className="size-3" />}
                loading={test.isPending}
                disabled={!report.available}
                onClick={() => test.mutate()}
              >
                {t("Short test")}
              </Button>
            </div>
          </>
        ) : null}
      </div>
    </div>
  );
}

function ShareCard({ share, onOpen }: { share: Share; onOpen: () => void }) {
  const { t } = useI18n();
  return (
    <Card hover className="card-pad cursor-pointer" onClick={onOpen}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <span className="grid size-8 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-[var(--accent)]">
            <FolderTree className="size-4" />
          </span>
          <div>
            <h3 className="text-[13px] font-semibold text-ink">{share.name}</h3>
            <p className="mt-0.5 line-clamp-1 text-[10px] text-subtle">{share.comment}</p>
          </div>
        </div>
        <span className="num text-[11px] text-muted">{share.usage.toFixed(0)}%</span>
      </div>
      <div className="mt-4">
        <Progress value={share.usage} height={6} />
        <div className="mt-2 flex justify-between font-mono text-[9.5px] text-subtle">
          <span>{t("{size} used", { size: formatBytes(share.used) })}</span>
          <span>{t("{size} free", { size: formatBytes(share.free) })}</span>
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-1.5">
        {share.smb ? <Badge tone="info">{t("SMB")}</Badge> : null}
        {share.nfs ? <Badge tone="violet">{t("NFS")}</Badge> : null}
        <Badge tone="neutral">{t("cache {mode}", { mode: cacheModeLabel(t, share.cacheMode) })}</Badge>
        {share.exclusive ? <Badge tone="success">{t("exclusive")}</Badge> : null}
      </div>
    </Card>
  );
}

function DataRow({ label, value, tone }: { label: string; value: string; tone?: "warning" | "success" }) {
  return (
    <div className="flex items-center justify-between gap-3 text-[11px]">
      <span className="text-muted">{label}</span>
      <span
        className={cn(
          "num max-w-[65%] truncate text-right text-soft",
          tone === "warning" && "text-[var(--warning)]",
          tone === "success" && "text-[var(--success)]",
        )}
      >
        {value}
      </span>
    </div>
  );
}

function Detail({ label, value, icon }: { label: string; value: string; icon?: ReactNode }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2">
      <p className="flex items-center gap-1 text-[9px] uppercase tracking-[0.1em] text-subtle">
        {icon}
        {label}
      </p>
      <p className="num mt-1 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Mover                                                                       */
/* -------------------------------------------------------------------------- */

function MoverCard({ arrayRunning }: { arrayRunning: boolean }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const [busy, setBusy] = useState(false);
  const [filesRun, setFilesRun] = useState<MoverRun | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);
  const statusQuery = useQuery({
    queryKey: ["mover"],
    queryFn: api.mover,
    refetchInterval: 1500,
  });
  const status = statusQuery.data;
  const running = Boolean(status?.running);
  const phaseIndex = MOVER_PHASES.findIndex((phase) => phase.key === status?.phase);
  const shareProgress = status && status.shareTotal > 0
    ? ((status.shareTotal - status.shareRemain) / status.shareTotal) * 100
    : 0;

  const run = async (action: "start" | "stop") => {
    setBusy(true);
    try {
      if (action === "start") {
        await api.startMover();
        toast({
          kind: "success",
          title: t("Mover started"),
          description: t("It runs in the background; you can leave this page."),
        });
      } else {
        await api.stopMover();
        toast({ kind: "info", title: t("Mover stopped") });
      }
      await queryClient.invalidateQueries({ queryKey: ["mover"] });
    } catch (error) {
      toast({
        kind: "error",
        title: action === "start" ? t("Unable to start mover") : t("Unable to stop mover"),
        description: (error as Error).message,
      });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card className="card-pad">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <span className="grid size-8 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-[var(--violet)]">
            <ArrowRightLeft className="size-4" />
          </span>
          <div>
            <h2 className="text-[13px] font-semibold text-ink">{t("Mover")}</h2>
            <p className="mt-0.5 text-[10.5px] text-subtle">
              {t("Move cache data to the array and back, in the background")}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Badge tone={running ? "accent" : "neutral"} dot pulse={running}>
            {running ? t("Running") : t("Ready")}
          </Badge>
          {running ? (
            <Button size="sm" variant="danger" icon={<Square className="size-3" />} loading={busy} onClick={() => void run("stop")}>
              {t("Stop")}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="primary"
              icon={<Play className="size-3" />}
              disabled={!arrayRunning || busy}
              loading={busy}
              onClick={() => void run("start")}
            >
              {t("Start mover")}
            </Button>
          )}
        </div>
      </div>

      {running ? (
        <div className="mt-4 space-y-3">
          <div className="grid grid-cols-4 gap-1.5">
            {MOVER_PHASES.map((phase, index) => {
              const active = index === phaseIndex || (phaseIndex < 0 && index === 0);
              const done = phaseIndex > index;
              return (
                <div
                  key={phase.key}
                  className={cn(
                    "rounded-lg border px-2 py-1.5 text-center text-[9px] font-medium uppercase tracking-[0.08em]",
                    active
                      ? "border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)]"
                      : done
                        ? "border-[color-mix(in_srgb,var(--success)_30%,transparent)] bg-[var(--success-soft)] text-[var(--success)]"
                        : "border-line bg-[var(--surface-2)]/40 text-subtle",
                  )}
                >
                  {t(phase.label)}
                </div>
              );
            })}
          </div>
          <div className="flex items-baseline justify-between gap-3">
            <p className="min-w-0 truncate text-[11.5px] text-soft">
              {status?.action || (status?.files?.length ? t("Moving files") : t("Calculating totals…"))}
              {status?.file ? <span className="ml-2 font-mono text-[10px] text-subtle">{status.file}</span> : null}
            </p>
            <span className="num shrink-0 text-[12px] text-ink">
              {status?.total
                ? `${status?.progress.toFixed(1)}%`
                : status?.files?.length
                  ? status.files.length === 1
                    ? t("1 file moved")
                    : t("{count} files moved", { count: status.files.length })
                  : `${status?.progress.toFixed(1)}%`}
            </span>
          </div>
          <Progress value={status?.progress ?? 0} height={7} />
          <div className="grid grid-cols-2 gap-3 text-[10px] sm:grid-cols-4">
            <MoverStat label={t("Moved")} value={formatBytes(Math.max(0, (status?.total ?? 0) - (status?.remain ?? 0)))} />
            <MoverStat label={t("Remaining")} value={formatBytes(status?.remain ?? 0)} />
            <MoverStat label={t("Throughput")} value={formatSpeed(status?.throughput ?? 0)} />
            <MoverStat label={t("ETA")} value={status && status.eta > 0 ? formatDuration(status.eta) : "—"} />
          </div>
          {status?.files?.length ? (
            <button
              type="button"
              onClick={() => setFilesRun({ started: "", finished: "", duration: 0, total: 0, moved: 0, status: "running", files: status.files })}
              className="text-left text-[10px] text-subtle underline decoration-dotted underline-offset-2 hover:text-soft"
            >
              {status.files.length === 1
                ? t("{count} moved file observed", { count: status.files.length })
                : t("{count} moved files observed", { count: status.files.length })}
            </button>
          ) : null}
          {status && status.shareTotal > 0 ? (
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/45 p-3">
              <div className="flex items-center justify-between text-[10px]">
                <span className="text-muted">{t("Current item")}</span>
                <span className="num text-subtle">{shareProgress.toFixed(1)}% · {formatSpeed(status.shareThroughput)}</span>
              </div>
              <Progress value={shareProgress} height={4} className="mt-2" tone="var(--violet)" />
            </div>
          ) : null}
          {!status?.total ? (
            <p className="text-[10px] text-subtle">
              {t("Byte progress is disabled in Unraid (Settings → Scheduler → Mover). File tracking still works for runs started from Aurora.")}
            </p>
          ) : null}
        </div>
      ) : (
        <p className="mt-4 text-[11px] leading-relaxed text-muted">
          {t("The mover honours each share's cache mode:")}{" "}
          <span className="text-soft">{t("Yes")}</span>{" "}
          {t("moves to the array,")}{" "}
          <span className="text-soft">{t("Prefer")}</span>{" "}
          {t("brings data back to the pool. Progress is read from Unraid's")}
          <span className="font-mono text-[10px]"> mover.ini</span>{" "}
          {t("and survives a page refresh.")}
        </p>
      )}

      {status?.history?.length ? (
        <div className="mt-4 border-t border-line">
          <button
            type="button"
            onClick={() => setHistoryOpen((value) => !value)}
            aria-expanded={historyOpen}
            className="flex w-full items-center gap-1.5 py-2.5 text-left transition hover:text-ink"
          >
            <History className="size-3 text-subtle" />
            <span className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Recent runs")}</span>
            <span className="ml-auto text-[9.5px] text-subtle">{status.history.length}</span>
            <ChevronDown className={cn("size-3 text-subtle transition-transform", historyOpen && "rotate-180")} />
          </button>
          {historyOpen ? (
            <div className="space-y-1.5 pb-1">
              {status.history.slice(0, 10).map((run, index) => (
                <div key={`${run.finished}-${index}`} className="flex flex-wrap items-center justify-between gap-2 text-[10px]">
                  <span className="text-muted">{formatDateTime(run.finished)}</span>
                  <span className="num text-subtle">{formatDuration(run.duration)} · {formatBytes(run.moved)}</span>
                  {run.files?.length ? (
                    <button
                      type="button"
                      onClick={() => setFilesRun(run)}
                      className="text-[10px] text-[var(--accent)] underline decoration-dotted underline-offset-2"
                    >
                      {run.files.length === 1
                        ? t("{count} file", { count: run.files.length })
                        : t("{count} files", { count: run.files.length })}
                    </button>
                  ) : (
                    <span className="text-[9.5px] text-subtle">{t("no file list")}</span>
                  )}
                  <Badge tone={run.status === "completed" ? "success" : "warning"}>
                    {run.status === "completed"
                      ? t("Completed")
                      : run.status === "failed"
                        ? t("Failed")
                        : run.status}
                  </Badge>
                </div>
              ))}
              <p className="pt-1 text-[9px] text-subtle">{t("File lists are captured for runs started from Aurora.")}</p>
            </div>
          ) : null}
        </div>
      ) : null}

      <Modal
        open={Boolean(filesRun)}
        onClose={() => setFilesRun(null)}
        title={t("Moved files")}
        description={filesRun?.finished ? formatDateTime(filesRun.finished) : t("Run in progress")}
        size="lg"
        footer={
          <Button variant="secondary" onClick={() => setFilesRun(null)}>
            {t("Close")}
          </Button>
        }
      >
        {filesRun?.files?.length ? (
          <div className="max-h-[55vh] space-y-1 overflow-y-auto">
            {filesRun.files.map((file, index) => (
              <p
                key={`${file}-${index}`}
                className="truncate rounded-md border border-line bg-[var(--surface-2)]/50 px-2.5 py-1.5 font-mono text-[9.5px] text-soft"
              >
                {file}
              </p>
            ))}
          </div>
        ) : (
          <p className="text-[11px] text-muted">{t("No file was recorded for this run.")}</p>
        )}
      </Modal>
    </Card>
  );
}

function MoverStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/45 px-2.5 py-2">
      <p className="text-[8.5px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="num mt-0.5 truncate text-[10.5px] text-soft">{value}</p>
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Array start / stop                                                          */
/* -------------------------------------------------------------------------- */

function ArrayStartModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const [keyType, setKeyType] = useState<"none" | "passphrase" | "keyfile">("none");
  const [passphrase, setPassphrase] = useState("");
  const [keyfile, setKeyfile] = useState("");
  const [maintenance, setMaintenance] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setKeyType("none");
    setPassphrase("");
    setKeyfile("");
    setMaintenance(false);
    setError("");
  }, [open]);

  const readKeyfile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    if (file.size > 1024 * 1024) {
      setError(t("The keyfile must be smaller than 1 MiB."));
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const result = String(reader.result ?? "");
      setKeyfile(result.includes(",") ? result.split(",")[1] : result);
      setError("");
    };
    reader.onerror = () => setError(t("Unable to read the keyfile."));
    reader.readAsDataURL(file);
  };

  const start = async () => {
    if (keyType === "passphrase" && !passphrase) {
      setError(t("Enter the array passphrase."));
      return;
    }
    if (keyType === "keyfile" && !keyfile) {
      setError(t("Select the encryption keyfile."));
      return;
    }
    setSaving(true);
    setError("");
    try {
      await api.startArray({
        keyType,
        key: keyType === "passphrase" ? passphrase : keyType === "keyfile" ? keyfile : "",
        mode: maintenance ? "maintenance" : "normal",
      });
      await queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      await queryClient.invalidateQueries({ queryKey: ["mover"] });
      toast({
        kind: "success",
        title: t("Array start requested"),
        description: t("The array is starting."),
      });
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("Start the array")}
      description={t("Provide the encryption key when the array uses LUKS.")}
      size="md"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>{t("Cancel")}</Button>
          <Button variant="primary" icon={<Play className="size-3.5" />} loading={saving} onClick={() => void start()}>
            {t("Start array")}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <label htmlFor="array-key-type" className="mb-1.5 block text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
            {t("Encryption key")}
          </label>
          <select
            id="array-key-type"
            value={keyType}
            onChange={(event) => { setKeyType(event.target.value as typeof keyType); setError(""); }}
            className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11.5px] text-soft outline-none focus:border-[var(--accent-line)]"
          >
            <option value="none">{t("No key (unencrypted array)")}</option>
            <option value="passphrase">{t("Passphrase")}</option>
            <option value="keyfile">{t("Keyfile")}</option>
          </select>
        </div>
        {keyType === "passphrase" ? (
          <div>
            <label htmlFor="array-passphrase" className="mb-1.5 block text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
              {t("Passphrase")}
            </label>
            <div className="flex h-9 items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)] px-2.5">
              <LockKeyhole className="size-3.5 text-subtle" />
              <input
                id="array-passphrase"
                type="password"
                value={passphrase}
                onChange={(event) => setPassphrase(event.target.value)}
                placeholder={t("Printable ASCII characters")}
                className="h-full w-full bg-transparent text-[12px] text-ink outline-none placeholder:text-subtle"
              />
            </div>
            <p className="mt-1 text-[9.5px] text-subtle">
              {t("Unraid accepts printable ASCII characters from space to tilde.")}
            </p>
          </div>
        ) : null}
        {keyType === "keyfile" ? (
          <div>
            <label htmlFor="array-keyfile" className="mb-1.5 block text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
              {t("Keyfile")}
            </label>
            <input
              id="array-keyfile"
              type="file"
              onChange={readKeyfile}
              className="block w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 py-2 text-[10.5px] text-muted file:mr-3 file:rounded-md file:border-0 file:bg-[var(--surface-3)] file:px-2 file:py-1 file:text-[10px] file:text-soft"
            />
            <p className="mt-1 text-[9.5px] text-subtle">
              {t("The file is uploaded to the keyfile path configured by Unraid.")}
            </p>
          </div>
        ) : null}
        <label className="flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
          <span>
            <span className="block text-[11px] font-medium text-soft">{t("Maintenance mode")}</span>
            <span className="block text-[9.5px] text-subtle">{t("Start the array without mounting disks.")}</span>
          </span>
          <Switch checked={maintenance} onChange={setMaintenance} label={t("Maintenance mode")} />
        </label>
        {error ? (
          <div className="rounded-lg border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] text-[var(--danger)]">
            {error}
          </div>
        ) : null}
      </div>
    </Modal>
  );
}

function ArrayStopModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { t } = useI18n();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const stop = async () => {
    setSaving(true);
    setError("");
    try {
      await api.stopArray();
      await queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      await queryClient.invalidateQueries({ queryKey: ["mover"] });
      toast({
        kind: "info",
        title: t("Array stop requested"),
        description: t("The array is stopping."),
      });
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("Stop the array?")}
      description={t("Services and shares will become unavailable until the array is started again.")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>{t("Cancel")}</Button>
          <Button variant="danger" icon={<Square className="size-3.5" />} loading={saving} onClick={() => void stop()}>
            {t("Stop array")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <p className="text-[11px] leading-relaxed text-muted">
          {t("Unraid will flush pending writes and stop the array. Encryption keys are not stored by Aurora.")}
        </p>
        {error ? (
          <div className="rounded-lg border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] text-[var(--danger)]">
            {error}
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
