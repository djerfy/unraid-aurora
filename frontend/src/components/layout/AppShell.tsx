import { useCallback, useEffect, useMemo, useState } from "react";
import { Outlet, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import {
  Boxes,
  Bug,
  MonitorPlay,
  Package,
  Play,
  RefreshCw,
  RotateCw,
  Square,
  Sun,
  Moon,
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { Sidebar, MobileSidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { CommandPalette, Modal, type Command } from "../ui/overlays";
import { Button } from "../ui/primitives";
import { useApp } from "../../app/AppDataProvider";
import { useTheme } from "../../app/ThemeProvider";
import { useAction } from "../../lib/useAction";
import { useHotkeys } from "../../lib/hooks";
import { api } from "../../lib/api";
import { NAV_ITEMS } from "./nav";
import { EntityIcon, containerIconURL, vmIconURL } from "../ui/entity-icon";
import { AuroraWordmark } from "./Brand";
import { useI18n, type TranslationParams } from "../../i18n";
import type {
  ContainerVulnerability,
  SecurityContainerRef,
  SecurityImage,
  SecurityOverview,
} from "../../lib/types";

// Shortcuts must not steal keystrokes from editors, terminals or consoles.
function isTypingTarget(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null;
  if (!element) return false;
  return ["INPUT", "TEXTAREA", "SELECT", "CANVAS"].includes(element.tagName) || element.isContentEditable;
}

const CVE_QUERY = /^cve-/i;

type Translate = (key: string, params?: TranslationParams) => string;
type Navigate = (to: string) => void;

type CveFinding = { image: SecurityImage; vulnerability: ContainerVulnerability };

type CveContainerHit = {
  ref: SecurityContainerRef;
  image: string;
  cves: Set<string>;
  packages: Set<string>;
  severity: string;
};

type CvePackageHit = {
  name: string;
  installed: string;
  fixed: string;
  severity: string;
  cves: Set<string>;
  containers: Set<string>;
};

function flattenCveFindings(overview?: SecurityOverview): CveFinding[] {
  if (!overview?.enabled) return [];
  const findings: CveFinding[] = [];
  for (const image of overview.images ?? []) {
    for (const vulnerability of image.vulnerabilities ?? []) {
      findings.push({ image, vulnerability });
    }
  }
  return findings;
}

function severityRank(value: string): number {
  switch (value.toUpperCase()) {
    case "CRITICAL":
      return 5;
    case "HIGH":
      return 4;
    case "MEDIUM":
      return 3;
    case "LOW":
      return 2;
    default:
      return 1;
  }
}

// Built from the same merged Trivy overview as the Docker Security tab.
function buildCveCommands(findings: CveFinding[], query: string, t: Translate, navigate: Navigate): Command[] {
  const needle = query.toLowerCase();
  const containers = new Map<string, CveContainerHit>();
  const packages = new Map<string, CvePackageHit>();
  for (const { image, vulnerability } of findings) {
    if (!vulnerability.id.toLowerCase().includes(needle)) continue;
    for (const container of image.containers ?? []) {
      let hit = containers.get(container.id);
      if (!hit) {
        hit = { ref: container, image: image.image, cves: new Set(), packages: new Set(), severity: "UNKNOWN" };
        containers.set(container.id, hit);
      }
      hit.cves.add(vulnerability.id);
      hit.packages.add(vulnerability.package);
      if (severityRank(vulnerability.severity) > severityRank(hit.severity)) hit.severity = vulnerability.severity;
    }
    const key = `${vulnerability.package}@${vulnerability.installedVersion}@${vulnerability.fixedVersion}`;
    let pkg = packages.get(key);
    if (!pkg) {
      pkg = {
        name: vulnerability.package,
        installed: vulnerability.installedVersion,
        fixed: vulnerability.fixedVersion,
        severity: "UNKNOWN",
        cves: new Set(),
        containers: new Set(),
      };
      packages.set(key, pkg);
    }
    pkg.cves.add(vulnerability.id);
    for (const container of image.containers ?? []) pkg.containers.add(container.name);
    if (severityRank(vulnerability.severity) > severityRank(pkg.severity)) pkg.severity = vulnerability.severity;
  }

  const containerRows = [...containers.values()]
    .sort((left, right) => severityRank(right.severity) - severityRank(left.severity) || left.ref.name.localeCompare(right.ref.name))
    .slice(0, 20);
  const packageRows = [...packages.values()]
    .sort((left, right) => severityRank(right.severity) - severityRank(left.severity) || left.name.localeCompare(right.name))
    .slice(0, 20);
  const base = `/docker?tab=security&securityQuery=${encodeURIComponent(query)}`;
  return [
    ...containerRows.map((hit) => ({
      id: `cve-container-${hit.ref.id}`,
      label: hit.ref.name,
      hint: `${hit.image} · ${t(hit.cves.size === 1 ? "{count} CVE" : "{count} CVEs", { count: hit.cves.size })}`,
      group: t("Affected containers"),
      keywords: `${query} cve container ${hit.ref.name} ${hit.image} ${[...hit.cves].join(" ")} ${[...hit.packages].join(" ")}`,
      icon: <Boxes className="size-3.5" />,
      run: () => navigate(`${base}&securityGroup=container`),
    })),
    ...packageRows.map((hit, index) => ({
      id: `cve-package-${index}-${hit.name}-${hit.installed}`,
      label: hit.name,
      hint: `v${hit.installed}${hit.fixed ? ` → ${hit.fixed}` : ""} · ${hit.severity} · ${t(hit.containers.size === 1 ? "{count} container" : "{count} containers", { count: hit.containers.size })}`,
      group: t("Affected packages"),
      keywords: `${query} cve package ${hit.name} ${hit.installed} ${[...hit.cves].join(" ")}`,
      icon: <Package className="size-3.5" />,
      run: () => navigate(`${base}&securityGroup=package`),
    })),
  ];
}

function BootScreen({ message }: { message?: string }) {
  const { t } = useI18n();
  return (
    <div className="relative grid min-h-dvh place-items-center bg-[var(--bg)]">
      <div className="app-atmosphere" />
      <div className="relative flex flex-col items-center">
        <AuroraWordmark />
        <div className="mt-8 flex items-center gap-2 text-[11px] text-subtle">
          <span className="status-dot live" />
          {message ? t(message) : t("Connecting to your server")}
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
  const { t } = useI18n();
  return (
    <div className="relative grid min-h-dvh place-items-center bg-[var(--bg)] px-6">
      <div className="app-atmosphere" />
      <div className="card relative w-full max-w-md p-8 text-center">
        <p className="eyebrow text-[var(--danger)]">{t("Connection lost")}</p>
        <h1 className="mt-3 text-xl font-semibold text-ink">{t("Aurora cannot reach the server")}</h1>
        <p className="mt-2 text-xs leading-relaxed text-muted">{error.message}</p>
        <div className="mt-6 flex justify-center gap-2">
          <Button variant="primary" onClick={onRetry}>{t("Try again")}</Button>
          <Button variant="secondary" onClick={() => navigate("/login")}>{t("Sign in")}</Button>
        </div>
      </div>
    </div>
  );
}

export function AppShell() {
  const { snapshot, session, error, live, refresh } = useApp();
  const { resolvedTheme, setTheme, setPreferences, wallpaper } = useTheme();
  const { t } = useI18n();
  const action = useAction();
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [showOffline, setShowOffline] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const isTerminal = location.pathname === "/terminal" || location.pathname.startsWith("/docker/terminal/");
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
  const [paletteQuery, setPaletteQuery] = useState("");

  const securityOverview = useQuery({
    queryKey: ["security-overview"],
    queryFn: api.securityOverview,
    enabled: commandOpen && CVE_QUERY.test(paletteQuery.trim()),
    staleTime: 30_000,
    retry: false,
  });
  const cveFindings = useMemo(() => flattenCveFindings(securityOverview.data), [securityOverview.data]);

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
        "?": (event: KeyboardEvent) => {
          if (isTypingTarget(event.target)) return;
          event.preventDefault();
          setShortcutsOpen(true);
        },
      }),
      [],
    ),
  );

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [location.pathname]);

  useEffect(() => {
    if (live) {
      setShowOffline(false);
      return;
    }
    const timer = window.setTimeout(() => setShowOffline(true), 5000);
    return () => window.clearTimeout(timer);
  }, [live]);

  // A browser that has never chosen a theme adopts the server preference once.
  useEffect(() => {
    if (!snapshot) return;
    try {
      if (localStorage.getItem("aurora.theme")) return;
      setPreferences({
        theme: snapshot.settings.theme as "dark" | "light" | "system",
        accent: snapshot.settings.accent as "ember" | "violet" | "cyan" | "emerald" | "amber" | "blue",
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
      label: t(item.label),
      hint: t("Go to {page}", { page: t(item.label) }),
      group: t("Navigation"),
      icon: <item.icon className="size-3.5" />,
      run: () => navigate(item.to),
    }));

    const globals: Command[] = [
      {
        id: "theme",
        label: resolvedTheme === "dark" ? t("Switch to light theme") : t("Switch to dark theme"),
        hint: t("Appearance"),
        group: t("Actions"),
        icon: resolvedTheme === "dark" ? <Sun className="size-3.5" /> : <Moon className="size-3.5" />,
        run: () => setTheme(resolvedTheme === "dark" ? "light" : "dark"),
      },
      {
        id: "refresh",
        label: t("Refresh server data"),
        hint: t("Re-sync all widgets"),
        group: t("Actions"),
        icon: <RefreshCw className="size-3.5" />,
        run: () => refresh(),
      },
      {
        id: "new-container",
        label: t("Open containers"),
        hint: t("{count} running", { count: snapshot.containers.filter((c) => c.state === "running").length }),
        group: t("Actions"),
        icon: <Boxes className="size-3.5" />,
        run: () => navigate("/docker"),
      },
      {
        id: "new-vm",
        label: t("Open virtual machines"),
        hint: t("{count} running", { count: snapshot.vms.filter((v) => v.state === "running").length }),
        group: t("Actions"),
        icon: <MonitorPlay className="size-3.5" />,
        run: () => navigate("/vms"),
      },
    ];

    const containers: Command[] = snapshot.containers.map((container) => ({
      id: `ct-open-${container.id}`,
      label: container.name,
      hint: t("Container · {state} · {image}", { state: container.state, image: container.image }),
      group: t("Containers"),
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
        label: running ? t("Restart {name}", { name: container.name }) : t("Start {name}", { name: container.name }),
        hint: `${container.state} · ${container.image}`,
        group: t("Container actions"),
        keywords: `docker container ${container.state} ${container.image}`,
        icon: running ? <RotateCw className="size-3.5" /> : <Play className="size-3.5" />,
        run: () =>
          action.mutate({
            kind: running ? "container.restart" : "container.start",
            target: container.name,
            success: running ? t("{name} restarted", { name: container.name }) : t("{name} started", { name: container.name }),
          }),
      };
    });

    const vms: Command[] = snapshot.vms.map((vm) => ({
      id: `vm-open-${vm.id}`,
      label: vm.name,
      hint: t("Virtual machine · {state} · {os}", { state: vm.state, os: vm.os }),
      group: t("Virtual machines"),
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
        label: running ? t("Stop {name}", { name: vm.name }) : t("Start {name}", { name: vm.name }),
        hint: `${vm.state} · ${vm.os}`,
        group: t("VM actions"),
        keywords: `vm qemu kvm ${vm.state} ${vm.os}`,
        icon: running ? <Square className="size-3.5" /> : <Play className="size-3.5" />,
        run: () =>
          action.mutate({
            kind: running ? "vm.stop" : "vm.start",
            target: vm.name,
            success: running ? t("{name} is shutting down", { name: vm.name }) : t("{name} started", { name: vm.name }),
          }),
      };
    });

    return [...navigation, ...globals, ...containers, ...containerActions, ...vms, ...vmActions];
  }, [snapshot, navigate, resolvedTheme, setTheme, refresh, action, t]);

  const resolveExtra = useCallback(
    (current: string) => {
      if (!commandOpen) return [];
      const query = current.trim();
      if (!CVE_QUERY.test(query)) return [];
      const base = `/docker?tab=security&securityQuery=${encodeURIComponent(query)}`;
      if (securityOverview.isPending) {
        return [{
          id: "cve-loading",
          label: t("Searching Trivy findings…"),
          hint: query,
          group: t("CVE search"),
          keywords: query,
          icon: <Bug className="size-3.5" />,
          run: () => navigate(base),
        }];
      }
      if (securityOverview.isError) {
        return [{
          id: "cve-error",
          label: t("Could not load CVE data"),
          hint: (securityOverview.error as Error).message,
          group: t("CVE search"),
          keywords: query,
          icon: <Bug className="size-3.5" />,
          run: () => navigate(base),
        }];
      }
      if (securityOverview.data && !securityOverview.data.enabled) {
        return [{
          id: "cve-disabled",
          label: t("Image scanning is disabled"),
          hint: t("Enable Trivy in Settings → Image scanning"),
          group: t("CVE search"),
          keywords: query,
          icon: <Bug className="size-3.5" />,
          run: () => navigate("/settings"),
        }];
      }
      return buildCveCommands(cveFindings, query, t, navigate);
    },
    [commandOpen, cveFindings, navigate, securityOverview.data, securityOverview.error, securityOverview.isError, securityOverview.isPending, t],
  );

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
      <a href="#main-content" className="skip-link">{t("Skip to content")}</a>
      {wallpaper ? (
        <div
          className="app-wallpaper"
          style={{ backgroundImage: `url("${wallpaper}")` }}
          aria-hidden="true"
        />
      ) : null}
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

        {showOffline ? (
          <div role="status" aria-live="polite" className="relative z-10 border-b border-[var(--warning)]/30 bg-[var(--warning-soft)] px-4 py-2 text-center text-[10px] text-[var(--warning)]">
            {t("Live metrics are unavailable. Showing the last snapshot and reconnecting automatically.")}
          </div>
        ) : null}

        <main id="main-content" className={isTerminal ? "flex min-h-0 w-full flex-1 flex-col" : "w-full flex-1 px-4 pb-16 pt-6 sm:px-6 lg:px-8 lg:pt-8"}>
          <div key={location.pathname} className={isTerminal ? "flex min-h-0 flex-1 flex-col" : "animate-fade-up"}>
            <Outlet />
          </div>
        </main>

        {!isTerminal ? <footer className="flex w-full flex-wrap items-center justify-between gap-2 border-t border-line px-4 py-5 text-[10.5px] text-subtle sm:px-6 lg:px-8">
          <span>
            {t("Aurora · an independent, modern control plane for Unraid. Session {user}.", {
              user: session?.user ?? "root",
            })}
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

      <CommandPalette
        open={commandOpen}
        onClose={() => setCommandOpen(false)}
        commands={commands}
        resolveExtra={resolveExtra}
        onQueryChange={setPaletteQuery}
      />

      <Modal
        open={shortcutsOpen}
        onClose={() => setShortcutsOpen(false)}
        title={t("Keyboard shortcuts")}
        description={t("Navigate Aurora without leaving the keyboard.")}
        footer={<Button variant="secondary" onClick={() => setShortcutsOpen(false)}>{t("Close")}</Button>}
      >
        <div className="space-y-2">
          {([
            ["⌘K / Ctrl K", "Open the command palette"],
            ["/", "Focus search"],
            ["?", "Show this help"],
            ["Esc", "Close dialogs"],
          ] as const).map(([keys, label]) => (
            <div key={keys} className="flex items-center justify-between rounded-lg border border-line bg-[var(--surface-2)]/45 px-3 py-2 text-[11px]">
              <span className="text-muted">{t(label)}</span>
              <kbd className="rounded-md border border-line bg-[var(--surface)] px-2 py-0.5 font-mono text-[9.5px] text-soft">{keys}</kbd>
            </div>
          ))}
        </div>
      </Modal>
    </div>
  );
}

export { BootScreen, ErrorScreen };
