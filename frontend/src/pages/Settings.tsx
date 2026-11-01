import { useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Bell,
  Check,
  Cpu,
  ExternalLink,
  Info,
  KeyRound,
  LogOut,
  Monitor,
  Moon,
  Palette,
  RotateCcw,
  Save,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  Sun,
} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { PageHeader } from "../components/layout/PageHeader";
import {
  Badge,
  Button,
  Card,
  Switch,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useTheme, type Accent, type ThemeMode } from "../app/ThemeProvider";
import { useToast } from "../app/ToastProvider";
import { api } from "../lib/api";
import { formatMemory, formatUptime } from "../lib/format";
import type { Settings } from "../lib/types";

const SECTIONS = [
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "general", label: "General", icon: SlidersHorizontal },
  { id: "notifications", label: "Notifications", icon: Bell },
  { id: "security", label: "Security", icon: ShieldCheck },
  { id: "about", label: "About", icon: Info },
];

const ACCENTS: { value: Accent; label: string; color: string; secondary: string }[] = [
  { value: "ember", label: "Ember", color: "#ff6b4a", secondary: "#ff3d67" },
  { value: "violet", label: "Violet", color: "#8b6bff", secondary: "#c66bff" },
  { value: "cyan", label: "Cyan", color: "#22b8d6", secondary: "#4de3c0" },
  { value: "emerald", label: "Emerald", color: "#22b573", secondary: "#7ee2a8" },
  { value: "amber", label: "Amber", color: "#e8a020", secondary: "#ffd066" },
];

export function SettingsPage() {
  const { snapshot, session, refresh } = useApp();
  const theme = useTheme();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [section, setSection] = useState("appearance");
  const [draft, setDraft] = useState<Settings | null>(snapshot?.settings ?? null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  if (!snapshot) return null;
  const settings = draft ?? snapshot.settings;
  const dirty = JSON.stringify(settings) !== JSON.stringify(snapshot.settings);

  const update = <K extends keyof Settings>(key: K, value: Settings[K]) => {
    setDraft({ ...settings, [key]: value });
    setSaved(false);
  };

  const chooseTheme = (mode: ThemeMode) => {
    update("theme", mode);
    theme.setTheme(mode);
  };

  const chooseAccent = (accent: Accent) => {
    update("accent", accent);
    theme.setAccent(accent);
  };

  const save = async () => {
    setSaving(true);
    try {
      await api.saveSettings(settings);
      await queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      setSaved(true);
      toast({ kind: "success", title: "Settings saved", description: "Your preferences are now active." });
      window.setTimeout(() => setSaved(false), 2400);
    } catch (error) {
      toast({ kind: "error", title: "Could not save settings", description: (error as Error).message });
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    setDraft(snapshot.settings);
    theme.setTheme(snapshot.settings.theme as ThemeMode);
    theme.setAccent(snapshot.settings.accent as Accent);
    toast({ kind: "info", title: "Changes discarded" });
  };

  const signOut = async () => {
    await api.logout();
    await queryClient.invalidateQueries({ queryKey: ["session"] });
    queryClient.removeQueries({ queryKey: ["bootstrap"] });
    navigate("/login");
  };

  return (
    <>
      <PageHeader
        eyebrow="System"
        title="Settings"
        description="Make Aurora yours, tune how often it syncs, and review the connection to this server."
        meta={
          <>
            <Badge tone="neutral">Unraid {snapshot.system.version}</Badge>
            <Badge tone="success" dot>Authenticated</Badge>
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RotateCcw className="size-3.5" />} onClick={reset} disabled={!dirty}>
              Reset
            </Button>
            <Button
              variant="primary"
              icon={saved ? <Check className="size-3.5" /> : <Save className="size-3.5" />}
              loading={saving}
              disabled={!dirty || saving}
              onClick={() => void save()}
            >
              {saved ? "Saved" : "Save changes"}
            </Button>
          </>
        }
      />

      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[210px_minmax(0,1fr)]">
        <aside className="lg:sticky lg:top-24 lg:self-start">
          <nav className="flex gap-1 overflow-x-auto rounded-xl border border-line bg-[var(--surface-2)]/60 p-1.5 lg:flex-col">
            {SECTIONS.map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSection(item.id)}
                  className={cn(
                    "flex shrink-0 items-center gap-2.5 rounded-lg px-3 py-2 text-left text-[12px] font-medium transition",
                    section === item.id
                      ? "bg-[var(--surface)] text-ink shadow-sm ring-1 ring-line"
                      : "text-muted hover:text-ink",
                  )}
                >
                  <Icon className={cn("size-4", section === item.id ? "text-[var(--accent)]" : "text-subtle")} />
                  {item.label}
                </button>
              );
            })}
          </nav>
          <div className="mt-3 hidden rounded-xl border border-line px-3 py-3 lg:block">
            <p className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">Connected to</p>
            <p className="mt-1.5 text-[12px] font-medium text-ink">{snapshot.system.name}</p>
            <p className="font-mono text-[9.5px] text-subtle">{snapshot.system.hostname} · {snapshot.system.version}</p>
            <Button size="sm" variant="ghost" className="mt-2 h-7 px-2 text-[10px]" onClick={refresh}>
              Re-sync now
            </Button>
          </div>
        </aside>

        <div className="space-y-4">
          {section === "appearance" ? (
            <>
              <Card className="card-pad">
                <CardTitle title="Theme" description="Aurora adapts every surface, chart and control to your preference." />
                <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
                  {([
                    { value: "dark", label: "Midnight", icon: Moon, hint: "Deep graphite" },
                    { value: "light", label: "Daylight", icon: Sun, hint: "Warm porcelain" },
                    { value: "system", label: "Automatic", icon: Monitor, hint: "Follows the OS" },
                  ] as const).map((option) => {
                    const Icon = option.icon;
                    const active = settings.theme === option.value;
                    return (
                      <button
                        key={option.value}
                        type="button"
                        onClick={() => chooseTheme(option.value)}
                        className={cn(
                          "relative overflow-hidden rounded-xl border p-3 text-left transition",
                          active
                            ? "border-[var(--accent-line)] bg-[var(--accent-softer)] ring-1 ring-[var(--accent-line)]"
                            : "border-line bg-[var(--surface-2)]/50 hover:border-line-strong",
                        )}
                      >
                        <div
                          className={cn(
                            "mb-3 h-16 rounded-lg border p-2",
                            option.value === "light"
                              ? "border-black/10 bg-[#f4f4f6]"
                              : option.value === "dark"
                                ? "border-white/10 bg-[#0a0b10]"
                                : "border-white/10 bg-[linear-gradient(110deg,#0a0b10_50%,#f4f4f6_50%)]",
                          )}
                        >
                          <div className="flex gap-1">
                            <span className="h-1.5 w-8 rounded-full bg-[#ff6b4a]" />
                            <span className={cn("h-1.5 w-12 rounded-full", option.value === "light" ? "bg-black/15" : "bg-white/15")} />
                          </div>
                          <div className="mt-2 flex gap-1">
                            <span className={cn("h-6 flex-1 rounded", option.value === "light" ? "bg-black/[0.06]" : "bg-white/[0.06]")} />
                            <span className={cn("h-6 w-5 rounded", option.value === "light" ? "bg-black/[0.06]" : "bg-white/[0.06]")} />
                          </div>
                        </div>
                        <div className="flex items-center gap-1.5">
                          <Icon className="size-3.5 text-muted" />
                          <span className="text-[12px] font-medium text-ink">{option.label}</span>
                          {active ? <Check className="ml-auto size-3.5 text-[var(--accent)]" /> : null}
                        </div>
                        <p className="mt-0.5 text-[10px] text-subtle">{option.hint}</p>
                      </button>
                    );
                  })}
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title="Accent" description="One colour drives highlights, charts and focus states." />
                <div className="mt-4 flex flex-wrap gap-2.5">
                  {ACCENTS.map((accent) => (
                    <button
                      key={accent.value}
                      type="button"
                      onClick={() => chooseAccent(accent.value)}
                      className={cn(
                        "flex items-center gap-2 rounded-xl border px-3 py-2 transition",
                        settings.accent === accent.value
                          ? "border-[var(--accent-line)] bg-[var(--accent-softer)]"
                          : "border-line bg-[var(--surface-2)]/50 hover:border-line-strong",
                      )}
                    >
                      <span
                        className="size-4 rounded-full border border-white/20"
                        style={{ background: `linear-gradient(135deg, ${accent.color}, ${accent.secondary})` }}
                      />
                      <span className="text-[11.5px] font-medium text-soft">{accent.label}</span>
                      {settings.accent === accent.value ? <Check className="size-3 text-[var(--accent)]" /> : null}
                    </button>
                  ))}
                </div>

                <div className="my-5 h-px bg-line" />

                <SettingsRow label="Density" description="Compact reduces padding and row height across the interface.">
                  <div className="flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
                    {["comfortable", "compact"].map((value) => (
                      <button
                        key={value}
                        type="button"
                        onClick={() => {
                          update("density", value);
                          theme.setDensity(value as "comfortable" | "compact");
                        }}
                        className={cn(
                          "rounded-md px-3 py-1 text-[11px] font-medium capitalize transition",
                          settings.density === value ? "bg-[var(--surface)] text-ink shadow-sm" : "text-muted hover:text-ink",
                        )}
                      >
                        {value}
                      </button>
                    ))}
                  </div>
                </SettingsRow>

                <SettingsRow label="Temperature unit" description="Used for disks, CPU and sensors.">
                  <Select
                    ariaLabel="Temperature unit"
                    value={settings.temperatureUnit}
                    onChange={(value) => update("temperatureUnit", value)}
                    options={[
                      { value: "celsius", label: "Celsius" },
                      { value: "fahrenheit", label: "Fahrenheit" },
                    ]}
                  />
                </SettingsRow>
              </Card>
            </>
          ) : null}

          {section === "general" ? (
            <Card className="card-pad">
              <CardTitle title="General" description="Server identity, refresh cadence and language." />
              <div className="mt-2 divide-y divide-line">
                <SettingsRow label="Server display name" description="Shown in the sidebar and page titles.">
                  <input
                    value={settings.serverName}
                    onChange={(event) => update("serverName", event.target.value)}
                    className="h-9 w-52 rounded-[10px] border border-line bg-[var(--surface-2)] px-3 text-[12px] text-ink outline-none transition focus:border-[var(--accent-line)]"
                  />
                </SettingsRow>
                <SettingsRow label="Refresh interval" description="How often widgets re-sync with the server.">
                  <Select
                    ariaLabel="Refresh interval"
                    value={String(settings.refreshInterval)}
                    onChange={(value) => update("refreshInterval", Number(value))}
                    options={[2, 5, 10, 30, 60].map((value) => ({ value: String(value), label: `${value} seconds` }))}
                  />
                </SettingsRow>
                <SettingsRow label="Language" description="Interface language for Aurora.">
                  <Select
                    ariaLabel="Language"
                    value={settings.language}
                    onChange={(value) => update("language", value)}
                    options={[
                      { value: "en", label: "English" },
                      { value: "fr", label: "Français" },
                      { value: "de", label: "Deutsch" },
                    ]}
                  />
                </SettingsRow>
                <SettingsRow label="Automatic updates" description="Allow Aurora to surface and apply plugin updates.">
                  <Switch checked={settings.autoUpdate} onChange={(value) => update("autoUpdate", value)} label="Automatic updates" />
                </SettingsRow>
                <SettingsRow label="Anonymous telemetry" description="Share crash reports and feature usage. Off by default.">
                  <Switch checked={settings.telemetry} onChange={(value) => update("telemetry", value)} label="Anonymous telemetry" />
                </SettingsRow>
              </div>
            </Card>
          ) : null}

          {section === "notifications" ? (
            <Card className="card-pad">
              <CardTitle title="Notifications" description="Choose which channels receive alerts from your server." />
              <div className="mt-2 divide-y divide-line">
                <SettingsRow label="Email alerts" description="Send warnings and alerts to the address configured in Unraid.">
                  <Switch checked={settings.emailAlerts} onChange={(value) => update("emailAlerts", value)} label="Email alerts" />
                </SettingsRow>
                <SettingsRow label="Push notifications" description="Browser and mobile push through Unraid Connect agents.">
                  <Switch checked={settings.pushAlerts} onChange={(value) => update("pushAlerts", value)} label="Push notifications" />
                </SettingsRow>
              </div>
              <div className="mt-3 flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
                <Bell className="mt-0.5 size-3.5 shrink-0 text-subtle" />
                Delivery agents are configured on the Unraid side. Aurora mirrors the settings and shows the
                resulting in-app inbox.
              </div>
            </Card>
          ) : null}

          {section === "security" ? (
            <>
              <Card className="card-pad">
                <CardTitle title="Session" description="How this browser is authenticated against Aurora." />
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <InfoBlock label="Account" value={session?.user ?? "admin"} icon={<KeyRound className="size-3.5" />} />
                  <InfoBlock label="Authentication" value="Native · /etc/shadow" tone="success" />
                  <InfoBlock label="Session store" value="HMAC-SHA256 · HttpOnly" icon={<ShieldCheck className="size-3.5" />} />
                  <InfoBlock label="CSRF protection" value="Custom header + origin check" />
                </div>
                <div className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
                  <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
                  Only the host <code className="code-chip">root</code> account can sign in, matching the Unraid
                  webGUI. Credentials are validated against the local shadow database and are never stored by Aurora.
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title="Access" description="End the current session on this device." />
                <div className="mt-3 flex items-center justify-between gap-4">
                  <p className="text-[11px] text-muted">You will be returned to the sign-in screen.</p>
                  <Button variant="danger" icon={<LogOut className="size-3.5" />} onClick={() => void signOut()}>
                    Sign out
                  </Button>
                </div>
              </Card>
            </>
          ) : null}

          {section === "about" ? (
            <>
              <Card className="card-pad">
                <div className="flex flex-wrap items-center gap-4">
                  <span className="grid size-14 place-items-center rounded-2xl border border-line bg-[var(--surface-2)]">
                    <Server className="size-6 text-[var(--accent)]" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="text-[16px] font-semibold text-ink">Aurora</h2>
                      <Badge tone="accent">v{__APP_VERSION__}</Badge>
                      <Badge tone="success" dot>live host</Badge>
                    </div>
                    <p className="mt-1 max-w-xl text-[11.5px] leading-relaxed text-muted">
                      An independent, opinionated control plane for Unraid. Built in Go with a React front end —
                      designed to make homelab infrastructure feel calm and precise.
                    </p>
                  </div>
                </div>
                <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-3">
                  <InfoBlock label="Unraid" value={snapshot.system.version} />
                  <InfoBlock label="Kernel" value={snapshot.system.kernel} />
                  <InfoBlock label="Uptime" value={formatUptime(snapshot.system.uptime)} />
                  <InfoBlock label="Board" value={snapshot.system.board} />
                  <InfoBlock label="Memory" value={formatMemory(snapshot.system.memory.total)} icon={<Cpu className="size-3.5" />} />
                  <InfoBlock label="License" value={snapshot.system.license || "Unknown"} />
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title="Credits" description="Aurora stands on the shoulders of the Unraid community." />
                <div className="mt-3 space-y-2">
                  {[
                    { label: "Unraid webGUI", href: "https://github.com/unraid/webgui", description: "Original PHP interface and state contracts" },
                    { label: "Unraid documentation", href: "https://docs.unraid.net", description: "Array, parity and Docker behaviour" },
                    { label: "Community Applications", href: "https://ca.unraid.net", description: "Plugin ecosystem" },
                  ].map((link) => (
                    <a
                      key={link.label}
                      href={link.href}
                      target="_blank"
                      rel="noreferrer"
                      className="flex items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5 transition hover:border-line-strong hover:bg-[var(--surface-2)]"
                    >
                      <ExternalLink className="size-3.5 text-subtle" />
                      <span className="min-w-0 flex-1">
                        <span className="block text-[11.5px] font-medium text-soft">{link.label}</span>
                        <span className="block truncate text-[9.5px] text-subtle">{link.description}</span>
                      </span>
                      <span className="text-[10px] text-subtle">↗</span>
                    </a>
                  ))}
                </div>
                <p className="mt-4 border-t border-line pt-3 text-[9.5px] leading-relaxed text-subtle">
                  Aurora is not affiliated with Lime Technology. Unraid is a trademark of Lime Technology, Inc.
                </p>
              </Card>
            </>
          ) : null}
        </div>
      </div>
    </>
  );
}

function CardTitle({ title, description }: { title: string; description: string }) {
  return (
    <div>
      <h2 className="text-[13px] font-semibold text-ink">{title}</h2>
      <p className="mt-0.5 text-[10.5px] text-muted">{description}</p>
    </div>
  );
}

function SettingsRow({ label, description, children }: { label: string; description: string; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 py-3.5 first:pt-3 last:pb-0">
      <div className="min-w-0 max-w-md">
        <p className="text-[11.5px] font-medium text-soft">{label}</p>
        <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">{description}</p>
      </div>
      {children}
    </div>
  );
}

function Select({
  value,
  onChange,
  options,
  ariaLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  ariaLabel: string;
}) {
  return (
    <select
      value={value}
      aria-label={ariaLabel}
      onChange={(event) => onChange(event.target.value)}
      className="h-9 rounded-[10px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-soft outline-none transition focus:border-[var(--accent-line)]"
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
}

function InfoBlock({
  label,
  value,
  icon,
  tone,
}: {
  label: string;
  value: string;
  icon?: ReactNode;
  tone?: "success" | "warning";
}) {
  return (
    <div className="rounded-xl border border-line bg-[var(--surface-2)]/50 px-3 py-2.5">
      <p className="flex items-center gap-1 text-[9px] uppercase tracking-[0.1em] text-subtle">
        {icon}
        {label}
      </p>
      <p className={cn("mt-1 truncate text-[11px] font-medium", tone === "warning" ? "text-[var(--warning)]" : tone === "success" ? "text-[var(--success)]" : "text-soft")}>
        {value}
      </p>
    </div>
  );
}
