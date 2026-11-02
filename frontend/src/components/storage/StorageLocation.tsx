import { useEffect, useMemo, useRef, useState, type DragEvent, type PointerEvent as ReactPointerEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  CheckCircle2,
  GripVertical,
  HardDrive,
  Info,
  PcCase,
  Pencil,
  Plus,
  Search,
  Server,
  Sparkles,
  Trash2,
  TriangleAlert,
  Undo2,
} from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { formatBytes, hexToRgba } from "../../lib/format";
import { Badge, Button, Card, EmptyState, IconButton, InputBase, cn } from "../ui/primitives";
import { Modal } from "../ui/overlays";
import type { Disk, StorageLayout, StorageSection, StorageView } from "../../lib/types";
import {
  BOARD_COLUMNS,
  GRID_GAP,
  ROW_HEIGHT,
  defaultSection,
  defaultView,
  diskBrandModel,
  diskPlacement,
  findFreeSpot,
  mediaLabel,
  panelDimensions,
  rectsOverlap,
  type PanelKind,
  type PanelSize,
  type PresetSize,
} from "./location-model";

const DISK_MIME = "application/x-aurora-disk";
const SECTION_MIME = "application/x-aurora-section";

type Assignments = Record<string, number>;
type DropTarget = { viewId: string; sectionId: string; slot: number } | null;
type SectionDrag = { viewId: string; sectionId: string } | null;
type SectionDrop = { viewId: string; beforeId?: string } | null;
type SectionEditorState = { viewId: string; section: StorageSection; isNew: boolean } | null;
type ResizeEdge = "n" | "s" | "e" | "w" | "ne" | "nw" | "se" | "sw";

function newId(prefix: string): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return `${prefix}-${crypto.randomUUID()}`;
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function mediaTone(media: string): "neutral" | "info" | "violet" {
  if (media === "nvme") return "violet";
  if (media === "ssd") return "info";
  return "neutral";
}

function sectionMediaKey(media: string): string {
  if (media === "hdd") return "HDD";
  if (media === "ssd") return "SSD";
  if (media === "nvme") return "NVMe";
  return "Any";
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
  const [views, setViews] = useState<StorageView[]>(() => [defaultView("tower", 1)]);
  const [initialized, setInitialized] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [draggingDisk, setDraggingDisk] = useState<string | null>(null);
  const [draggingView, setDraggingView] = useState<string | null>(null);
  const [draggingSection, setDraggingSection] = useState<SectionDrag>(null);
  const [sectionDrop, setSectionDrop] = useState<SectionDrop>(null);
  const [overTarget, setOverTarget] = useState<DropTarget>(null);
  const [query, setQuery] = useState("");
  const [sectionEditor, setSectionEditor] = useState<SectionEditorState>(null);
  const [boardWidth, setBoardWidth] = useState(0);
  const boardRef = useRef<HTMLDivElement>(null);

  const layoutQuery = useQuery({
    queryKey: ["storage-layout"],
    queryFn: api.storageLayout,
  });

  useEffect(() => {
    if (!layoutQuery.data || initialized) return;
    const names = new Set(physical.map((disk) => disk.name));
    const next = layoutQuery.data.views.map((view) => {
      const kind: PanelKind = view.kind === "server" || view.kind === "nas" ? view.kind : "tower";
      const size: PanelSize = view.size === "s" || view.size === "m" || view.size === "l" ? view.size : "custom";
      const dims = panelDimensions(kind, size === "custom" ? "m" : size);
      const width = view.w >= 1 && view.w <= BOARD_COLUMNS ? view.w : dims.w;
      const height = view.h >= 1 && view.h <= 60 ? view.h : dims.h;
      const sections = (view.sections ?? []).map((section) => {
        const assignments: Assignments = {};
        for (const [name, slot] of Object.entries(section.assignments ?? {})) {
          if (names.has(name) && slot >= 0 && slot < section.rows * section.cols) assignments[name] = slot;
        }
        return { ...section, assignments };
      });
      return {
        ...view,
        kind,
        size,
        w: width,
        h: height,
        x: Math.max(0, Math.min(BOARD_COLUMNS - width, view.x)),
        y: Math.max(0, view.y),
        sections: sections.length > 0 ? sections : [defaultSection(kind)],
      };
    });
    setViews(next.length > 0 ? next : [defaultView("tower", 1)]);
    setInitialized(true);
  }, [initialized, layoutQuery.data, physical]);

  useEffect(() => {
    const element = boardRef.current;
    if (!element) return;
    const observer = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (rect) setBoardWidth(rect.width);
    });
    observer.observe(element);
    setBoardWidth(element.getBoundingClientRect().width);
    return () => observer.disconnect();
  }, []);

  const saveLayout = useMutation({
    mutationFn: (layout: StorageLayout) => api.saveStorageLayout(layout),
    onSuccess: (layout) => queryClient.setQueryData(["storage-layout"], layout),
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Could not save layout"), description: error.message });
    },
  });

  const persist = (next: StorageView[]) => {
    setViews(next);
    saveLayout.mutate({ version: 2, views: next, updated: "" });
  };

  const retry = () => {
    setInitialized(false);
    void queryClient.invalidateQueries({ queryKey: ["storage-layout"] });
  };

  const byName = useMemo(() => new Map(physical.map((disk) => [disk.name, disk])), [physical]);
  const locationByDisk = useMemo(() => {
    const map = new Map<string, { viewId: string; sectionId: string; slot: number }>();
    for (const view of views) {
      for (const section of view.sections) {
        for (const [name, slot] of Object.entries(section.assignments)) {
          if (byName.has(name)) map.set(name, { viewId: view.id, sectionId: section.id, slot });
        }
      }
    }
    return map;
  }, [byName, views]);

  const disksBySection = useMemo(() => {
    const map = new Map<string, Map<number, Disk>>();
    for (const view of views) {
      for (const section of view.sections) {
        const slots = new Map<number, Disk>();
        for (const [name, slot] of Object.entries(section.assignments)) {
          const disk = byName.get(name);
          if (disk) slots.set(slot, disk);
        }
        map.set(`${view.id}:${section.id}`, slots);
      }
    }
    return map;
  }, [byName, views]);

  const unplacedAll = useMemo(
    () => physical.filter((disk) => !locationByDisk.has(disk.name)),
    [locationByDisk, physical],
  );
  const unplaced = useMemo(
    () => unplacedAll.filter((disk) =>
      !query.trim() ||
      `${disk.name} ${disk.model} ${disk.device} ${disk.type} ${disk.media}`.toLowerCase().includes(query.toLowerCase()),
    ),
    [query, unplacedAll],
  );

  const placed = physical.length - unplacedAll.length;
  const totalSlots = views.reduce(
    (sum, view) => sum + view.sections.reduce((viewSum, section) => viewSum + section.rows * section.cols, 0),
    0,
  );
  const empty = Math.max(0, totalSlots - placed);
  const stacked = boardWidth > 0 && boardWidth < 760;
  const unitX = boardWidth > 0 ? (boardWidth - GRID_GAP * (BOARD_COLUMNS - 1)) / BOARD_COLUMNS : 0;
  const boardHeight = views.reduce((height, view) => Math.max(height, (view.y + view.h) * (ROW_HEIGHT + GRID_GAP)), 0);

  const place = (name: string, viewId: string, sectionId: string, slot: number) => {
    if (!byName.has(name)) return;
    const source = locationByDisk.get(name);
    if (source?.viewId === viewId && source.sectionId === sectionId && source.slot === slot) return;
    const next = views.map((view) => ({
      ...view,
      sections: view.sections.map((section) => ({ ...section, assignments: { ...section.assignments } })),
    }));
    for (const view of next) {
      for (const section of view.sections) delete section.assignments[name];
    }
    const targetView = next.find((view) => view.id === viewId);
    const target = targetView?.sections.find((section) => section.id === sectionId);
    if (!target) return;
    const occupantName = Object.keys(target.assignments).find((candidate) => target.assignments[candidate] === slot);
    if (occupantName) {
      delete target.assignments[occupantName];
      if (source) {
        const sourceView = next.find((view) => view.id === source.viewId);
        const sourceSection = sourceView?.sections.find((section) => section.id === source.sectionId);
        if (sourceSection) sourceSection.assignments[occupantName] = source.slot;
      }
    }
    target.assignments[name] = slot;
    setSelected(null);
    setDraggingDisk(null);
    setOverTarget(null);
    persist(next);
  };

  const unassign = (name: string) => {
    const next = views.map((view) => ({
      ...view,
      sections: view.sections.map((section) => {
        const assignments = { ...section.assignments };
        delete assignments[name];
        return { ...section, assignments };
      }),
    }));
    setSelected(null);
    persist(next);
  };

  const changeSize = (viewId: string, size: PresetSize) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view || view.size === size) return;
    const dims = panelDimensions(view.kind, size);
    const spot = findFreeSpot(views, dims.w, dims.h, view.x, view.y, view.id);
    if (!spot) {
      toast({ kind: "warning", title: t("No free space for this size"), description: t("Move a panel first, then try again.") });
      return;
    }
    persist(views.map((candidate) => (
      candidate.id === viewId ? { ...candidate, size, ...spot, w: dims.w, h: dims.h } : candidate
    )));
  };

  const addView = (kind: PanelKind) => {
    const index = views.filter((view) => view.kind === kind).length + 1;
    const dims = panelDimensions(kind, "m");
    const spot = findFreeSpot(views, dims.w, dims.h, 0, 0);
    if (!spot) {
      toast({ kind: "warning", title: t("No free space on the board"), description: t("Move or remove a panel first.") });
      return;
    }
    const view = { ...defaultView(kind, index, "m"), ...spot };
    persist([...views, view]);
  };

  const removeView = (viewId: string) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view) return;
    if (!window.confirm(t("Remove {name}? Its disks become unplaced.", { name: view.name }))) return;
    persist(views.filter((candidate) => candidate.id !== viewId));
  };

  const renameView = (viewId: string, name: string) => {
    persist(views.map((view) => (view.id === viewId ? { ...view, name } : view)));
  };

  const autoArrange = () => {
    let placedViews: StorageView[] = [];
    const next = views.map((view) => {
      const spot = findFreeSpot(placedViews, view.w, view.h, 0, 0);
      const positioned = spot ? { ...view, ...spot } : view;
      placedViews = [...placedViews, positioned];
      return positioned;
    });
    persist(next);
  };

  const autoPlace = () => {
    const priority: Record<string, number> = { Parity: 0, Data: 1, Cache: 2 };
    const remaining = [...physical].sort(
      (a, b) => (priority[a.type] ?? 9) - (priority[b.type] ?? 9) || a.position - b.position || a.name.localeCompare(b.name),
    );
    const orderedViews = [...views].sort((a, b) => a.y - b.y || a.x - b.x);
    const next = views.map((view) => ({
      ...view,
      sections: view.sections.map((section) => ({ ...section, assignments: {} as Assignments })),
    }));
    for (const orderedView of orderedViews) {
      const targetView = next.find((view) => view.id === orderedView.id);
      if (!targetView) continue;
      for (const orderedSection of orderedView.sections) {
        const targetSection = targetView.sections.find((section) => section.id === orderedSection.id);
        if (!targetSection) continue;
        for (let slot = 0; slot < targetSection.rows * targetSection.cols; slot += 1) {
          let index = remaining.findIndex((disk) => targetSection.media === "any" || disk.media === targetSection.media);
          if (index < 0) index = 0;
          const disk = remaining.splice(index, 1)[0];
          if (!disk) break;
          targetSection.assignments[disk.name] = slot;
        }
      }
    }
    setSelected(null);
    persist(next);
  };

  const reset = () => {
    setSelected(null);
    persist(views.map((view) => ({
      ...view,
      sections: view.sections.map((section) => ({ ...section, assignments: {} })),
    })));
  };

  const reorderSections = (viewId: string, sectionId: string, beforeId?: string) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view) return;
    const moving = view.sections.find((section) => section.id === sectionId);
    if (!moving) return;
    const rest = view.sections.filter((section) => section.id !== sectionId);
    let index = rest.length;
    if (beforeId) {
      const before = rest.findIndex((section) => section.id === beforeId);
      if (before >= 0) index = before;
    }
    rest.splice(index, 0, moving);
    persist(views.map((candidate) => (candidate.id === viewId ? { ...candidate, sections: rest } : candidate)));
  };

  const openSectionEditor = (viewId: string, section: StorageSection | null) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view) return;
    setSectionEditor({
      viewId,
      isNew: !section,
      section: section
        ? { ...section, assignments: { ...section.assignments } }
        : { ...defaultSection(view.kind), id: newId("section"), name: t("Section") },
    });
  };

  const saveSection = (viewId: string, section: StorageSection) => {
    const limit = section.rows * section.cols;
    const assignments = Object.fromEntries(
      Object.entries(section.assignments).filter(([, slot]) => slot >= 0 && slot < limit),
    );
    const next = views.map((view) => {
      if (view.id !== viewId) return view;
      const exists = view.sections.some((candidate) => candidate.id === section.id);
      return {
        ...view,
        sections: exists
          ? view.sections.map((candidate) => (candidate.id === section.id ? { ...section, assignments } : candidate))
          : [...view.sections, { ...section, assignments }],
      };
    });
    setSectionEditor(null);
    persist(next);
  };

  const deleteSection = (viewId: string, section: StorageSection) => {
    const view = views.find((candidate) => candidate.id === viewId);
    if (!view) return;
    if (view.sections.length <= 1) {
      toast({ kind: "warning", title: t("A chassis needs at least one section") });
      return;
    }
    if (!window.confirm(t("Remove section {name}? Its disks become unplaced.", { name: section.name }))) return;
    persist(views.map((candidate) => (
      candidate.id === viewId ? { ...candidate, sections: candidate.sections.filter((item) => item.id !== section.id) } : candidate
    )));
  };

  const startViewDrag = (event: ReactPointerEvent<HTMLSpanElement>, view: StorageView) => {
    if (stacked || event.button !== 0 || boardWidth <= 0) return;
    event.preventDefault();
    const startX = event.clientX;
    const startY = event.clientY;
    const originLeft = view.x * (unitX + GRID_GAP);
    const originTop = view.y * (ROW_HEIGHT + GRID_GAP);
    let current = views;
    let moved = false;
    setDraggingView(view.id);
    document.body.style.userSelect = "none";
    const move = (moveEvent: PointerEvent) => {
      const x = Math.max(0, Math.min(BOARD_COLUMNS - view.w, Math.round((originLeft + moveEvent.clientX - startX) / (unitX + GRID_GAP))));
      const y = Math.max(0, Math.round((originTop + moveEvent.clientY - startY) / (ROW_HEIGHT + GRID_GAP)));
      const currentView = current.find((candidate) => candidate.id === view.id);
      if (!currentView || (currentView.x === x && currentView.y === y)) return;
      if (current.some((candidate) => candidate.id !== view.id && rectsOverlap({ x, y, w: view.w, h: view.h }, candidate))) return;
      current = current.map((candidate) => (candidate.id === view.id ? { ...candidate, x, y } : candidate));
      moved = true;
      setViews(current);
    };
    const stop = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
      document.body.style.userSelect = "";
      setDraggingView(null);
      if (moved) persist(current);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
  };

  const startResize = (event: ReactPointerEvent<HTMLSpanElement>, view: StorageView, edge: ResizeEdge) => {
    if (stacked || event.button !== 0 || boardWidth <= 0) return;
    event.preventDefault();
    event.stopPropagation();
    const startX = event.clientX;
    const startY = event.clientY;
    const initialLeft = view.x * (unitX + GRID_GAP);
    const initialTop = view.y * (ROW_HEIGHT + GRID_GAP);
    const initialWidth = view.w * (unitX + GRID_GAP) - GRID_GAP;
    const initialHeight = view.h * (ROW_HEIGHT + GRID_GAP) - GRID_GAP;
    const rightEdge = view.x + view.w;
    const bottomEdge = view.y + view.h;
    const minW = 4;
    const minH = 6;
    let current = views;
    let moved = false;
    document.body.style.userSelect = "none";

    const move = (moveEvent: PointerEvent) => {
      const dx = moveEvent.clientX - startX;
      const dy = moveEvent.clientY - startY;
      let x = view.x;
      let y = view.y;
      let w = view.w;
      let h = view.h;
      if (edge.includes("e")) w = Math.round((initialWidth + dx + GRID_GAP) / (unitX + GRID_GAP));
      if (edge.includes("w")) {
        x = Math.round((initialLeft + dx) / (unitX + GRID_GAP));
        w = rightEdge - x;
      }
      if (edge.includes("s")) h = Math.round((initialHeight + dy + GRID_GAP) / (ROW_HEIGHT + GRID_GAP));
      if (edge.includes("n")) {
        y = Math.round((initialTop + dy) / (ROW_HEIGHT + GRID_GAP));
        h = bottomEdge - y;
      }
      if (x < 0) {
        x = 0;
        w = rightEdge;
      }
      if (y < 0) {
        y = 0;
        h = bottomEdge;
      }
      if (w < minW) {
        if (edge.includes("w")) x = rightEdge - minW;
        w = minW;
      }
      if (h < minH) {
        if (edge.includes("n")) y = bottomEdge - minH;
        h = minH;
      }
      if (w > BOARD_COLUMNS) w = BOARD_COLUMNS;
      if (x + w > BOARD_COLUMNS) w = BOARD_COLUMNS - x;
      if (h > 60) h = 60;
      if (w < minW || h < minH) return;
      const currentView = current.find((candidate) => candidate.id === view.id);
      if (!currentView) return;
      if (currentView.x === x && currentView.y === y && currentView.w === w && currentView.h === h) return;
      if (current.some((candidate) => candidate.id !== view.id && rectsOverlap({ x, y, w, h }, candidate))) return;
      current = current.map((candidate) => (
        candidate.id === view.id ? { ...candidate, x, y, w, h, size: "custom" } : candidate
      ));
      moved = true;
      setViews(current);
    };

    const stop = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
      document.body.style.userSelect = "";
      if (moved) persist(current);
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
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

  const renderPanel = (view: StorageView) => (
    <ChassisPanel
      key={view.id}
      view={view}
      disksBySection={disksBySection}
      selected={selected}
      draggingDisk={draggingDisk}
      draggingView={draggingView}
      draggingSection={draggingSection}
      sectionDrop={sectionDrop}
      overTarget={overTarget}
      canRemove={views.length > 1}
      stacked={stacked}
      onDragView={(event) => startViewDrag(event, view)}
      onRename={(name) => renameView(view.id, name)}
      onChangeSize={(size) => changeSize(view.id, size)}
      onRemove={() => removeView(view.id)}
      onAddSection={() => openSectionEditor(view.id, null)}
      onEditSection={(section) => openSectionEditor(view.id, section)}
      onDeleteSection={(section) => deleteSection(view.id, section)}
      onSectionDragStart={(sectionId) => setDraggingSection({ viewId: view.id, sectionId })}
      onSectionDragEnd={() => setDraggingSection(null)}
      onSectionDragOver={(beforeId) => setSectionDrop({ viewId: view.id, beforeId })}
      onSectionDrop={(beforeId) => {
        if (!draggingSection || draggingSection.viewId !== view.id) return;
        reorderSections(view.id, draggingSection.sectionId, beforeId);
        setDraggingSection(null);
        setSectionDrop(null);
      }}
      onDragDiskStart={(name) => setDraggingDisk(name)}
      onDragDiskEnd={() => setDraggingDisk(null)}
      onDragOverSlot={(sectionId, slot) => setOverTarget({ viewId: view.id, sectionId, slot })}
      onDragLeaveSlot={() => setOverTarget(null)}
      onDropDisk={(sectionId, slot, name) => place(name, view.id, sectionId, slot)}
      onSelectDisk={(name) => setSelected(selected === name ? null : name)}
      onRemoveDisk={unassign}
      onOpenDisk={onOpenDisk}
      onPlaceSelected={(sectionId, slot) => selected && place(selected, view.id, sectionId, slot)}
    />
  );

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/50 px-3 py-2.5">
        <Info className="size-3.5 shrink-0 text-[var(--accent)]" />
        <p className="min-w-0 flex-1 text-[10.5px] leading-relaxed text-muted">
          {t("Drag a disk into a bay, or select a disk then click a bay.")}{" "}
          <span className="text-subtle">{t("Visual layout only — Unraid disk assignments are unchanged.")}</span>
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="secondary" icon={<PcCase className="size-3" />} onClick={() => addView("tower")}>
            {t("Add tower")}
          </Button>
          <Button size="sm" variant="secondary" icon={<Server className="size-3" />} onClick={() => addView("server")}>
            {t("Add server")}
          </Button>
          <Button size="sm" variant="secondary" icon={<Box className="size-3" />} onClick={() => addView("nas")}>
            {t("Add NAS")}
          </Button>
          <Button size="sm" variant="secondary" icon={<Sparkles className="size-3" />} onClick={autoArrange}>
            {t("Auto-arrange")}
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

      {totalSlots < physical.length ? (
        <p className="text-[10px] text-[var(--warning)]">
          {t("There are fewer slots than disks. Add a section or increase its grid.")}
        </p>
      ) : null}

      <div className="grid gap-4 2xl:grid-cols-[minmax(0,1fr)_310px]">
        <div
          ref={boardRef}
          className={cn("relative w-full", stacked ? "space-y-4" : "")}
          style={!stacked ? { height: boardHeight } : undefined}
        >
          {stacked ? (
            views.map((view) => (
              <div key={view.id} style={{ height: Math.max(420, view.h * (ROW_HEIGHT + GRID_GAP)) }}>
                {renderPanel(view)}
              </div>
            ))
          ) : (
            views.map((view) => (
              <div
                key={view.id}
                className="group absolute"
                style={{
                  left: view.x * (unitX + GRID_GAP),
                  top: view.y * (ROW_HEIGHT + GRID_GAP),
                  width: view.w * unitX + (view.w - 1) * GRID_GAP,
                  height: view.h * (ROW_HEIGHT + GRID_GAP) - GRID_GAP,
                  zIndex: draggingView === view.id ? 30 : 1,
                }}
              >
                {renderPanel(view)}
                <div className="pointer-events-none absolute inset-0 z-20 opacity-0 transition-opacity group-hover:opacity-100">
                  {([
                    ["n", "left-3 right-3 -top-1 h-2 cursor-row-resize"],
                    ["s", "left-3 right-3 -bottom-1 h-2 cursor-row-resize"],
                    ["w", "top-3 bottom-3 -left-1 w-2 cursor-col-resize"],
                    ["e", "top-3 bottom-3 -right-1 w-2 cursor-col-resize"],
                    ["nw", "-left-1 -top-1 size-3 cursor-nwse-resize"],
                    ["ne", "-right-1 -top-1 size-3 cursor-nesw-resize"],
                    ["sw", "-left-1 -bottom-1 size-3 cursor-nesw-resize"],
                    ["se", "-right-1 -bottom-1 size-3 cursor-nwse-resize"],
                  ] as [ResizeEdge, string][]).map(([edge, className]) => (
                    <span
                      key={edge}
                      onPointerDown={(event) => startResize(event, view, edge)}
                      className={cn("pointer-events-auto absolute rounded-sm hover:bg-[var(--accent)]/40", className)}
                    />
                  ))}
                </div>
              </div>
            ))
          )}
        </div>

        <Card className="p-3 2xl:sticky 2xl:top-24 2xl:self-start">
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
                  onDragStart={() => setDraggingDisk(disk.name)}
                  onDragEnd={() => setDraggingDisk(null)}
                />
              ))}
            </div>
          )}
        </Card>
      </div>

      <div className="flex flex-wrap items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/35 px-3 py-2 text-[9.5px] text-subtle">
        <span>{t("{placed} placed · {empty} empty", { placed, empty })}</span>
        <span>{t("{count} panels", { count: views.length })}</span>
        <span className="ml-auto">{t("Drag an edge or corner to resize")}</span>
        <span>{t("Snap grid · 12 columns")}</span>
      </div>

      <SectionEditorModal
        open={Boolean(sectionEditor)}
        section={sectionEditor?.section ?? null}
        isNew={Boolean(sectionEditor?.isNew)}
        onClose={() => setSectionEditor(null)}
        onSave={(section) => sectionEditor && saveSection(sectionEditor.viewId, section)}
      />
    </div>
  );
}

function ChassisPanel({
  view,
  disksBySection,
  selected,
  draggingDisk,
  draggingView,
  draggingSection,
  sectionDrop,
  overTarget,
  canRemove,
  stacked,
  onDragView,
  onRename,
  onChangeSize,
  onRemove,
  onAddSection,
  onEditSection,
  onDeleteSection,
  onSectionDragStart,
  onSectionDragEnd,
  onSectionDragOver,
  onSectionDrop,
  onDragDiskStart,
  onDragDiskEnd,
  onDragOverSlot,
  onDragLeaveSlot,
  onDropDisk,
  onSelectDisk,
  onRemoveDisk,
  onOpenDisk,
  onPlaceSelected,
}: {
  view: StorageView;
  disksBySection: Map<string, Map<number, Disk>>;
  selected: string | null;
  draggingDisk: string | null;
  draggingView: string | null;
  draggingSection: SectionDrag;
  sectionDrop: SectionDrop;
  overTarget: DropTarget;
  canRemove: boolean;
  stacked: boolean;
  onDragView: (event: ReactPointerEvent<HTMLSpanElement>) => void;
  onRename: (name: string) => void;
  onChangeSize: (size: PresetSize) => void;
  onRemove: () => void;
  onAddSection: () => void;
  onEditSection: (section: StorageSection) => void;
  onDeleteSection: (section: StorageSection) => void;
  onSectionDragStart: (sectionId: string) => void;
  onSectionDragEnd: () => void;
  onSectionDragOver: (beforeId?: string) => void;
  onSectionDrop: (beforeId?: string) => void;
  onDragDiskStart: (name: string) => void;
  onDragDiskEnd: () => void;
  onDragOverSlot: (sectionId: string, slot: number) => void;
  onDragLeaveSlot: () => void;
  onDropDisk: (sectionId: string, slot: number, name: string) => void;
  onSelectDisk: (name: string) => void;
  onRemoveDisk: (name: string) => void;
  onOpenDisk: (disk: Disk) => void;
  onPlaceSelected: (sectionId: string, slot: number) => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState(view.name);
  useEffect(() => setName(view.name), [view.name]);
  const shell = view.kind === "server"
    ? "rounded-[18px] border-line bg-[linear-gradient(150deg,var(--surface-2),var(--surface))] p-3"
    : view.kind === "nas"
      ? "rounded-[24px] border-line bg-[linear-gradient(145deg,var(--surface-2),var(--surface))] p-3"
      : "rounded-[30px] border-line bg-[linear-gradient(150deg,var(--surface-2),var(--surface))] p-3";
  return (
    <div
      className={cn(
        "relative flex h-full min-h-0 flex-col overflow-hidden border shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_30px_80px_-50px_rgba(0,0,0,0.9)]",
        shell,
        draggingView === view.id && "opacity-60",
      )}
    >
      {view.kind === "server" ? (
        <>
          <span className="absolute inset-y-8 -left-1 w-2 rounded-full bg-[var(--surface-3)]" />
          <span className="absolute inset-y-8 -right-1 w-2 rounded-full bg-[var(--surface-3)]" />
        </>
      ) : null}
      {view.kind === "tower" ? (
        <>
          <span className="absolute bottom-1 left-1/2 h-1.5 w-10 -translate-x-1/2 rounded-full bg-[var(--surface-3)]" />
          <span className="absolute inset-x-6 top-2 h-1 rounded-full bg-[var(--surface-3)]/70" />
        </>
      ) : null}

      <div className={cn("relative flex min-h-0 flex-1 flex-col rounded-2xl border border-line bg-[var(--surface)]/75 p-2.5", view.kind === "nas" && "rounded-xl")}>
        <div className="flex shrink-0 flex-wrap items-center gap-1.5">
          <span
            onPointerDown={onDragView}
            title={t("Drag view")}
            className="cursor-grab touch-none rounded p-0.5 text-subtle transition hover:bg-[var(--surface-3)] hover:text-ink active:cursor-grabbing"
          >
            <GripVertical className="size-3.5" />
          </span>
          {view.kind === "tower" ? <PcCase className="size-3.5 text-[var(--accent)]" /> : view.kind === "server" ? <Server className="size-3.5 text-[var(--accent)]" /> : <Box className="size-3.5 text-[var(--accent)]" />}
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
            className="min-w-0 max-w-[150px] flex-1 rounded border-0 bg-transparent text-[11.5px] font-semibold text-ink outline-none focus:bg-[var(--surface-2)] focus:px-1 focus:py-0.5"
          />
          <Badge tone={view.kind === "server" ? "info" : view.kind === "nas" ? "warning" : "violet"}>
            {t(view.kind === "tower" ? "Tower" : view.kind === "server" ? "Server" : "NAS")}
          </Badge>
          <div className="flex items-center rounded-md border border-line bg-[var(--surface-2)] p-0.5">
            {(["s", "m", "l"] as PresetSize[]).map((size) => (
              <button
                key={size}
                type="button"
                onClick={() => onChangeSize(size)}
                className={cn(
                  "grid size-5 place-items-center rounded text-[8.5px] font-semibold uppercase transition",
                  view.size === size ? "bg-[var(--surface)] text-ink shadow-sm" : "text-subtle hover:text-ink",
                )}
              >
                {size}
              </button>
            ))}
          </div>
          <IconButton label={t("Add section")} size="sm" onClick={onAddSection}>
            <Plus className="size-3.5" />
          </IconButton>
          {canRemove ? (
            <IconButton label={t("Remove view")} size="sm" onClick={onRemove}>
              <Trash2 className="size-3.5" />
            </IconButton>
          ) : null}
        </div>

        <div className="mt-2 flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-0.5">
          {view.sections.length === 0 ? (
            <div className="grid flex-1 place-items-center rounded-xl border border-dashed border-line p-4 text-center">
              <div>
                <p className="text-[10px] text-muted">{t("No sections")}</p>
                <Button size="sm" variant="secondary" className="mt-2" icon={<Plus className="size-3" />} onClick={onAddSection}>
                  {t("Add section")}
                </Button>
              </div>
            </div>
          ) : null}
          {view.sections.map((section) => (
            <SectionBlock
              key={section.id}
              viewId={view.id}
              section={section}
              disks={disksBySection.get(`${view.id}:${section.id}`) ?? new Map()}
              selected={selected}
              draggingDisk={draggingDisk}
              draggingSection={draggingSection}
              sectionDrop={sectionDrop}
              overTarget={overTarget}
              onEdit={() => onEditSection(section)}
              onDelete={() => onDeleteSection(section)}
              onDragStart={() => onSectionDragStart(section.id)}
              onDragEnd={onSectionDragEnd}
              onDragOver={() => onSectionDragOver(section.id)}
              onDrop={() => onSectionDrop(section.id)}
              onDragDiskStart={onDragDiskStart}
              onDragDiskEnd={onDragDiskEnd}
              onDragOverSlot={(slot) => onDragOverSlot(section.id, slot)}
              onDragLeaveSlot={onDragLeaveSlot}
              onDropDisk={(slot, name) => onDropDisk(section.id, slot, name)}
              onSelectDisk={onSelectDisk}
              onRemoveDisk={onRemoveDisk}
              onOpenDisk={onOpenDisk}
              onPlaceSelected={(slot) => onPlaceSelected(section.id, slot)}
            />
          ))}
          <div
            onDragOver={(event) => {
              if (!event.dataTransfer.types.includes(SECTION_MIME)) return;
              event.preventDefault();
              onSectionDragOver(undefined);
            }}
            onDrop={(event) => {
              if (!event.dataTransfer.types.includes(SECTION_MIME)) return;
              event.preventDefault();
              onSectionDrop(undefined);
            }}
            className={cn(
              "mt-1 grid min-h-6 place-items-center rounded-lg border border-dashed text-[9px] transition",
              draggingSection?.viewId === view.id
                ? "border-[var(--accent-line)] bg-[var(--accent-soft)]/30 text-[var(--accent)]"
                : "border-transparent text-subtle",
            )}
          >
            {draggingSection?.viewId === view.id ? t("Drop section here") : null}
          </div>
        </div>
        {!stacked ? <span className="mt-1 shrink-0 text-right font-mono text-[8px] text-subtle">Aurora · {view.sections.reduce((sum, section) => sum + section.rows * section.cols, 0)} slots</span> : null}
      </div>
    </div>
  );
}

function SectionBlock({
  viewId,
  section,
  disks,
  selected,
  draggingDisk,
  draggingSection,
  sectionDrop,
  overTarget,
  onEdit,
  onDelete,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDrop,
  onDragDiskStart,
  onDragDiskEnd,
  onDragOverSlot,
  onDragLeaveSlot,
  onDropDisk,
  onSelectDisk,
  onRemoveDisk,
  onOpenDisk,
  onPlaceSelected,
}: {
  viewId: string;
  section: StorageSection;
  disks: Map<number, Disk>;
  selected: string | null;
  draggingDisk: string | null;
  draggingSection: SectionDrag;
  sectionDrop: SectionDrop;
  overTarget: DropTarget;
  onEdit: () => void;
  onDelete: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDragOver: () => void;
  onDrop: () => void;
  onDragDiskStart: (name: string) => void;
  onDragDiskEnd: () => void;
  onDragOverSlot: (slot: number) => void;
  onDragLeaveSlot: () => void;
  onDropDisk: (slot: number, name: string) => void;
  onSelectDisk: (name: string) => void;
  onRemoveDisk: (name: string) => void;
  onOpenDisk: (disk: Disk) => void;
  onPlaceSelected: (slot: number) => void;
}) {
  const { t } = useI18n();
  const slots = section.rows * section.cols;
  const warning = Object.keys(section.assignments).some((name) => {
    const disk = [...disks.values()].find((candidate) => candidate.name === name);
    return Boolean(disk?.media && section.media !== "any" && disk.media !== section.media);
  });
  return (
    <div
      onDragOver={(event) => {
        if (!event.dataTransfer.types.includes(SECTION_MIME)) return;
        event.preventDefault();
        event.stopPropagation();
        onDragOver();
      }}
      onDrop={(event) => {
        if (!event.dataTransfer.types.includes(SECTION_MIME)) return;
        event.preventDefault();
        event.stopPropagation();
        onDrop();
      }}
      className={cn(
        "relative rounded-xl border bg-[var(--surface-2)]/35 p-2 transition",
        sectionDrop?.viewId === viewId && sectionDrop.beforeId === section.id
          ? "border-[var(--accent-line)] ring-1 ring-[var(--accent-line)]"
          : "border-line/70",
        draggingSection?.viewId === viewId && draggingSection.sectionId === section.id && "opacity-50",
      )}
    >
      {sectionDrop?.viewId === viewId && sectionDrop.beforeId === section.id ? (
        <div className="absolute -top-1 left-2 right-2 h-0.5 rounded-full bg-[var(--accent)]" />
      ) : null}
      <div className="flex flex-wrap items-center gap-1.5">
        <span
          draggable
          onDragStart={(event) => {
            event.dataTransfer.effectAllowed = "move";
            event.dataTransfer.setData(SECTION_MIME, section.id);
            onDragStart();
          }}
          onDragEnd={onDragEnd}
          title={t("Drag section")}
          className="cursor-grab text-subtle transition hover:text-ink active:cursor-grabbing"
        >
          <GripVertical className="size-3" />
        </span>
        <span className="size-2 rounded-full" style={{ background: section.color }} />
        <p className="min-w-0 flex-1 truncate text-[10px] font-semibold text-soft">{section.name}</p>
        <Badge tone={mediaTone(section.media)}>{t(sectionMediaKey(section.media))}</Badge>
        <span className="text-[8px] text-subtle">{section.rows}×{section.cols}</span>
        {warning ? (
          <span title={t("Some disks do not match this section media")} className="text-[var(--warning)]">
            <TriangleAlert className="size-3" />
          </span>
        ) : null}
        <IconButton label={t("Edit section")} size="sm" onClick={onEdit}>
          <Pencil className="size-3" />
        </IconButton>
        <IconButton label={t("Remove section")} size="sm" onClick={onDelete}>
          <Trash2 className="size-3" />
        </IconButton>
      </div>
      <div className="mt-2 grid gap-1.5" style={{ gridTemplateColumns: `repeat(${section.cols}, minmax(0, 1fr))` }}>
        {Array.from({ length: slots }, (_, slot) => {
          const disk = disks.get(slot);
          const activeDrop = overTarget?.viewId === viewId && overTarget.sectionId === section.id && overTarget.slot === slot;
          return disk ? (
            <AssignedSlot
              key={slot}
              slot={slot}
              disk={disk}
              sectionMedia={section.media}
              selected={selected === disk.name}
              dragging={draggingDisk === disk.name}
              activeDrop={activeDrop}
              onSelect={() => onSelectDisk(disk.name)}
              onRemove={() => onRemoveDisk(disk.name)}
              onOpen={() => onOpenDisk(disk)}
              onDragStart={() => onDragDiskStart(disk.name)}
              onDragEnd={onDragDiskEnd}
              onDragOver={(event) => {
                if (!event.dataTransfer.types.includes(DISK_MIME)) return;
                event.preventDefault();
                onDragOverSlot(slot);
              }}
              onDragLeave={onDragLeaveSlot}
              onDrop={(event) => {
                if (!event.dataTransfer.types.includes(DISK_MIME)) return;
                event.preventDefault();
                const name = event.dataTransfer.getData(DISK_MIME) || draggingDisk;
                if (name) onDropDisk(slot, name);
              }}
            />
          ) : (
            <button
              key={slot}
              type="button"
              onClick={() => onPlaceSelected(slot)}
              onDragOver={(event) => {
                if (!event.dataTransfer.types.includes(DISK_MIME)) return;
                event.preventDefault();
                onDragOverSlot(slot);
              }}
              onDragLeave={onDragLeaveSlot}
              onDrop={(event) => {
                if (!event.dataTransfer.types.includes(DISK_MIME)) return;
                event.preventDefault();
                const name = event.dataTransfer.getData(DISK_MIME) || draggingDisk;
                if (name) onDropDisk(slot, name);
              }}
              className={cn(
                "flex min-h-[82px] flex-col justify-between rounded-lg border border-dashed p-2 text-left transition",
                activeDrop
                  ? "border-[var(--accent-line)] bg-[var(--accent-soft)]"
                  : "border-line/70 bg-[var(--surface-3)]/20 hover:border-line-strong hover:bg-[var(--surface-3)]/40",
              )}
            >
              <span className="flex items-center justify-between text-[7.5px] uppercase tracking-[0.1em] text-subtle">
                <span>{t("Bay {index}", { index: slot + 1 })}</span>
                <span className="size-1 rounded-full bg-[var(--subtle)]/30" />
              </span>
              <span className="text-center text-[8.5px] text-subtle">{activeDrop ? t("Drop here") : t("Empty")}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function AssignedSlot({
  slot,
  disk,
  sectionMedia,
  selected,
  dragging,
  activeDrop,
  onSelect,
  onRemove,
  onOpen,
  onDragStart,
  onDragEnd,
  onDragOver,
  onDragLeave,
  onDrop,
}: {
  slot: number;
  disk: Disk;
  sectionMedia: string;
  selected: boolean;
  dragging: boolean;
  activeDrop: boolean;
  onSelect: () => void;
  onRemove: () => void;
  onOpen: () => void;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDragOver: (event: DragEvent<HTMLDivElement>) => void;
  onDragLeave: () => void;
  onDrop: (event: DragEvent<HTMLDivElement>) => void;
}) {
  const { t } = useI18n();
  const mismatch = Boolean(disk.media && sectionMedia !== "any" && disk.media !== sectionMedia);
  return (
    <div
      role="button"
      tabIndex={0}
      draggable
      onDragStart={(event) => {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData(DISK_MIME, disk.name);
        onDragStart();
      }}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      onClick={onSelect}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
      style={{
        borderColor: hexToRgba(disk.color || "#8a8f98", selected ? 0.75 : 0.4),
        background: `linear-gradient(145deg, ${hexToRgba(disk.color || "#8a8f98", 0.16)}, color-mix(in srgb, var(--surface-3) 55%, transparent))`,
      }}
      className={cn(
        "flex min-h-[126px] cursor-grab flex-col justify-between rounded-lg border p-2 text-left transition active:cursor-grabbing",
        selected && "ring-1 ring-[var(--accent-line)]",
        activeDrop && "ring-2 ring-[var(--accent-line)]",
        dragging && "opacity-50",
      )}
      title={`${disk.name} · ${t("Drag to move")}`}
    >
      <div className="flex items-center justify-between gap-1">
        <span className="font-mono text-[7.5px] uppercase tracking-[0.1em] text-subtle">{t("Bay {index}", { index: slot + 1 })}</span>
        <span className="flex items-center gap-1">
          <span className="size-1.5 rounded-full" style={{ background: disk.color || "#8a8f98" }} />
          {mismatch ? <TriangleAlert className="size-2.5 text-[var(--warning)]" /> : null}
          <button type="button" aria-label={t("Disk details")} onClick={(event) => { event.stopPropagation(); onOpen(); }} className="text-subtle hover:text-ink">
            <Info className="size-2.5" />
          </button>
          <button type="button" aria-label={t("Remove from bay")} onClick={(event) => { event.stopPropagation(); onRemove(); }} className="text-subtle hover:text-[var(--danger)]">
            <Trash2 className="size-2.5" />
          </button>
        </span>
      </div>
      <div className="mt-0.5 min-w-0">
        <p className="truncate text-[10px] font-semibold text-ink">{disk.name}</p>
        <div className="mt-0.5 flex flex-wrap gap-1">
          <Badge tone={disk.type === "Parity" ? "violet" : disk.type === "Cache" ? "info" : "neutral"}>{t(disk.type)}</Badge>
          {disk.media ? <Badge tone={mediaTone(disk.media)}>{mediaLabel(disk)}</Badge> : null}
        </div>
        <p className="mt-0.5 truncate text-[7.5px] text-muted" title={diskBrandModel(disk)}>{diskBrandModel(disk)}</p>
        <p className="truncate font-mono text-[7.5px] text-subtle" title={disk.serial || undefined}>S/N {disk.serial || "—"}</p>
      </div>
      <p className="mt-0.5 truncate text-[7.5px] text-subtle" title={`${formatBytes(disk.size)} · ${diskPlacement(disk)} · ${disk.temp ? `${disk.temp}°C` : "—"}`}>
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
      onDragStart={(event) => {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData(DISK_MIME, disk.name);
        onDragStart();
      }}
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
        selected ? "border-[var(--accent-line)] bg-[var(--accent-soft)]" : "border-line hover:border-line-strong hover:bg-[var(--surface-2)]",
      )}
    >
      <span className="h-14 w-1 shrink-0 rounded-full" style={{ background: disk.color || "#8a8f98" }} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-1">
          <p className="truncate text-[10.5px] font-medium text-ink">{disk.name}</p>
          <Badge tone={disk.type === "Parity" ? "violet" : disk.type === "Cache" ? "info" : "neutral"}>{t(disk.type)}</Badge>
          {disk.media ? <Badge tone={mediaTone(disk.media)}>{mediaLabel(disk)}</Badge> : null}
        </div>
        <p className="truncate text-[9px] text-muted" title={diskBrandModel(disk)}>{diskBrandModel(disk)}</p>
        <p className="truncate font-mono text-[8px] text-subtle" title={disk.serial || undefined}>S/N {disk.serial || "—"}</p>
        <p className="truncate text-[8px] text-subtle" title={`${formatBytes(disk.size)} · ${diskPlacement(disk)} · ${disk.temp ? `${disk.temp}°C` : "—"}`}>
          {formatBytes(disk.size)} · {diskPlacement(disk)} · {disk.temp ? `${disk.temp}°C` : "—"}
        </p>
      </div>
      <button type="button" aria-label={t("Disk details")} onClick={(event) => { event.stopPropagation(); onOpen(); }} className="text-subtle transition hover:text-ink">
        <Info className="size-3.5" />
      </button>
      <GripVertical className="size-3.5 shrink-0 text-subtle" />
    </div>
  );
}

function SectionEditorModal({
  open,
  section,
  isNew,
  onClose,
  onSave,
}: {
  open: boolean;
  section: StorageSection | null;
  isNew: boolean;
  onClose: () => void;
  onSave: (section: StorageSection) => void;
}) {
  const { t } = useI18n();
  const [draft, setDraft] = useState<StorageSection>(section ?? {
    id: "", name: "", color: "#4aa3ff", media: "any", rows: 2, cols: 2, assignments: {},
  });
  useEffect(() => {
    if (open && section) setDraft({ ...section, assignments: { ...section.assignments } });
  }, [open, section]);
  if (!open || !section) return null;
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isNew ? t("New section") : t("Section settings")}
      description={t("Group drives into a labelled cage inside the chassis.")}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>{t("Cancel")}</Button>
          <Button
            variant="primary"
            disabled={!draft.name.trim()}
            onClick={() => onSave({ ...draft, name: draft.name.trim() })}
          >
            {t("Save section")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <label className="block">
          <span className="mb-1 block text-[9.5px] font-medium uppercase tracking-[0.1em] text-subtle">{t("Section name")}</span>
          <input
            value={draft.name}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })}
            className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[12px] text-ink outline-none focus:border-[var(--accent-line)]"
          />
        </label>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-[9.5px] font-medium uppercase tracking-[0.1em] text-subtle">{t("Media")}</span>
            <select
              value={draft.media}
              onChange={(event) => setDraft({ ...draft, media: event.target.value as StorageSection["media"] })}
              className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[12px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              <option value="any">{t("Any")}</option>
              <option value="hdd">HDD</option>
              <option value="ssd">SSD</option>
              <option value="nvme">NVMe</option>
            </select>
          </label>
          <label className="flex items-end gap-2">
            <span className="mb-2 block text-[9.5px] font-medium uppercase tracking-[0.1em] text-subtle">{t("Color")}</span>
            <input
              type="color"
              value={draft.color}
              onChange={(event) => setDraft({ ...draft, color: event.target.value })}
              className="h-9 w-14 cursor-pointer rounded-[7px] border border-line bg-[var(--surface-2)] p-1"
            />
          </label>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-[9.5px] font-medium uppercase tracking-[0.1em] text-subtle">{t("Rows")}</span>
            <select
              value={draft.rows}
              onChange={(event) => setDraft({ ...draft, rows: Number(event.target.value) })}
              className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[12px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              {[1, 2, 3, 4, 5, 6].map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-[9.5px] font-medium uppercase tracking-[0.1em] text-subtle">{t("Columns")}</span>
            <select
              value={draft.cols}
              onChange={(event) => setDraft({ ...draft, cols: Number(event.target.value) })}
              className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[12px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              {[1, 2, 3, 4, 5, 6].map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
          </label>
        </div>
        <p className="text-[9.5px] text-subtle">
          {t("{count} slots in this section.", { count: draft.rows * draft.cols })}
        </p>
      </div>
    </Modal>
  );
}
