import { useEffect, useMemo, useState } from "react";
import { Outlet, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import {
  Boxes,
  MonitorPlay,
  Play,
  RefreshCw,
  RotateCw,
  Square,
  Sun,
  Moon,
} from "lucide-react";
import { Sidebar, MobileSidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { CommandPalette, type Command } from "../ui/overlays";
import { Button } from "../ui/primitives";
import { useApp } from "../../app/AppDataProvider";
import { useTheme } from "../../app/ThemeProvider";
import { useAction } from "../../lib/useAction";
import { useHotkeys } from "../../lib/hooks";
import { NAV_ITEMS } from "./nav";
import { EntityIcon, containerIconURL, vmIconURL } from "../ui/entity-icon";
import { AuroraWordmark } from "./Brand";

// Shortcuts must not steal keystrokes from editors, terminals or consoles.
function isTypingTarget(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null;
  if (!element) return false;
  return ["INPUT", "TEXTAREA", "SELECT", "CANVAS"].includes(element.tagName) || element.isContentEditable;
}

function BootScreen({ message = "Connecting to your server" }: { message?: string }) {
  return (
    <div className="relative grid min-h-dvh place-items-center bg-[var(--bg)]">
      <div className="app-atmosphere" />
      <div className="relative flex flex-col items-center">
        <AuroraWordmark />
        <div className="mt-8 flex items-center gap-2 text-[11px] text-subtle">
          <span className="status-dot live" />
          {message}
        </div>
        <div className="mt-3 h-px w-40 overflow-hidden bg-[var(--surface-3)]">
          <div className="h-px w-1/2 animate-[shimmer_1.4s_infinite] bg-[var(--accent)]" />
        </div>
      </div>
    </div>
  );
}

function ErrorScreen({ error, onRetry }: { error: Error; onRetry: () => void }) {
  const navigate = useNavigate();
  return (
    <div className="relative grid min-h-dvh place-items-center bg-[var(--bg)] px-6">
      <div className="app-atmosphere" />
      <div className="card relative w-full max-w-md p-8 text-center">
        <p className="eyebrow text-[var(--danger)]">Connection lost</p>
        <h1 className="mt-3 text-xl font-semibold text-ink">Aurora cannot reach the server</h1>
        <p className="mt-2 text-xs leading-relaxed text-muted">{error.message}</p>
        <div className="mt-6 flex justify-center gap-2">
          <Button variant="primary" onClick={onRetry}>Try again</Button>
          <Button variant="secondary" onClick={() => navigate("/login")}>Sign in</Button>
        </div>
      </div>
    </div>
  );
}

export function AppShell() {
  const { snapshot, session, error, refresh } = useApp();
  const { resolvedTheme, setTheme, setPreferences } = useTheme();
  const action = useAction();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const isTerminal = location.pathname === "/terminal";
  const isStandaloneConsole = location.pathname.startsWith("/vms/console/");
  const terminalPopout = isTerminal && searchParams.get("popout") === "1";
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem("aurora.sidebar") === "collapsed";
    } catch {
      return false;
    }
  });
  const [mobileOpen, setMobileOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);

  useHotkeys(
    useMemo(
      () => ({
        "mod+k": (event: KeyboardEvent) => {
          if (isTypingTarget(event.target)) return;
          event.preventDefault();
          setCommandOpen((value) => !value);
        },
        "/": (event: KeyboardEvent) => {
          if (isTypingTarget(event.target)) return;
          event.preventDefault();
          setCommandOpen(true);
        },
      }),
      [],
    ),
  );

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [location.pathname]);

  // A browser that has never chosen a theme adopts the server preference once.
  useEffect(() => {
    if (!snapshot) return;
    try {
      if (localStorage.getItem("aurora.theme")) return;
      setPreferences({
        theme: snapshot.settings.theme as "dark" | "light" | "system",
        accent: snapshot.settings.accent as "ember" | "violet" | "cyan" | "emerald" | "amber",
        density: snapshot.settings.density as "comfortable" | "compact",
      });
    } catch {
      // Ignore storage failures; the default theme still applies.
    }
  }, [snapshot, setPreferences]);

  useEffect(() => {
    try {
      localStorage.setItem("aurora.sidebar", collapsed ? "collapsed" : "expanded");
    } catch {
      // Ignore storage failures; the sidebar state stays in memory.
    }
  }, [collapsed]);

  const commands = useMemo<Command[]>(() => {
    if (!snapshot) return [];
    const navigation: Command[] = NAV_ITEMS.map((item) => ({
      id: `nav-${item.to}`,
      label: item.label,
      hint: `Go to ${item.label.toLowerCase()}`,
      group: "Navigation",
      icon: <item.icon className="size-3.5" />,
      run: () => navigate(item.to),
    }));

    const globals: Command[] = [
      {
        id: "theme",
        label: resolvedTheme === "dark" ? "Switch to light theme" : "Switch to dark theme",
        hint: "Appearance",
        group: "Actions",
        icon: resolvedTheme === "dark" ? <Sun className="size-3.5" /> : <Moon className="size-3.5" />,
        run: () => setTheme(resolvedTheme === "dark" ? "light" : "dark"),
      },
      {
        id: "refresh",
        label: "Refresh server data",
        hint: "Re-sync all widgets",
        group: "Actions",
        icon: <RefreshCw className="size-3.5" />,
        run: () => refresh(),
      },
      {
        id: "new-container",
        label: "Open containers",
        hint: `${snapshot.containers.filter((c) => c.state === "running").length} running`,
        group: "Actions",
        icon: <Boxes className="size-3.5" />,
        run: () => navigate("/docker"),
      },
      {
        id: "new-vm",
        label: "Open virtual machines",
        hint: `${snapshot.vms.filter((v) => v.state === "running").length} running`,
        group: "Actions",
        icon: <MonitorPlay className="size-3.5" />,
        run: () => navigate("/vms"),
      },
    ];

    const containers: Command[] = snapshot.containers.map((container) => ({
      id: `ct-open-${container.id}`,
      label: container.name,
      hint: `Container · ${container.state} · ${container.image}`,
      group: "Containers",
      keywords: `container docker ${container.state} ${container.image} ${container.category} ${container.network} ${container.ip}`,
      icon: (
        <EntityIcon
          src={containerIconURL(container.id)}
          name={container.name}
          size={20}
          className="rounded-md"
        />
      ),
      run: () => navigate(`/docker?container=${encodeURIComponent(container.name)}`),
    }));

    const containerActions: Command[] = snapshot.containers.map((container) => {
      const running = container.state === "running";
      return {
        id: `ct-action-${container.id}`,
        label: running ? `Restart ${container.name}` : `Start ${container.name}`,
        hint: `${container.state} · ${container.image}`,
        group: "Container actions",
        keywords: `docker container ${container.state} ${container.image}`,
        icon: running ? <RotateCw className="size-3.5" /> : <Play className="size-3.5" />,
        run: () =>
          action.mutate({
            kind: running ? "container.restart" : "container.start",
            target: container.name,
            success: running ? `${container.name} restarted` : `${container.name} started`,
          }),
      };
    });

    const vms: Command[] = snapshot.vms.map((vm) => ({
      id: `vm-open-${vm.id}`,
      label: vm.name,
      hint: `Virtual machine · ${vm.state} · ${vm.os}`,
      group: "Virtual machines",
      keywords: `vm qemu kvm ${vm.state} ${vm.os}`,
      icon: (
        <EntityIcon
          src={vmIconURL(vm.id)}
          name={vm.name}
          size={20}
          className="rounded-md"
        />
      ),
      run: () => navigate(`/vms?vm=${encodeURIComponent(vm.name)}`),
    }));

    const vmActions: Command[] = snapshot.vms.map((vm) => {
      const running = vm.state === "running";
      return {
        id: `vm-action-${vm.id}`,
        label: running ? `Stop ${vm.name}` : `Start ${vm.name}`,
        hint: `${vm.state} · ${vm.os}`,
        group: "VM actions",
        keywords: `vm qemu kvm ${vm.state} ${vm.os}`,
        icon: running ? <Square className="size-3.5" /> : <Play className="size-3.5" />,
        run: () =>
          action.mutate({
            kind: running ? "vm.stop" : "vm.start",
            target: vm.name,
            success: running ? `${vm.name} is shutting down` : `${vm.name} started`,
          }),
      };
    });

    return [...navigation, ...globals, ...containers, ...containerActions, ...vms, ...vmActions];
  }, [snapshot, navigate, resolvedTheme, setTheme, refresh, action]);

  if (error) return <ErrorScreen error={error} onRetry={() => refresh()} />;
  if (!snapshot) return <BootScreen />;
  if (terminalPopout || isStandaloneConsole) {
    return (
      <div className="h-dvh w-screen overflow-hidden bg-[#08090d]">
        <Outlet />
      </div>
    );
  }

  return (
    <div className="relative min-h-dvh">
      <div className="app-atmosphere" />
      <div className="grain" />

      <Sidebar
        collapsed={collapsed}
        onToggle={() => setCollapsed((value) => !value)}
        onOpenCommand={() => setCommandOpen(true)}
      />

      <MobileSidebar
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        onOpenCommand={() => setCommandOpen(true)}
      />

      <div
        className={
          collapsed
            ? "relative z-10 flex min-h-dvh flex-col transition-[padding] duration-300 lg:pl-[82px]"
            : "relative z-10 flex min-h-dvh flex-col transition-[padding] duration-300 lg:pl-[264px]"
        }
      >
        <Topbar onOpenCommand={() => setCommandOpen(true)} onOpenMobile={() => setMobileOpen(true)} />

        <main className={isTerminal ? "flex min-h-0 w-full flex-1 flex-col" : "w-full flex-1 px-4 pb-16 pt-6 sm:px-6 lg:px-8 lg:pt-8"}>
          <div key={location.pathname} className={isTerminal ? "flex min-h-0 flex-1 flex-col" : "animate-fade-up"}>
            <Outlet />
          </div>
        </main>

        {!isTerminal ? <footer className="flex w-full flex-wrap items-center justify-between gap-2 border-t border-line px-4 py-5 text-[10.5px] text-subtle sm:px-6 lg:px-8">
          <span>
            Aurora · an independent, modern control plane for Unraid. Session {session?.user ?? "root"}.
          </span>
          <span className="flex items-center gap-3">
            <span className="flex items-center gap-1">
              <span className="status-dot live" /> Unraid {snapshot.system.version}
            </span>
            <span className="h-3 w-px bg-[var(--line)]" aria-hidden="true" />
            <span className="font-mono">Powered by 🌱 DJΞRFY 🚀</span>
          </span>
        </footer> : null}
      </div>

      <CommandPalette open={commandOpen} onClose={() => setCommandOpen(false)} commands={commands} />
    </div>
  );
}

export { BootScreen, ErrorScreen };
