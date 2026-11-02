import { useEffect, useMemo, useState, type DragEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  GripVertical,
  HardDrive,
  Info,
  PcCase,
  Search,
  Server,
  Sparkles,
  Trash2,
  Undo2,
} from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { formatBytes, hexToRgba } from "../../lib/format";
import { Badge, Button, Card, EmptyState, IconButton, InputBase, cn } from "../ui/primitives";
import type { Disk, StorageLayout, StorageView } from "../../lib/types";

type Assignments = Record<string, number>;
type Orientation = StorageView["orientation"];
type DropTarget = { viewId: string; bay: number } | null;

const BAY_OPTIONS: Record<Orientation, number[]> = {
  vertical: [2, 4, 6, 8, 10, 12, 16, 20, 24, 32, 40, 48],
  horizontal: [8, 12, 16, 20, 24, 32, 40, 48],
  nas: [1, 2, 4, 6, 8],
};

const ORIENTATION_LABEL: Record<Orientation, string> = {
  vertical: "Tower",
  horizontal: "Server",
  nas: "NAS",
};

function maxBays(orientation: Orientation): number {
  return orientation === "nas" ? 8 : 48;
}

function createView(orientation: Orientation, index: number): StorageView {
  const id = typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `view-${Date.now()}-${index}`;
  return {
    id,
    name: `${ORIENTATION_LABEL[orientation]} ${index}`,
    orientation,
    column: 0,
    bays: orientation === "nas" ? 4 : 12,
    assignments: {},
  };
}

function diskModelLabel(disk: Disk): string {
  const model = disk.model || disk.device;
  if (!disk.brand) return model;
  const prefix = `${disk.brand.toLowerCase()} `;
  if (model.toLowerCase().startsWith(prefix)) return model.slice(disk.brand.length).trim();
  return model;
}

function diskBrandModel(disk: Disk): string {
  const model = diskModelLabel(disk);
  return disk.brand ? `${disk.brand} · ${model}` : model;
}

function diskPlacement(disk: Disk): string {
  if (disk.type === "Parity") return disk.device ? `/dev/${disk.device}` : "—";
  const mount = disk.type === "Cache" ? `/mnt/${disk.pool || disk.name}` : `/mnt/${disk.name}`;
  return disk.device ? `${disk.device} → ${mount}` : mount;
}

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
  const [views, setViews] = useState<StorageView[]>(() => [createView("vertical", 1)]);
  const [columns, setColumns] = useState(2);
  const [initialized, setInitialized] = useState(false);
  const [draggingView, setDraggingView] = useState<string | null>(null);
  const [viewDrop, setViewDrop] = useState<{ column: number; beforeId?: string } | null>(null);
  const [dragging, setDragging] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [overTarget, setOverTarget] = useState<DropTarget>(null);
  const [query, setQuery] = useState("");

  const layoutQuery = useQuery({
    queryKey: ["storage-layout"],
    queryFn: api.storageLayout,
  });

  useEffect(() => {
    if (!layoutQuery.data || initialized) return;
    const names = new Set(physical.map((disk) => disk.name));
    const nextColumns = layoutQuery.data.columns >= 1 && layoutQuery.data.columns <= 3
      ? layoutQuery.data.columns
      : 2;
    const next = layoutQuery.data.views.map((view, index) => {
      const assignments: Assignments = {};
      for (const [name, bay] of Object.entries(view.assignments ?? {})) {
        if (names.has(name) && bay >= 0 && bay < view.bays) assignments[name] = bay;
      }
      const column = view.column >= 0 && view.column < nextColumns ? view.column : index % nextColumns;
      return { ...view, column, assignments };
    });
    setColumns(nextColumns);
    setViews(next.length > 0 ? next : [createView("vertical", 1)]);
    setInitialized(true);
  }, [initialized, layoutQuery.data, physical]);

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
  const locationByDisk = useMemo(() => {
    const map = new Map<string, { viewId: string; bay: number }>();
    for (const view of views) {
      for (const [name, bay] of Object.entries(view.assignments)) {
        if (byName.has(name)) map.set(name, { viewId: view.id, bay });
      }
    }
    return map;
  }, [byName, views]);

  const diskByViewBay = useMemo(() => {
    const map = new Map<string, Map<number, Disk>>();
    for (const view of views) {
      const bays = new Map<number, Disk>();
      for (const [name, bay] of Object.entries(view.assignments)) {
        const disk = byName.get(name);
        if (disk) bays.set(bay, disk);
      }
      map.set(view.id, bays);
    }
    return map;
  }, [byName, views]);

  const unplacedAll = useMemo(
    () => physical.filter((disk) => !locationByDisk.has(disk.name)),
    [locationByDisk, physical],
  );
  const unplaced = useMemo(
    () =>
      unplacedAll.filter((disk) =>
        !query.trim() ||
        `${disk.name} ${disk.model} ${disk.device} ${disk.type}`.toLowerCase().includes(query.toLowerCase()),
      ),
    [query, unplacedAll],
  );

  const totalBays = views.reduce((sum, view) => sum + view.bays, 0);
  const placed = physical.length - unplacedAll.length;
  const empty = Math.max(0, totalBays - placed);

  const persist = (next: StorageView[], nextColumns = columns) => {
    setViews(next);
    setColumns(nextColumns);
    saveLayout.mutate({ columns: nextColumns, views: next, updated: "" });
  };

  const place = (name: string, viewId: string, bay: number) => {
    if (!byName.has(name)) return;
    const source = locationByDisk.get(name);
    if (source?.viewId === viewId && source.bay === bay) return;
    const next = views.map((view) => ({ ...view, assignments: { ...view.assignments } }));
    for (const view of next) delete view.assignments[name];
    const target = next.find((view) => view.id === viewId);
    if (!target) return;
    const occupantName = Object.keys(target.assignments).find((candidate) => target.assignments[candidate] === bay);
    if (occupantName) {
      delete target.assignments[occupantName];
      if (source) {
        const sourceView = next.find((view) => view.id === source.viewId);
        if (sourceView) sourceView.assignments[occupantName] = source.bay;
      }
    }
    target.assignments[name] = bay;
    setSelected(null);
    setDragging(null);
    setOverTarget(null);
    persist(next);
  };

  const unassign = (name: string) => {
    const next = views.map((view) => ({ ...view, assignments: { ...view.assignments } }));
    for (const view of next) delete view.assignments[name];
    setSelected(null);
    persist(next);
  };

  const changeBays = (viewId: string, bays: number) => {
    persist(views.map((view) => {
      if (view.id !== viewId) return view;
      const assignments = Object.fromEntries(
        Object.entries(view.assignments).filter(([, bay]) => bay < bays),
      );
      return { ...view, bays, assignments };
    }));
  };

  const toggleOrientation = (viewId: string) => {
    persist(views.map((view) => {
      if (view.id !== viewId) return view;
      const orientation: Orientation =
        view.orientation === "vertical" ? "horizontal" : view.orientation === "horizontal" ? "nas" : "vertical";
      const bays = Math.min(view.bays, maxBays(orientation));
      const assignments = Object.fromEntries(
        Object.entries(view.assignments).filter(([, bay]) => bay < bays),
      );
      return { ...view, orientation, bays, assignments };
    }));
  };

  const rename = (viewId: string, name: string) => {
    persist(views.map((view) => (view.id === viewId ? { ...view, name } : view)));
  };

  const addView = (orientation: Orientation) => {
    const count = views.filter((view) => view.orientation === orientation).length + 1;
    const counts = Array.from({ length: columns }, (_, column) =>
      views.filter((view) => view.column === column).length,
    );
    const column = counts.indexOf(Math.min(...counts));
    persist([...views, { ...createView(orientation, count), column }]);
  };

  const removeView = (viewId: string) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view) return;
    if (!window.confirm(t("Remove {name}? Its disks become unplaced.", { name: view.name }))) return;
    persist(views.filter((candidate) => candidate.id !== viewId));
  };

  const changeColumns = (nextColumns: number) => {
    const clamped = Math.max(1, Math.min(3, nextColumns));
    persist(
      views.map((view) => ({ ...view, column: Math.min(view.column, clamped - 1) })),
      clamped,
    );
  };

  const moveColumn = (viewId: string, direction: -1 | 1) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view) return;
    const column = view.column + direction;
    if (column < 0 || column >= columns) return;
    persist([...views.filter((candidate) => candidate.id !== viewId), { ...view, column }]);
  };

  const moveViewTo = (viewId: string, column: number, beforeId?: string) => {
    const moving = views.find((view) => view.id === viewId);
    if (!moving) return;
    const rest = views.filter((view) => view.id !== viewId);
    const updated = { ...moving, column };
    let index = rest.length;
    if (beforeId) {
      const before = rest.findIndex((view) => view.id === beforeId);
      if (before >= 0) index = before;
    }
    rest.splice(index, 0, updated);
    setDraggingView(null);
    setViewDrop(null);
    persist(rest);
  };

  const autoPlace = () => {
    const priority: Record<string, number> = { Parity: 0, Data: 1, Cache: 2 };
    const slots: { viewId: string; bay: number }[] = [];
    for (const view of views) {
      for (let bay = 0; bay < view.bays; bay += 1) slots.push({ viewId: view.id, bay });
    }
    const sorted = [...physical].sort(
      (a, b) =>
        (priority[a.type] ?? 9) - (priority[b.type] ?? 9) ||
        a.position - b.position ||
        a.name.localeCompare(b.name),
    );
    const next = views.map((view) => ({ ...view, assignments: {} as Assignments }));
    sorted.slice(0, slots.length).forEach((disk, index) => {
      const slot = slots[index];
      const target = next.find((view) => view.id === slot.viewId);
      if (target) target.assignments[disk.name] = slot.bay;
    });
    setSelected(null);
    persist(next);
  };

  const reset = () => {
    setSelected(null);
    persist(views.map((view) => ({ ...view, assignments: {} })));
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
            {t("Columns")}
            <select
              value={columns}
              onChange={(event) => changeColumns(Number(event.target.value))}
              className="h-8 rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              {[1, 2, 3].map((value) => (
                <option key={value} value={value}>{value}</option>
              ))}
            </select>
          </label>
          <Button size="sm" variant="secondary" icon={<PcCase className="size-3" />} onClick={() => addView("vertical")}>
            {t("Add tower")}
          </Button>
          <Button size="sm" variant="secondary" icon={<Server className="size-3" />} onClick={() => addView("horizontal")}>
            {t("Add server")}
          </Button>
          <Button size="sm" variant="secondary" icon={<Box className="size-3" />} onClick={() => addView("nas")}>
            {t("Add NAS")}
          </Button>
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

      {totalBays < physical.length ? (
        <p className="text-[10px] text-[var(--warning)]">
          {t("This chassis has fewer bays than disks. Increase the bay count to place every disk.")}
        </p>
      ) : null}

      <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_310px]">
        <div
          className={cn(
            "grid items-start gap-4",
            columns === 1
              ? "grid-cols-1"
              : columns === 2
                ? "grid-cols-1 lg:grid-cols-2"
                : "grid-cols-1 lg:grid-cols-2 min-[2000px]:grid-cols-3",
          )}
        >
          {Array.from({ length: columns }, (_, column) => {
            const columnViews = views.filter((view) => view.column === column);
            const columnActive = viewDrop?.column === column && !viewDrop.beforeId;
            return (
              <div
                key={column}
                onDragOver={(event) => {
                  if (!event.dataTransfer.types.includes("text/plain")) return;
                  event.preventDefault();
                  setViewDrop({ column });
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  const viewId = event.dataTransfer.getData("text/plain") || draggingView;
                  if (viewId) moveViewTo(viewId, column);
                }}
                onDragLeave={() => setViewDrop(null)}
                className={cn(
                  "flex min-h-[180px] flex-col gap-4 rounded-xl border border-dashed p-1.5 transition",
                  columnActive
                    ? "border-[var(--accent-line)] bg-[var(--accent-soft)]/25"
                    : "border-line/40",
                )}
              >
                {columnViews.length === 0 ? (
                  <p className="grid min-h-[164px] place-items-center px-3 text-center text-[10px] text-subtle">
                    {t("Drop a view here")}
                  </p>
                ) : null}
                {columnViews.map((view) => (
                  <div
                    key={view.id}
                    onDragOver={(event) => {
                      if (!event.dataTransfer.types.includes("text/plain")) return;
                      event.preventDefault();
                      event.stopPropagation();
                      setViewDrop({ column, beforeId: view.id });
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      const viewId = event.dataTransfer.getData("text/plain") || draggingView;
                      if (viewId) moveViewTo(viewId, column, view.id);
                    }}
                    className="relative"
                  >
                    {viewDrop?.beforeId === view.id ? (
                      <div className="absolute -top-2 left-2 right-2 h-0.5 rounded-full bg-[var(--accent)]" />
                    ) : null}
                    <ViewCard
                      view={view}
                      disks={diskByViewBay.get(view.id) ?? new Map()}
                      dragging={dragging}
                      selected={selected}
                      overTarget={overTarget}
                      viewDragging={draggingView === view.id}
                      canRemove={views.length > 1}
                      canMoveLeft={view.column > 0}
                      canMoveRight={view.column < columns - 1}
                      onRename={(name) => rename(view.id, name)}
                      onChangeBays={(bays) => changeBays(view.id, bays)}
                      onToggleOrientation={() => toggleOrientation(view.id)}
                      onMoveLeft={() => moveColumn(view.id, -1)}
                      onMoveRight={() => moveColumn(view.id, 1)}
                      onRemove={() => removeView(view.id)}
                      onViewDragStart={() => setDraggingView(view.id)}
                      onViewDragEnd={() => {
                        setDraggingView(null);
                        setViewDrop(null);
                      }}
                      onPlace={(name, bay) => place(name, view.id, bay)}
                      onSelect={(name) => setSelected(selected === name ? null : name)}
                      onRemoveDisk={(name) => unassign(name)}
                      onOpenDisk={onOpenDisk}
                      onDragStart={(name) => setDragging(name)}
                      onDragEnd={() => setDragging(null)}
                      onDragOver={(bay) => setOverTarget({ viewId: view.id, bay })}
                      onDragLeave={() => setOverTarget(null)}
                    />
                  </div>
                ))}
              </div>
            );
          })}
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

      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/35 px-3 py-2 text-[9.5px] text-subtle">
        <span>{t("{placed} placed · {empty} empty", { placed, empty })}</span>
        <span className="ml-auto">{t("{count} views", { count: views.length })}</span>
      </div>
    </div>
  );
}

function ViewCard({
  view,
  disks,
  dragging,
  selected,
  overTarget,
  viewDragging,
  canRemove,
  canMoveLeft,
  canMoveRight,
  onRename,
  onChangeBays,
  onToggleOrientation,
  onMoveLeft,
  onMoveRight,
  onRemove,
  onViewDragStart,
  onViewDragEnd,
  onPlace,
  onSelect,
  onRemoveDisk,
  onOpenDisk,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDragLeave,
}: {
  view: StorageView;
  disks: Map<number, Disk>;
  dragging: string | null;
  selected: string | null;
  overTarget: DropTarget;
  viewDragging: boolean;
  canRemove: boolean;
  canMoveLeft: boolean;
  canMoveRight: boolean;
  onRename: (name: string) => void;
  onChangeBays: (bays: number) => void;
  onToggleOrientation: () => void;
  onMoveLeft: () => void;
  onMoveRight: () => void;
  onRemove: () => void;
  onViewDragStart: () => void;
  onViewDragEnd: () => void;
  onPlace: (name: string, bay: number) => void;
  onSelect: (name: string) => void;
  onRemoveDisk: (name: string) => void;
  onOpenDisk: (disk: Disk) => void;
  onDragStart: (name: string) => void;
  onDragEnd: () => void;
  onDragOver: (bay: number) => void;
  onDragLeave: () => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState(view.name);
  useEffect(() => setName(view.name), [view.name]);
  const vertical = view.orientation === "vertical";
  const nas = view.orientation === "nas";
  const options = [...new Set([...BAY_OPTIONS[view.orientation], view.bays])]
    .filter((value) => value >= 1 && value <= maxBays(view.orientation))
    .sort((a, b) => a - b);
  const assigned = Object.keys(view.assignments).length;
  const orientationIcon = vertical ? <Server className="size-3.5" /> : nas ? <PcCase className="size-3.5" /> : <Box className="size-3.5" />;

  return (
    <div
      className={cn(
        "relative min-w-0 overflow-hidden rounded-[26px] border border-line bg-[linear-gradient(145deg,var(--surface-2),var(--surface))] p-3 shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_30px_80px_-50px_rgba(0,0,0,0.9)]",
        vertical ? "mx-auto w-full max-w-[560px]" : nas ? "mx-auto w-full max-w-[820px]" : "w-full",
        viewDragging && "opacity-50",
      )}
    >
      <div className="pointer-events-none absolute inset-y-5 left-2 w-1.5 rounded-full bg-[var(--surface-3)]" />
      <div className="pointer-events-none absolute inset-y-5 right-2 w-1.5 rounded-full bg-[var(--surface-3)]" />
      <div className="relative rounded-2xl border border-line bg-[var(--surface)]/60 p-4">
        <div className="flex flex-wrap items-center gap-2">
          <span
            draggable
            onDragStart={(event) => {
              event.dataTransfer.effectAllowed = "move";
              event.dataTransfer.setData("text/plain", view.id);
              onViewDragStart();
            }}
            onDragEnd={onViewDragEnd}
            title={t("Drag view")}
            className="cursor-grab text-subtle transition hover:text-ink active:cursor-grabbing"
          >
            <GripVertical className="size-3.5" />
          </span>
          <HardDrive className="size-3.5 text-[var(--accent)]" />
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            onBlur={() => {
              const next = name.trim();
              if (!next) setName(view.name);
              else if (next !== view.name) onRename(next);
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") event.currentTarget.blur();
            }}
            aria-label={t("View name")}
            className="min-w-0 max-w-[190px] flex-1 rounded border-0 bg-transparent text-[12px] font-semibold text-ink outline-none focus:bg-[var(--surface-2)] focus:px-1.5 focus:py-1"
          />
          <Badge tone={vertical ? "violet" : nas ? "warning" : "info"}>{t(ORIENTATION_LABEL[view.orientation])}</Badge>
          <IconButton label={t("Switch orientation")} size="sm" onClick={onToggleOrientation}>
            {orientationIcon}
          </IconButton>
          <label className="ml-auto flex items-center gap-1.5 text-[9.5px] text-subtle">
            {t("Bays")}
            <select
              value={view.bays}
              onChange={(event) => onChangeBays(Number(event.target.value))}
              className="h-7 rounded-[6px] border border-line bg-[var(--surface-2)] px-1.5 text-[10px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              {options.map((value) => (
                <option key={value} value={value}>{value}</option>
              ))}
            </select>
          </label>
          <span className="text-[9.5px] text-subtle">{assigned}/{view.bays}</span>
          <IconButton label={t("Move view left")} size="sm" disabled={!canMoveLeft} onClick={onMoveLeft}>
            <ChevronLeft className="size-3.5" />
          </IconButton>
          <IconButton label={t("Move view right")} size="sm" disabled={!canMoveRight} onClick={onMoveRight}>
            <ChevronRight className="size-3.5" />
          </IconButton>
          {canRemove ? (
            <IconButton label={t("Remove view")} size="sm" onClick={onRemove}>
              <Trash2 className="size-3.5 text-subtle hover:text-[var(--danger)]" />
            </IconButton>
          ) : null}
        </div>

        <div
          className={cn("mt-3", vertical ? "flex flex-col gap-1.5" : "grid gap-2")}
          style={vertical ? undefined : { gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))" }}
        >
          {Array.from({ length: view.bays }, (_, bay) => {
            const disk = disks.get(bay);
            const activeDrop = overTarget?.viewId === view.id && overTarget.bay === bay;
            return disk ? (
              <AssignedBay
                key={bay}
                bay={bay}
                disk={disk}
                activeDrop={activeDrop}
                dragActive={dragging === disk.name}
                selected={selected === disk.name}
                onSelect={() => onSelect(disk.name)}
                onRemove={() => onRemoveDisk(disk.name)}
                onOpen={() => onOpenDisk(disk)}
                onDragStart={() => onDragStart(disk.name)}
                onDragEnd={onDragEnd}
                onDragOver={(event) => {
                  event.preventDefault();
                  onDragOver(bay);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  const source = dragging ?? selected;
                  if (source) onPlace(source, bay);
                }}
                onDragLeave={onDragLeave}
              />
            ) : (
              <button
                key={bay}
                type="button"
                onClick={() => selected && onPlace(selected, bay)}
                onDragOver={(event) => {
                  event.preventDefault();
                  onDragOver(bay);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  const source = dragging ?? selected;
                  if (source) onPlace(source, bay);
                }}
                onDragLeave={onDragLeave}
                className={cn(
                  "flex flex-col justify-between rounded-xl border border-dashed p-2.5 text-left transition",
                  vertical ? "min-h-[54px]" : "min-h-[94px]",
                  activeDrop
                    ? "border-[var(--accent-line)] bg-[var(--accent-soft)]"
                    : "border-line/80 bg-[var(--surface-3)]/20 hover:border-line-strong hover:bg-[var(--surface-3)]/40",
                )}
              >
                <span className="flex items-center justify-between text-[8.5px] uppercase tracking-[0.1em] text-subtle">
                  <span>{t("Bay {index}", { index: bay + 1 })}</span>
                  <span className="size-1.5 rounded-full bg-[var(--subtle)]/30" />
                </span>
                <span className={cn("text-[9.5px] text-subtle", vertical ? "text-left" : "text-center")}>
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
          <span className="ml-auto font-mono">Aurora · {view.bays}× HDD</span>
        </div>
      </div>
    </div>
  );
}

function AssignedBay({
  bay,
  disk,
  activeDrop,
  dragActive,
  selected,
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
  activeDrop: boolean;
  dragActive: boolean;
  selected: boolean;
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
  const color = disk.color || "#8a8f98";
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
        borderColor: hexToRgba(color, selected ? 0.75 : 0.4),
        background: `linear-gradient(145deg, ${hexToRgba(color, 0.16)}, color-mix(in srgb, var(--surface-3) 55%, transparent))`,
      }}
      className={cn(
        "flex min-h-[108px] cursor-grab flex-col justify-between rounded-xl border p-2.5 text-left transition active:cursor-grabbing",
        selected && "ring-1 ring-[var(--accent-line)]",
        activeDrop && "ring-2 ring-[var(--accent-line)]",
        dragActive && "opacity-50",
      )}
      title={`${disk.name} · ${t("Drag to move")}`}
    >
      <div className="flex items-center justify-between gap-1">
        <span className="font-mono text-[8.5px] uppercase tracking-[0.1em] text-subtle">{t("Bay {index}", { index: bay + 1 })}</span>
        <span className="flex items-center gap-1">
          <span className="size-1.5 rounded-full" style={{ background: color }} />
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
        <div className="flex items-center gap-1.5">
          <p className="truncate text-[11px] font-semibold text-ink">{disk.name}</p>
          <Badge tone={tone}>{t(disk.type)}</Badge>
        </div>
        <p className="truncate text-[8.5px] text-muted" title={diskBrandModel(disk)}>{diskBrandModel(disk)}</p>
        <p className="truncate font-mono text-[8px] text-subtle" title={disk.serial || undefined}>
          S/N {disk.serial || "—"}
        </p>
      </div>
      <p
        className="mt-1 truncate text-[8.5px] text-subtle"
        title={`${formatBytes(disk.size)} · ${diskPlacement(disk)} · ${disk.temp ? `${disk.temp}°C` : "—"}`}
      >
        {formatBytes(disk.size)} · {diskPlacement(disk)} · {disk.temp ? `${disk.temp}°C` : "—"}
      </p>
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
      <span className="h-14 w-1 shrink-0 rounded-full" style={{ background: disk.color || "#8a8f98" }} />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <p className="truncate text-[10.5px] font-medium text-ink">{disk.name}</p>
          <Badge tone={disk.type === "Parity" ? "violet" : disk.type === "Cache" ? "info" : "neutral"}>
            {t(disk.type)}
          </Badge>
        </div>
        <p className="truncate text-[9px] text-muted" title={diskBrandModel(disk)}>
          {diskBrandModel(disk)}
        </p>
        <p className="truncate font-mono text-[8.5px] text-subtle" title={disk.serial || undefined}>
          S/N {disk.serial || "—"}
        </p>
        <p
          className="truncate text-[8.5px] text-subtle"
          title={`${formatBytes(disk.size)} · ${diskPlacement(disk)} · ${disk.temp ? `${disk.temp}°C` : "—"}`}
        >
          {formatBytes(disk.size)} · {diskPlacement(disk)} · {disk.temp ? `${disk.temp}°C` : "—"}
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
