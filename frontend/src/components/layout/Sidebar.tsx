import { NavLink, useNavigate } from "react-router-dom";
import {
  ChevronLeft,
  ChevronsUpDown,
  Command,
  ExternalLink,
  HardDrive,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { cn, Progress } from "../ui/primitives";
import { EntityIcon, userIconURL } from "../ui/entity-icon";
import { AuroraWordmark } from "./Brand";
import { NAV_GROUPS, NAV_ITEMS } from "./nav";
import { useApp } from "../../app/AppDataProvider";
import { useToast } from "../../app/ToastProvider";
import { formatBytes, formatUptime } from "../../lib/format";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";

export function Sidebar({
  collapsed,
  onToggle,
  onOpenCommand,
}: {
  collapsed: boolean;
  onToggle: () => void;
  onOpenCommand: () => void;
}) {
  const { snapshot, session, live } = useApp();
  const { toast } = useToast();
  const { t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const unread = snapshot?.notifications.filter((n) => !n.read && !n.archived).length ?? 0;

  const signOut = async () => {
    try {
      await api.logout();
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      queryClient.removeQueries({ queryKey: ["bootstrap"] });
      toast({ kind: "info", title: t("Signed out"), description: t("Your session has been closed.") });
      navigate("/login");
    } catch (error) {
      toast({ kind: "error", title: t("Sign out failed"), description: (error as Error).message });
    }
  };

  return (
    <aside
      className={cn(
        "fixed inset-y-0 left-0 z-40 hidden flex-col border-r border-line bg-[var(--surface)]/72 backdrop-blur-2xl transition-[width] duration-300 lg:flex",
        collapsed ? "w-[82px]" : "w-[264px]",
      )}
    >
      <div className={cn("flex h-16 shrink-0 items-center border-b border-line", collapsed ? "justify-between px-2" : "justify-between px-4")}>
        <button
          type="button"
          onClick={() => navigate("/")}
          className="shrink-0 rounded-lg text-left transition-opacity hover:opacity-80"
          aria-label={t("Aurora home")}
        >
          <AuroraWordmark compact={collapsed} />
        </button>
        <button
          type="button"
          onClick={onToggle}
          aria-label={collapsed ? t("Expand sidebar") : t("Collapse sidebar")}
          title={collapsed ? t("Expand sidebar") : t("Collapse sidebar")}
          className={cn(
            "grid shrink-0 place-items-center rounded-lg text-subtle transition hover:bg-[var(--surface-3)] hover:text-ink",
            collapsed ? "size-7" : "size-8",
          )}
        >
          {collapsed ? <PanelLeftOpen className="size-4" /> : <PanelLeftClose className="size-4" />}
        </button>
      </div>

      {snapshot ? (
        collapsed ? (
          <button
            type="button"
            onClick={() => navigate("/settings")}
            title={t("{name} · Unraid {version} · {uptime} uptime", {
              name: snapshot.system.name,
              version: snapshot.system.version,
              uptime: formatUptime(snapshot.system.uptime),
            })}
            aria-label={t("{name}, online", { name: snapshot.system.name })}
            className="mx-auto mt-4 grid size-10 place-items-center rounded-xl border border-line bg-[var(--surface-2)] transition hover:border-line-strong hover:bg-[var(--surface-3)]"
          >
            <span className={cn("status-dot", live ? "live" : "muted")} />
          </button>
        ) : (
          <button
            type="button"
            onClick={() => navigate("/settings")}
            className="group mx-3 mt-4 rounded-2xl border border-line bg-[var(--surface-2)]/80 p-3 text-left transition hover:border-line-strong hover:bg-[var(--surface-3)]/70"
          >
            <div className="flex items-center justify-between">
              <span className="flex min-w-0 items-center gap-2">
                <span className={cn("status-dot", live ? "live" : "muted")} />
                <span className="truncate text-[12.5px] font-semibold text-ink">{snapshot.system.name}</span>
              </span>
              <ChevronsUpDown className="size-3.5 shrink-0 text-subtle" />
            </div>
            <div className="mt-1.5 flex items-center justify-between text-[10px] text-subtle">
              <span>Unraid {snapshot.system.version}</span>
              <span className="flex items-center gap-1">
                <span className="size-1 rounded-full bg-[var(--success)]" />
                {live ? t("Live") : t("Offline")}
              </span>
            </div>
            <div className="mt-2.5">
              <Progress value={snapshot.array.usage} height={3} />
              <div className="mt-1.5 flex justify-between font-mono text-[9px] text-subtle">
                <span>{t("{used} used", { used: formatBytes(snapshot.array.used) })}</span>
                <span>{t("{free} free", { free: formatBytes(snapshot.array.free) })}</span>
              </div>
            </div>
          </button>
        )
      ) : (
        <div className={cn("mt-4", collapsed ? "mx-3 h-16" : "mx-3 h-[92px] skeleton rounded-2xl")} />
      )}

      <nav className={cn("mt-5 flex-1 overflow-y-auto overflow-x-hidden pb-4", collapsed ? "px-2" : "px-3")}>
        {NAV_GROUPS.map((group) => {
          const items = NAV_ITEMS.filter((item) => item.group === group);
          return (
            <div key={group} className="mb-5 last:mb-0">
              {!collapsed ? (
                <p className="mb-1.5 px-2.5 text-[9.5px] font-semibold uppercase tracking-[0.16em] text-subtle">{t(group)}</p>
              ) : (
                <div className="mx-auto mb-2 h-px w-6 bg-line" />
              )}
              <div className="space-y-0.5">
                {items.map((item) => {
                  const Icon = item.icon;
                  const badge = item.badge === "notifications" ? unread : 0;
                  const link = (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      end={item.to === "/"}
                      title={collapsed ? t(item.label) : undefined}
                      aria-label={collapsed ? t(item.label) : undefined}
                      className={({ isActive }) =>
                        cn(
                          "group/nav relative flex w-full items-center rounded-[7px] py-2 text-[12.5px] font-medium transition-all duration-200",
                          collapsed ? "justify-center px-0" : "gap-2.5 px-2.5",
                          isActive
                            ? "bg-[var(--surface-3)] text-ink shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]"
                            : "text-muted hover:bg-[var(--surface-2)] hover:text-soft",
                        )
                      }
                    >
                      {({ isActive }) => (
                        <>
                          {isActive ? (
                            <span className="absolute inset-y-2 left-0 w-[2px] rounded-full bg-[var(--accent)]" />
                          ) : null}
                          <Icon
                            className={cn("size-[16px] shrink-0 transition-colors", isActive ? "text-[var(--accent)]" : "text-subtle group-hover/nav:text-muted")}
                            strokeWidth={1.8}
                          />
                          {!collapsed ? <span className="flex-1 truncate">{t(item.label)}</span> : null}
                          {!collapsed && badge > 0 ? (
                            <span className="grid h-[18px] min-w-[18px] place-items-center rounded-full bg-[var(--accent-soft)] px-1 text-[9.5px] font-semibold text-[var(--accent)]">
                              {badge}
                            </span>
                          ) : null}
                          {collapsed && badge > 0 ? (
                            <span className="absolute right-2 top-1.5 size-1.5 rounded-full bg-[var(--accent)]" />
                          ) : null}
                        </>
                      )}
                    </NavLink>
                  );
                  return link;
                })}
              </div>
            </div>
          );
        })}
      </nav>

      <div className={cn("shrink-0 border-t border-line", collapsed ? "p-2" : "p-3")}>
        {!collapsed ? (
          <>
            <button
              type="button"
              onClick={onOpenCommand}
              className="mb-2.5 flex w-full items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 py-1.5 text-[11px] text-subtle transition hover:border-line-strong hover:text-muted"
            >
              <Command className="size-3" />
              <span className="flex-1 text-left">{t("Quick actions")}</span>
              <kbd className="rounded border border-line bg-[var(--surface)] px-1 font-mono text-[9px]">⌘K</kbd>
            </button>
            <div className="flex items-center gap-2.5 rounded-xl p-1.5 hover:bg-[var(--surface-2)]">
              <EntityIcon src={userIconURL(session?.user ?? "root")} name={session?.user ?? "root"} size={30} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-[12px] font-medium text-ink">{session?.user ?? "root"}</p>
                <p className="truncate text-[10px] text-subtle">{t("Host administrator")}</p>
              </div>
              <button
                type="button"
                onClick={signOut}
                title={t("Sign out")}
                aria-label={t("Sign out")}
                className="grid size-7 place-items-center rounded-md text-subtle transition hover:bg-[var(--surface-3)] hover:text-[var(--danger)]"
              >
                <LogOut className="size-3.5" />
              </button>
            </div>
          </>
        ) : (
          <div className="flex flex-col items-center gap-2">
            <EntityIcon src={userIconURL(session?.user ?? "root")} name={session?.user ?? "root"} size={30} />
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={onOpenCommand}
                title={t("Quick actions (⌘K)")}
                aria-label={t("Quick actions")}
                className="grid size-7 place-items-center rounded-md text-subtle transition hover:bg-[var(--surface-2)] hover:text-ink"
              >
                <Command className="size-3.5" />
              </button>
              <button
                type="button"
                onClick={signOut}
                title={t("Sign out")}
                aria-label={t("Sign out")}
                className="grid size-7 place-items-center rounded-md text-subtle transition hover:bg-[var(--surface-2)] hover:text-[var(--danger)]"
              >
                <LogOut className="size-3.5" />
              </button>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
}

export function MobileSidebar({
  open,
  onClose,
  onOpenCommand,
}: {
  open: boolean;
  onClose: () => void;
  onOpenCommand: () => void;
}) {
  const { snapshot } = useApp();
  const { t } = useI18n();
  const unread = snapshot?.notifications.filter((n) => !n.read && !n.archived).length ?? 0;
  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[70] lg:hidden">
      <div className="animate-fade-in absolute inset-0 bg-black/55 backdrop-blur-[4px]" onClick={onClose} />
      <div className="animate-slide-in absolute inset-y-0 left-0 flex w-[286px] flex-col border-r border-line bg-[var(--surface)] shadow-2xl">
        <div className="flex h-16 items-center justify-between border-b border-line px-4">
          <AuroraWordmark />
          <button
            type="button"
            onClick={onClose}
            aria-label={t("Close menu")}
            className="grid size-8 place-items-center rounded-lg text-muted hover:bg-[var(--surface-3)]"
          >
            <ChevronLeft className="size-5" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-3 py-4">
          {NAV_GROUPS.map((group) => (
            <div key={group} className="mb-5">
              <p className="mb-1.5 px-2.5 text-[9.5px] font-semibold uppercase tracking-[0.16em] text-subtle">{t(group)}</p>
              {NAV_ITEMS.filter((item) => item.group === group).map((item) => {
                const Icon = item.icon;
                const badge = item.badge === "notifications" ? unread : 0;
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.to === "/"}
                    onClick={onClose}
                    className={({ isActive }) =>
                      cn(
                        "flex items-center gap-2.5 rounded-[7px] px-2.5 py-2 text-[13px] font-medium",
                        isActive ? "bg-[var(--surface-3)] text-ink" : "text-muted",
                      )
                    }
                  >
                    <Icon className="size-4 text-subtle" />
                    <span className="flex-1">{t(item.label)}</span>
                    {badge > 0 ? (
                      <span className="grid h-4 min-w-4 place-items-center rounded-full bg-[var(--accent-soft)] px-1 text-[9px] text-[var(--accent)]">
                        {badge}
                      </span>
                    ) : null}
                  </NavLink>
                );
              })}
            </div>
          ))}
          <button
            type="button"
            onClick={() => {
              onClose();
              onOpenCommand();
            }}
            className="mt-1 flex w-full items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)] px-3 py-2 text-xs text-muted"
          >
            <Command className="size-3.5" /> {t("Quick actions")}
          </button>
        </div>
        <div className="border-t border-line p-4">
          <div className="flex items-center gap-2 text-[11px] text-subtle">
            <HardDrive className="size-3.5" /> {snapshot?.system.name ?? "Aurora"} · Unraid{" "}
            {snapshot?.system.version ?? "7.2"}
          </div>
          <a
            href="https://unraid.net"
            target="_blank"
            rel="noreferrer"
            className="mt-2 inline-flex items-center gap-1 text-[11px] text-muted hover:text-ink"
          >
            unraid.net <ExternalLink className="size-3" />
          </a>
        </div>
      </div>
    </div>
  );
}
