import { useEffect, useMemo, useState, type DragEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CheckCircle2,
  GripVertical,
  HardDrive,
  Info,
  Search,
  Sparkles,
  Trash2,
  Undo2,
} from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { formatBytes, hexToRgba } from "../../lib/format";
import { Badge, Button, Card, EmptyState, InputBase, cn } from "../ui/primitives";
import type { Disk, StorageLayout } from "../../lib/types";

const BAY_OPTIONS = [8, 12, 16, 20, 24, 32, 40, 48];

type Assignments = Record<string, number>;

export function StorageLocation({
  disks,
  onOpenDisk,
}: {
  disks: Disk[];
  onOpenDisk: (disk: Disk) => void;
}) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const physical = useMemo(() => disks.filter((disk) => disk.type !== "Flash"), [disks]);
  const defaultBays = Math.max(12, Math.ceil(physical.length / 4) * 4);
  const [bays, setBays] = useState(defaultBays);
  const [assignments, setAssignments] = useState<Assignments>({});
  const [initialized, setInitialized] = useState(false);
  const [dragging, setDragging] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [overBay, setOverBay] = useState<number | null>(null);
  const [query, setQuery] = useState("");

  const layoutQuery = useQuery({
    queryKey: ["storage-layout"],
    queryFn: api.storageLayout,
  });

  useEffect(() => {
    if (!layoutQuery.data || initialized) return;
    const names = new Set(physical.map((disk) => disk.name));
    const next: Assignments = {};
    for (const [name, bay] of Object.entries(layoutQuery.data.assignments ?? {})) {
      if (names.has(name) && bay >= 0 && bay < layoutQuery.data.bays) next[name] = bay;
    }
    setBays(Math.max(4, layoutQuery.data.bays || defaultBays));
    setAssignments(next);
    setInitialized(true);
  }, [defaultBays, initialized, layoutQuery.data, physical]);

  const saveLayout = useMutation({
    mutationFn: (layout: StorageLayout) => api.saveStorageLayout(layout),
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not save layout"), description: error.message });
    },
  });
  const retry = () => {
    setInitialized(false);
    void queryClient.invalidateQueries({ queryKey: ["storage-layout"] });
  };

  const byName = useMemo(() => new Map(physical.map((disk) => [disk.name, disk])), [physical]);
  const diskByBay = useMemo(() => {
    const map = new Map<number, Disk>();
    for (const [name, bay] of Object.entries(assignments)) {
      const disk = byName.get(name);
      if (disk) map.set(bay, disk);
    }
    return map;
  }, [assignments, byName]);

  const unplacedAll = useMemo(
    () => physical.filter((disk) => !(disk.name in assignments)),
    [assignments, physical],
  );
  const unplaced = useMemo(
    () =>
      unplacedAll.filter((disk) =>
        !query.trim() ||
        `${disk.name} ${disk.model} ${disk.device} ${disk.type}`.toLowerCase().includes(query.toLowerCase()),
      ),
    [query, unplacedAll],
  );

  const parked = physical.length - unplacedAll.length;
  const empty = Math.max(0, bays - parked);
  const availableBayOptions = [...new Set([...BAY_OPTIONS, bays, defaultBays])]
    .filter((value) => value >= 4 && value <= 48)
    .sort((a, b) => a - b);

  const persist = (nextBays: number, nextAssignments: Assignments) => {
    setBays(nextBays);
    setAssignments(nextAssignments);
    saveLayout.mutate({ bays: nextBays, assignments: nextAssignments, updated: "" });
  };

  const place = (name: string, bay: number) => {
    if (!byName.has(name)) return;
    const from = assignments[name];
    if (from === bay) return;
    const occupant = diskByBay.get(bay);
    const next = { ...assignments };
    if (occupant && occupant.name !== name) {
      if (from === undefined) delete next[occupant.name];
      else next[occupant.name] = from;
    }
    delete next[name];
    next[name] = bay;
    setSelected(null);
    setDragging(null);
    persist(bays, next);
  };

  const unassign = (name: string) => {
    const next = { ...assignments };
    delete next[name];
    setSelected(null);
    persist(bays, next);
  };

  const changeBays = (nextBays: number) => {
    const next = Object.fromEntries(
      Object.entries(assignments).filter(([, bay]) => bay < nextBays),
    );
    persist(nextBays, next);
  };

  const autoPlace = () => {
    const priority: Record<string, number> = { Parity: 0, Data: 1, Cache: 2 };
    const sorted = [...physical].sort(
      (a, b) =>
        (priority[a.type] ?? 9) - (priority[b.type] ?? 9) ||
        a.position - b.position ||
        a.name.localeCompare(b.name),
    );
    const next: Assignments = {};
    sorted.slice(0, bays).forEach((disk, index) => {
      next[disk.name] = index;
    });
    setSelected(null);
    persist(bays, next);
  };

  const reset = () => {
    setSelected(null);
    persist(bays, {});
  };

  if (physical.length === 0) {
    return (
      <Card>
        <EmptyState
          icon={<HardDrive className="size-5" />}
          title={t("No physical disks found")}
          description={t("The location map becomes available when Unraid reports array or pool disks.")}
        />
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/50 px-3 py-2.5">
        <Info className="size-3.5 shrink-0 text-[var(--accent)]" />
        <p className="min-w-0 flex-1 text-[10.5px] leading-relaxed text-muted">
          {t("Drag a disk into a bay, or select a disk then click a bay.")}{" "}
          <span className="text-subtle">{t("Visual layout only — Unraid disk assignments are unchanged.")}</span>
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-1.5 text-[10px] text-subtle">
            {t("Bays")}
            <select
              value={bays}
              onChange={(event) => changeBays(Number(event.target.value))}
              className="h-8 rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              {availableBayOptions.map((value) => (
                <option key={value} value={value}>{value}</option>
              ))}
            </select>
          </label>
          <Button size="sm" variant="secondary" icon={<Sparkles className="size-3" />} onClick={autoPlace}>
            {t("Auto-place")}
          </Button>
          <Button size="sm" variant="ghost" icon={<Undo2 className="size-3" />} onClick={reset}>
            {t("Reset layout")}
          </Button>
        </div>
      </div>

      {layoutQuery.isError ? (
        <div className="flex items-center gap-3 rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] text-[var(--danger)]">
          <span className="min-w-0 flex-1">{t("Could not load layout")}</span>
          <Button size="sm" variant="secondary" onClick={retry}>{t("Retry")}</Button>
        </div>
      ) : null}

      {bays < physical.length ? (
        <p className="text-[10px] text-[var(--warning)]">
          {t("This chassis has fewer bays than disks. Increase the bay count to place every disk.")}
        </p>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_310px]">
        <div className="relative overflow-hidden rounded-[26px] border border-line bg-[linear-gradient(145deg,var(--surface-2),var(--surface))] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_30px_80px_-50px_rgba(0,0,0,0.9)]">
          <div className="pointer-events-none absolute inset-y-5 left-2 w-1.5 rounded-full bg-[var(--surface-3)]" />
          <div className="pointer-events-none absolute inset-y-5 right-2 w-1.5 rounded-full bg-[var(--surface-3)]" />
          <div className="relative rounded-2xl border border-line bg-[var(--surface)]/60 p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <HardDrive className="size-3.5 text-[var(--accent)]" />
                <span className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Front view")}</span>
              </div>
              <span className="text-[9.5px] text-subtle">
                {t("{placed} placed · {empty} empty", { placed: parked, empty })}
              </span>
            </div>

            <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-4">
              {Array.from({ length: bays }, (_, bay) => {
                const disk = diskByBay.get(bay);
                const activeDrop = overBay === bay;
                return disk ? (
                  <AssignedBay
                    key={bay}
                    bay={bay}
                    disk={disk}
                    dragActive={dragging === disk.name}
                    onSelect={() => setSelected(selected === disk.name ? null : disk.name)}
                    onRemove={() => unassign(disk.name)}
                    onOpen={() => onOpenDisk(disk)}
                    onDragStart={() => setDragging(disk.name)}
                    onDragEnd={() => setDragging(null)}
                    onDragOver={(event) => {
                      event.preventDefault();
                      setOverBay(bay);
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      const name = dragging ?? selected;
                      if (name) place(name, bay);
                      setOverBay(null);
                    }}
                    onDragLeave={() => setOverBay(null)}
                  />
                ) : (
                  <button
                    key={bay}
                    type="button"
                    onClick={() => selected && place(selected, bay)}
                    onDragOver={(event) => {
                      event.preventDefault();
                      setOverBay(bay);
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      const name = dragging ?? selected;
                      if (name) place(name, bay);
                      setOverBay(null);
                    }}
                    onDragLeave={() => setOverBay(null)}
                    className={cn(
                      "flex min-h-[94px] flex-col justify-between rounded-xl border border-dashed p-2.5 text-left transition",
                      activeDrop
                        ? "border-[var(--accent-line)] bg-[var(--accent-soft)]"
                        : "border-line/80 bg-[var(--surface-3)]/20 hover:border-line-strong hover:bg-[var(--surface-3)]/40",
                    )}
                  >
                    <span className="flex items-center justify-between text-[8.5px] uppercase tracking-[0.1em] text-subtle">
                      <span>{t("Bay {index}", { index: bay + 1 })}</span>
                      <span className="size-1.5 rounded-full bg-[var(--subtle)]/30" />
                    </span>
                    <span className="text-center text-[9.5px] text-subtle">
                      {activeDrop ? t("Drop here") : t("Empty")}
                    </span>
                  </button>
                );
              })}
            </div>

            <div className="mt-4 flex items-center gap-3 border-t border-line/70 pt-2.5 text-[8.5px] uppercase tracking-[0.1em] text-subtle">
              <span className="flex items-center gap-1.5">
                <span className="size-1.5 rounded-full bg-[var(--success)]" /> {t("Active")}
              </span>
              <span className="flex items-center gap-1.5">
                <span className="size-1.5 rounded-full bg-[var(--warning)]" /> {t("Warning")}
              </span>
              <span className="ml-auto font-mono">Aurora · {bays}× HDD</span>
            </div>
          </div>
        </div>

        <Card className="p-3 xl:sticky xl:top-24 xl:self-start">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-[11px] font-semibold text-ink">{t("Disks to place")}</h3>
            <Badge tone={unplaced.length ? "neutral" : "success"}>{unplaced.length}</Badge>
          </div>
          <div className="mt-2">
            <InputBase
              value={query}
              onChange={setQuery}
              placeholder={t("Search disks…")}
              icon={<Search className="size-3.5" />}
            />
          </div>
          {unplacedAll.length === 0 ? (
            <div className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-[var(--success-soft)] px-3 py-2.5 text-[10px] text-[var(--success)]">
              <CheckCircle2 className="mt-0.5 size-3.5 shrink-0" />
              {t("All disks are placed")}
            </div>
          ) : unplaced.length === 0 ? (
            <p className="mt-3 text-[10px] text-subtle">{t("No disks match this search.")}</p>
          ) : (
            <div className="mt-2 max-h-[560px] space-y-1.5 overflow-y-auto pr-1">
              {unplaced.map((disk) => (
                <DiskCard
                  key={disk.name}
                  disk={disk}
                  selected={selected === disk.name}
                  onSelect={() => setSelected(selected === disk.name ? null : disk.name)}
                  onOpen={() => onOpenDisk(disk)}
                  onDragStart={() => setDragging(disk.name)}
                  onDragEnd={() => setDragging(null)}
                />
              ))}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}

function AssignedBay({
  bay,
  disk,
  dragActive,
  onSelect,
  onRemove,
  onOpen,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
  onDragLeave,
}: {
  bay: number;
  disk: Disk;
  dragActive: boolean;
  onSelect: () => void;
  onRemove: () => void;
  onOpen: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDragOver: (event: DragEvent<HTMLDivElement>) => void;
  onDrop: (event: DragEvent<HTMLDivElement>) => void;
  onDragLeave: () => void;
}) {
  const { t } = useI18n();
  const tone = disk.type === "Parity" ? "violet" : disk.type === "Cache" ? "info" : "neutral";
  return (
    <div
      role="button"
      tabIndex={0}
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragLeave={onDragLeave}
      onClick={onSelect}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
      style={{
        borderColor: hexToRgba(disk.color || "#8a8f98", 0.4),
        background: `linear-gradient(145deg, ${hexToRgba(disk.color || "#8a8f98", 0.16)}, color-mix(in srgb, var(--surface-3) 55%, transparent))`,
      }}
      className={cn(
        "flex min-h-[94px] cursor-grab flex-col justify-between rounded-xl border p-2.5 text-left transition active:cursor-grabbing",
        dragActive && "opacity-50",
      )}
      title={`${disk.name} · ${t("Drag to move")}`}
    >
      <div className="flex items-center justify-between gap-1">
        <span className="font-mono text-[8.5px] uppercase tracking-[0.1em] text-subtle">{t("Bay {index}", { index: bay + 1 })}</span>
        <span className="flex items-center gap-1">
          <span className="size-1.5 rounded-full" style={{ background: disk.color || "#8a8f98" }} />
          <button
            type="button"
            aria-label={t("Disk details")}
            onClick={(event) => {
              event.stopPropagation();
              onOpen();
            }}
            className="text-subtle transition hover:text-ink"
          >
            <Info className="size-3" />
          </button>
          <button
            type="button"
            aria-label={t("Remove from bay")}
            onClick={(event) => {
              event.stopPropagation();
              onRemove();
            }}
            className="text-subtle transition hover:text-[var(--danger)]"
          >
            <Trash2 className="size-3" />
          </button>
        </span>
      </div>
      <div className="mt-1 min-w-0">
        <p className="truncate text-[11px] font-semibold text-ink">{disk.name}</p>
        <p className="truncate text-[8.5px] text-subtle">{disk.model || disk.device}</p>
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-1.5">
        <Badge tone={tone}>{t(disk.type)}</Badge>
        <span className="text-[8.5px] text-subtle">{formatBytes(disk.size)}</span>
        <span className="ml-auto text-[8.5px] text-subtle">{disk.temp ? `${disk.temp}°C` : "—"}</span>
      </div>
    </div>
  );
}

function DiskCard({
  disk,
  selected,
  onSelect,
  onOpen,
  onDragStart,
  onDragEnd,
}: {
  disk: Disk;
  selected: boolean;
  onSelect: () => void;
  onOpen: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
}) {
  const { t } = useI18n();
  return (
    <div
      role="button"
      tabIndex={0}
      draggable
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={onSelect}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
      className={cn(
        "flex cursor-grab items-center gap-2 rounded-lg border bg-[var(--surface-2)]/45 p-2 transition active:cursor-grabbing",
        selected
          ? "border-[var(--accent-line)] bg-[var(--accent-soft)]"
          : "border-line hover:border-line-strong hover:bg-[var(--surface-2)]",
      )}
    >
      <span className="h-8 w-1 shrink-0 rounded-full" style={{ background: disk.color || "#8a8f98" }} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <p className="truncate text-[10.5px] font-medium text-ink">{disk.name}</p>
          <Badge tone={disk.type === "Parity" ? "violet" : disk.type === "Cache" ? "info" : "neutral"}>
            {t(disk.type)}
          </Badge>
        </div>
        <p className="truncate text-[8.5px] text-subtle">
          {formatBytes(disk.size)} · {disk.model || disk.device} · {disk.temp ? `${disk.temp}°C` : "—"}
        </p>
      </div>
      <button
        type="button"
        aria-label={t("Disk details")}
        onClick={(event) => {
          event.stopPropagation();
          onOpen();
        }}
        className="text-subtle transition hover:text-ink"
      >
        <Info className="size-3.5" />
      </button>
      <GripVertical className="size-3.5 shrink-0 text-subtle" />
    </div>
  );
}
