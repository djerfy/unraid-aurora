import {
  ArrowRightLeft,
  Bell,
  Boxes,
  CalendarClock,
  Cpu,
  Gauge,
  HardDrive,
  Info,
  ListChecks,
  MonitorPlay,
  Network,
  Puzzle,
  ScrollText,
  Settings2,
  ShoppingBag,
  SquareTerminal,
  FileCode2,
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
  { to: "/apps", label: "Applications", icon: ShoppingBag, group: "Infrastructure" },
  { to: "/vms", label: "Virtual machines", icon: MonitorPlay, group: "Infrastructure" },
  { to: "/network", label: "Network", icon: Network, group: "Infrastructure" },
  { to: "/gpus", label: "GPUs", icon: Cpu, group: "Infrastructure" },
  { to: "/balance", label: "Balance", icon: ArrowRightLeft, group: "Infrastructure" },
  { to: "/users", label: "Users", icon: Users, group: "System" },
  { to: "/scheduler", label: "Scheduler", icon: CalendarClock, group: "System" },
  { to: "/plugins", label: "Plugins", icon: Puzzle, group: "System" },
  { to: "/logs", label: "Logs", icon: ScrollText, group: "System" },
  { to: "/terminal", label: "Terminal", icon: SquareTerminal, group: "System" },
  { to: "/tools", label: "Tools", icon: Wrench, group: "System" },
  { to: "/tasks", label: "Tasks", icon: ListChecks, group: "System" },
  { to: "/scripts", label: "User scripts", icon: FileCode2, group: "System" },
  { to: "/settings", label: "Settings", icon: Settings2, group: "System" },
  { to: "/about", label: "About", icon: Info, group: "System" },
];

export const PAGE_META: Record<string, { title: string; section: string }> = {
  "/": { title: "Dashboard", section: "Overview" },
  "/notifications": { title: "Notifications", section: "Overview" },
  "/storage": { title: "Storage", section: "Infrastructure" },
  "/docker": { title: "Containers", section: "Infrastructure" },
  "/apps": { title: "Applications", section: "Infrastructure" },
  "/vms": { title: "Virtual machines", section: "Infrastructure" },
  "/network": { title: "Network", section: "Infrastructure" },
  "/gpus": { title: "GPUs", section: "Infrastructure" },
  "/balance": { title: "Balance", section: "Infrastructure" },
  "/users": { title: "Users", section: "System" },
  "/scheduler": { title: "Scheduler", section: "System" },
  "/plugins": { title: "Plugins", section: "System" },
  "/logs": { title: "Logs", section: "System" },
  "/terminal": { title: "Terminal", section: "System" },
  "/tools": { title: "Tools", section: "System" },
  "/tasks": { title: "Tasks", section: "System" },
  "/scripts": { title: "User scripts", section: "System" },
  "/settings": { title: "Settings", section: "System" },
  "/about": { title: "About", section: "System" },
};
