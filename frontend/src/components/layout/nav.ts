import {
  Bell,
  Boxes,
  Cpu,
  Gauge,
  HardDrive,
  History,
  MonitorPlay,
  Network,
  Puzzle,
  ScrollText,
  Settings2,
  SquareTerminal,
  Wrench,
  Users,
  type LucideIcon,
} from "lucide-react";

export type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  group: string;
  badge?: "notifications";
};

export const NAV_GROUPS = ["Overview", "Infrastructure", "System"];

export const NAV_ITEMS: NavItem[] = [
  { to: "/", label: "Dashboard", icon: Gauge, group: "Overview" },
  { to: "/notifications", label: "Notifications", icon: Bell, group: "Overview", badge: "notifications" },
  { to: "/storage", label: "Storage", icon: HardDrive, group: "Infrastructure" },
  { to: "/docker", label: "Containers", icon: Boxes, group: "Infrastructure" },
  { to: "/vms", label: "Virtual machines", icon: MonitorPlay, group: "Infrastructure" },
  { to: "/network", label: "Network", icon: Network, group: "Infrastructure" },
  { to: "/gpus", label: "GPUs", icon: Cpu, group: "Infrastructure" },
  { to: "/users", label: "Users", icon: Users, group: "System" },
  { to: "/plugins", label: "Plugins", icon: Puzzle, group: "System" },
  { to: "/logs", label: "Logs", icon: ScrollText, group: "System" },
  { to: "/terminal", label: "Terminal", icon: SquareTerminal, group: "System" },
  { to: "/tools", label: "Tools", icon: Wrench, group: "System" },
  { to: "/history", label: "Historique", icon: History, group: "System" },
  { to: "/settings", label: "Settings", icon: Settings2, group: "System" },
];

export const PAGE_META: Record<string, { title: string; section: string }> = {
  "/": { title: "Dashboard", section: "Overview" },
  "/notifications": { title: "Notifications", section: "Overview" },
  "/storage": { title: "Storage", section: "Infrastructure" },
  "/docker": { title: "Containers", section: "Infrastructure" },
  "/vms": { title: "Virtual machines", section: "Infrastructure" },
  "/network": { title: "Network", section: "Infrastructure" },
  "/gpus": { title: "GPUs", section: "Infrastructure" },
  "/users": { title: "Users", section: "System" },
  "/plugins": { title: "Plugins", section: "System" },
  "/logs": { title: "Logs", section: "System" },
  "/terminal": { title: "Terminal", section: "System" },
  "/tools": { title: "Tools", section: "System" },
  "/history": { title: "Historique", section: "System" },
  "/settings": { title: "Settings", section: "System" },
};
