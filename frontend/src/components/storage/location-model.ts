import type { Disk, StorageSection, StorageView } from "../../lib/types";

export const BOARD_COLUMNS = 12;
export const ROW_HEIGHT = 24;
export const GRID_GAP = 12;

export type PanelKind = StorageView["kind"];

// "tower" is the desktop chassis form factor; the UI labels it Desktop.
const DEFAULT_DIMENSIONS: Record<PanelKind, { w: number; h: number }> = {
  tower: { w: 6, h: 20 },
  server: { w: 9, h: 14 },
  nas: { w: 6, h: 14 },
};

export function defaultDimensions(kind: PanelKind) {
  return DEFAULT_DIMENSIONS[kind];
}

export function diskAccent(disk: Disk): string {
  if (disk.type === "Parity") return "#9B7BFF";
  if (disk.type === "Cache") return "#5AA9FF";
  return "#3DDC97";
}

export function diskMediaKind(disk: Disk): "hdd" | "ssd" | "nvme" {
  if (disk.media === "nvme" || disk.media === "ssd" || disk.media === "hdd") return disk.media;
  return disk.rotational ? "hdd" : "ssd";
}

export function rectsOverlap(
  a: { x: number; y: number; w: number; h: number },
  b: { x: number; y: number; w: number; h: number },
): boolean {
  return a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
}

export function findFreeSpot(
  views: StorageView[],
  w: number,
  h: number,
  preferredX = 0,
  preferredY = 0,
  ignoreId?: string,
): { x: number; y: number } | null {
  const others = views.filter((view) => view.id !== ignoreId);
  const preferred = {
    x: Math.max(0, Math.min(BOARD_COLUMNS - w, preferredX)),
    y: Math.max(0, preferredY),
  };
  if (!others.some((view) => rectsOverlap({ ...preferred, w, h }, view))) return preferred;
  for (let y = 0; y < 240; y += 1) {
    for (let x = 0; x <= BOARD_COLUMNS - w; x += 1) {
      if (!others.some((view) => rectsOverlap({ x, y, w, h }, view))) return { x, y };
    }
  }
  return null;
}

export function diskModelLabel(disk: Disk): string {
  const model = disk.model || disk.device;
  if (!disk.brand) return model;
  const prefix = `${disk.brand.toLowerCase()} `;
  if (model.toLowerCase().startsWith(prefix)) return model.slice(disk.brand.length).trim();
  return model;
}

export function diskBrandModel(disk: Disk): string {
  const model = diskModelLabel(disk);
  return disk.brand ? `${disk.brand} · ${model}` : model;
}

export function defaultSection(kind: PanelKind, index = 1): StorageSection {
  const rows = kind === "tower" ? 4 : kind === "server" ? 2 : 1;
  const cols = kind === "tower" ? 2 : 4;
  return {
    id: `section-${Date.now()}-${index}`,
    name: "Data",
    color: "#4aa3ff",
    media: "any",
    rows,
    cols,
    assignments: {},
  };
}

export function defaultView(kind: PanelKind, index: number, label?: string): StorageView {
  const dims = defaultDimensions(kind);
  const fallbackLabel = kind === "tower" ? "Desktop" : kind === "server" ? "Server" : "NAS";
  return {
    id: `view-${Date.now()}-${index}`,
    name: `${label ?? fallbackLabel} ${index}`,
    kind,
    size: "custom",
    x: 0,
    y: 0,
    w: dims.w,
    h: dims.h,
    sections: [defaultSection(kind, index)],
  };
}
