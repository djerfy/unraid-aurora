import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  Bell,
  BookOpen,
  CheckCheck,
  ChevronRight,
  ExternalLink,
  LogOut,
  Menu as MenuIcon,
  Moon,
  Search,
  Settings2,
  Sun,
} from "lucide-react";
import { IconButton, Kbd, cn } from "../ui/primitives";
import { EntityIcon, userIconURL } from "../ui/entity-icon";
import { useApp } from "../../app/AppDataProvider";
import { useTheme } from "../../app/ThemeProvider";
import { useToast } from "../../app/ToastProvider";
import { useAction } from "../../lib/useAction";
import { api } from "../../lib/api";
import { relativeTime } from "../../lib/format";
import { PAGE_META } from "./nav";
import { Menu } from "../ui/overlays";
import { useI18n } from "../../i18n";

export function Topbar({
  onOpenCommand,
  onOpenMobile,
}: {
  onOpenCommand: () => void;
  onOpenMobile: () => void;
}) {
  const location = useLocation();
  const navigate = useNavigate();
  const { snapshot, session, live } = useApp();
  const { resolvedTheme, setTheme } = useTheme();
  const { toast } = useToast();
  const { t } = useI18n();
  const action = useAction();
  const queryClient = useQueryClient();
  const meta = PAGE_META[location.pathname] ?? { title: "Aurora", section: "Overview" };

  const [notifOpen, setNotifOpen] = useState(false);
  const notifRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!notifOpen) return;
    const onClick = (event: MouseEvent) => {
      if (notifRef.current && !notifRef.current.contains(event.target as Node)) setNotifOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [notifOpen]);

  const notifications = snapshot?.notifications ?? [];
  const unread = notifications.filter((n) => !n.read);

  const signOut = async () => {
    try {
      await api.logout();
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      queryClient.removeQueries({ queryKey: ["bootstrap"] });
      navigate("/login");
    } catch (error) {
      toast({ kind: "error", title: t("Sign out failed"), description: (error as Error).message });
    }
  };

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-line bg-[var(--bg)]/72 px-4 backdrop-blur-2xl sm:px-6 lg:px-8">
      <button
        type="button"
        onClick={onOpenMobile}
        aria-label={t("Open navigation")}
        className="grid size-9 place-items-center rounded-lg text-muted transition hover:bg-[var(--surface-3)] hover:text-ink lg:hidden"
      >
        <MenuIcon className="size-[18px]" />
      </button>

      <div className="hidden min-w-0 items-center gap-2 text-[11.5px] text-subtle sm:flex">
        <span className="flex items-center gap-1.5 font-medium text-muted">
          <span className={cn("status-dot", live ? "live" : "muted")} />
          {snapshot?.system.name ?? "Vault"}
        </span>
        <ChevronRight className="size-3 text-subtle/70" />
        <span className="truncate">{t(meta.section)}</span>
        <ChevronRight className="size-3 text-subtle/70" />
        <span className="truncate font-medium text-soft">{t(meta.title)}</span>
      </div>

      <div className="flex-1" />

      <button
        type="button"
        onClick={onOpenCommand}
        className="hidden h-9 w-full max-w-[270px] items-center gap-2 rounded-[7px] border border-line bg-[var(--surface-2)]/80 px-3 text-left text-[12px] text-subtle transition hover:border-line-strong hover:bg-[var(--surface-2)] hover:text-muted sm:flex"
      >
        <Search className="size-3.5 shrink-0" />
        <span className="flex-1 truncate">{t("Search or jump to…")}</span>
        <Kbd>⌘K</Kbd>
      </button>

      <IconButton
        label={resolvedTheme === "dark" ? t("Switch to light theme") : t("Switch to dark theme")}
        onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
        className="hidden sm:grid"
      >
        {resolvedTheme === "dark" ? <Sun className="size-[17px]" /> : <Moon className="size-[17px]" />}
      </IconButton>

      <div className="relative" ref={notifRef}>
        <IconButton
          label={t("Notifications")}
          onClick={() => setNotifOpen((value) => !value)}
          className={cn(notifOpen && "bg-[var(--surface-3)] text-ink")}
        >
          <Bell className="size-[17px]" />
          {unread.length > 0 ? (
            <span className="absolute right-0.5 top-0.5 grid min-w-[15px] place-items-center rounded-full bg-[var(--accent)] px-[3px] text-[8px] font-bold leading-[14px] text-[var(--accent-contrast)]">
              {unread.length}
            </span>
          ) : null}
        </IconButton>

        {notifOpen ? (
          <div className="animate-scale-in absolute right-0 top-[calc(100%+8px)] w-[360px] origin-top-right overflow-hidden rounded-2xl border border-line bg-[var(--surface)] shadow-[var(--shadow-pop)]">
            <div className="flex items-center justify-between border-b border-line px-4 py-3">
              <div>
                <p className="text-[13px] font-semibold text-ink">{t("Notifications")}</p>
                <p className="text-[10.5px] text-subtle">
                  {unread.length > 0 ? t("{count} unread", { count: unread.length }) : t("You are all caught up")}
                </p>
              </div>
              {unread.length > 0 ? (
                <button
                  type="button"
                  onClick={() => action.mutate({ kind: "notification.ackall", success: t("All notifications acknowledged") })}
                  className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[10.5px] font-medium text-[var(--accent)] transition hover:bg-[var(--accent-soft)]"
                >
                  <CheckCheck className="size-3" /> {t("Mark all")}
                </button>
              ) : null}
            </div>
            <div className="max-h-[330px] overflow-y-auto">
              {notifications.slice(0, 5).map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => {
                    setNotifOpen(false);
                    navigate("/notifications");
                  }}
                  className="flex w-full gap-3 border-b border-line/60 px-4 py-3 text-left transition hover:bg-[var(--surface-2)]"
                >
                  <span
                    className="mt-1 size-1.5 shrink-0 rounded-full"
                    style={{
                      background:
                        item.importance === "alert"
                          ? "var(--danger)"
                          : item.importance === "warning"
                            ? "var(--warning)"
                            : "var(--info)",
                    }}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[12px] font-medium text-ink">{item.title}</span>
                    <span className="mt-0.5 block truncate text-[11px] text-muted">{item.subject}</span>
                    <span className="mt-1 block font-mono text-[9.5px] text-subtle">{relativeTime(item.timestamp)}</span>
                  </span>
                  {!item.read ? <span className="mt-1.5 size-1 shrink-0 rounded-full bg-[var(--accent)]" /> : null}
                </button>
              ))}
              {notifications.length === 0 ? (
                <p className="px-4 py-8 text-center text-xs text-muted">{t("No notifications yet.")}</p>
              ) : null}
            </div>
            <button
              type="button"
              onClick={() => {
                setNotifOpen(false);
                navigate("/notifications");
              }}
              className="flex w-full items-center justify-center gap-1.5 bg-[var(--surface-2)]/60 py-2.5 text-[11px] font-medium text-muted transition hover:text-ink"
            >
              {t("View all notifications")} <ArrowRight className="size-3" />
            </button>
          </div>
        ) : null}
      </div>

      <div className="mx-1 hidden h-5 w-px bg-line sm:block" />

      <Menu
        width="w-56"
        trigger={
          <button type="button" className="flex items-center gap-2 rounded-full py-1 pl-1 pr-1 transition hover:bg-[var(--surface-2)] sm:pr-2">
            <EntityIcon src={userIconURL(session?.user ?? "root")} name={session?.user ?? "root"} size={28} />
            <span className="hidden max-w-[100px] truncate text-[12px] font-medium text-soft md:block">
              {session?.user ?? "admin"}
            </span>
          </button>
        }
        items={[
          {
            label: t("Account settings"),
            icon: <Settings2 className="size-3.5" />,
            onSelect: () => navigate("/settings"),
          },
          {
            label: t("Documentation"),
            icon: <BookOpen className="size-3.5" />,
            onSelect: () => window.open("https://docs.unraid.net", "_blank", "noopener"),
          },
          {
            label: "Unraid.net",
            icon: <ExternalLink className="size-3.5" />,
            onSelect: () => window.open("https://unraid.net", "_blank", "noopener"),
          },
          { label: "", separator: true },
          {
            label: t("Sign out"),
            icon: <LogOut className="size-3.5" />,
            danger: true,
            onSelect: () => void signOut(),
          },
        ]}
      />
    </header>
  );
}
