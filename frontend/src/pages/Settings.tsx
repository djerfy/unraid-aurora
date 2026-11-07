import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bell,
  Activity,
  Bot,
  Check,
  Container,
  Cpu,
  Download,
  ExternalLink,
  Globe,
  Info,
  KeyRound,
  LogOut,
  BatteryCharging,
  Monitor,
  Moon,
  Palette,
  Plus,
  RotateCcw,
  Save,
  ScanSearch,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  Upload,
  Sun,
  Trash2,
} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { PageHeader } from "../components/layout/PageHeader";
import {
  Badge,
  Button,
  Card,
  IconButton,
  Switch,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useTheme, type Accent, type ThemeMode } from "../app/ThemeProvider";
import { useToast } from "../app/ToastProvider";
import { useI18n } from "../i18n";
import { UPSView } from "./UPS";
import { OSUpdatePanel } from "../components/system/OSUpdatePanel";
import { UnraidAPIPanel } from "../components/system/UnraidAPIPanel";
import { TrivyPanel } from "../components/system/TrivyPanel";
import { DockerSettingsPanel } from "../components/system/DockerSettingsPanel";
import { VMSettingsPanel } from "../components/system/VMSettingsPanel";
import { ProxySettingsPanel } from "../components/system/ProxySettingsPanel";
import { NvidiaDriverPanel } from "../components/system/NvidiaDriverPanel";
import { api } from "../lib/api";
import { formatDateTime, formatMemory, formatUptime } from "../lib/format";
import type { Settings } from "../lib/types";

const SECTIONS = [
  { id: "general", label: "General", icon: SlidersHorizontal },
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "notifications", label: "Notifications", icon: Bell },
  { id: "trivy", label: "Image scanning", icon: ScanSearch },
  { id: "ups", label: "UPS", icon: BatteryCharging },
  { id: "os", label: "Unraid OS", icon: Download },
  { id: "unraid-api", label: "Unraid API", icon: Activity },
  { id: "docker", label: "Docker", icon: Container },
  { id: "vm", label: "VM Manager", icon: Monitor },
  { id: "proxy", label: "Outbound proxy", icon: Globe },
  { id: "nvidia", label: "NVIDIA driver", icon: Cpu },
  { id: "api", label: "API tokens", icon: KeyRound },
  { id: "mcp", label: "MCP", icon: Bot },
  { id: "security", label: "Security", icon: ShieldCheck },
  { id: "about", label: "About", icon: Info },
];

const ACCENTS: { value: Accent; label: string; color: string; secondary: string }[] = [
  { value: "ember", label: "Ember", color: "#ff6b4a", secondary: "#ff3d67" },
  { value: "violet", label: "Violet", color: "#8b6bff", secondary: "#c66bff" },
  { value: "cyan", label: "Cyan", color: "#22b8d6", secondary: "#4de3c0" },
  { value: "emerald", label: "Emerald", color: "#22b573", secondary: "#7ee2a8" },
  { value: "amber", label: "Amber", color: "#e8a020", secondary: "#ffd066" },
  { value: "blue", label: "Blue", color: "#3b82f6", secondary: "#60a5fa" },
];

function readAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(typeof reader.result === "string" ? reader.result : "");
    reader.onerror = () => reject(reader.error ?? new Error("read failed"));
    reader.readAsDataURL(file);
  });
}

async function wallpaperDataURL(file: File): Promise<string> {
  const source = await readAsDataURL(file);
  if (!source) throw new Error("empty image");
  const image = await new Promise<HTMLImageElement>((resolve, reject) => {
    const element = new Image();
    element.onload = () => resolve(element);
    element.onerror = () => reject(new Error("decode failed"));
    element.src = source;
  });
  const maxDimension = 2560;
  const scale = Math.min(1, maxDimension / Math.max(image.naturalWidth, image.naturalHeight));
  if (scale === 1 && source.length <= 2_500_000) return source;
  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
  canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
  const context = canvas.getContext("2d");
  if (!context) return source;
  context.drawImage(image, 0, 0, canvas.width, canvas.height);
  return canvas.toDataURL("image/jpeg", 0.85);
}

export function SettingsPage() {
  const { snapshot, session, refresh } = useApp();
  const theme = useTheme();
  const { toast } = useToast();
  const { t, setLocale } = useI18n();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [section, setSection] = useState("general");
  const [draft, setDraft] = useState<Settings | null>(snapshot?.settings ?? null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [testingNotification, setTestingNotification] = useState(false);
  const [tokenName, setTokenName] = useState("");
  const [tokenRole, setTokenRole] = useState<"viewer" | "operator" | "admin">("viewer");
  const [tokenExpiry, setTokenExpiry] = useState(90);
  const [tokenScopes, setTokenScopes] = useState<string[]>([]);
  const [newToken, setNewToken] = useState("");
  const [wallpaperBusy, setWallpaperBusy] = useState(false);

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

  const applyWallpaper = async (file: File) => {
    if (!["image/png", "image/jpeg", "image/webp"].includes(file.type)) {
      toast({ kind: "error", title: t("Unsupported image"), description: t("Choose a PNG, JPEG or WebP image.") });
      return;
    }
    if (file.size > 8 * 1024 * 1024) {
      toast({ kind: "error", title: t("Image too large"), description: t("Choose a file under 8 MB.") });
      return;
    }
    setWallpaperBusy(true);
    try {
      theme.setWallpaper(await wallpaperDataURL(file));
      toast({ kind: "success", title: t("Wallpaper updated"), description: t("The image is stored on this device.") });
    } catch {
      toast({ kind: "error", title: t("Could not load the wallpaper"), description: t("Choose a valid image file.") });
    } finally {
      setWallpaperBusy(false);
    }
  };

  const testNotification = async () => {
    setTestingNotification(true);
    try {
      await api.testNotification(settings);
      toast({ kind: "success", title: t("Test notification sent"), description: t("Check the configured channels.") });
    } catch (error) {
      toast({ kind: "error", title: t("Test notification failed"), description: (error as Error).message });
    } finally {
      setTestingNotification(false);
    }
  };

  const tokens = useQuery({ queryKey: ["api-tokens"], queryFn: api.apiTokens, enabled: section === "api" });
  const authSessions = useQuery({ queryKey: ["auth-sessions"], queryFn: api.authSessions, enabled: section === "security", refetchInterval: 30_000 });
  const createToken = useMutation({
    mutationFn: () => api.createAPIToken(tokenName.trim(), tokenRole, tokenExpiry, tokenScopes),
    onSuccess: (result) => {
      setNewToken(result.secret);
      setTokenName("");
      void queryClient.invalidateQueries({ queryKey: ["api-tokens"] });
      toast({ kind: "success", title: t("API token created") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not create the API token"), description: error.message }),
  });
  const revokeToken = useMutation({
    mutationFn: (id: string) => api.revokeAPIToken(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["api-tokens"] });
      toast({ kind: "success", title: t("API token revoked") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not revoke the API token"), description: error.message }),
  });

  const save = async () => {
    setSaving(true);
    try {
      await api.saveSettings(settings);
      await queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["trivy-status"] });
      setSaved(true);
      toast({ kind: "success", title: t("Settings saved"), description: t("Your preferences are now active.") });
      window.setTimeout(() => setSaved(false), 2400);
    } catch (error) {
      toast({ kind: "error", title: t("Could not save settings"), description: (error as Error).message });
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    setDraft(snapshot.settings);
    theme.setTheme(snapshot.settings.theme as ThemeMode);
    theme.setAccent(snapshot.settings.accent as Accent);
    setLocale(snapshot.settings.language === "fr" ? "fr" : "en");
    toast({ kind: "info", title: t("Changes discarded") });
  };

  const signOut = async () => {
    await api.logout();
    await queryClient.invalidateQueries({ queryKey: ["session"] });
    queryClient.removeQueries({ queryKey: ["bootstrap"] });
    navigate("/login");
  };

  const revokeSession = useMutation({
    mutationFn: (nonce: string) => api.revokeSession(nonce),
    onSuccess: async (_result, nonce) => {
      void queryClient.invalidateQueries({ queryKey: ["auth-sessions"] });
      if ((authSessions.data ?? []).some((item) => item.nonce === nonce && item.current)) {
        await signOut();
        return;
      }
      toast({ kind: "success", title: t("Session revoked") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not revoke the session"), description: error.message }),
  });

  const revokeAll = useMutation({
    mutationFn: api.revokeAllSessions,
    onSuccess: async () => {
      toast({ kind: "success", title: t("All sessions revoked"), description: t("Sign in again on every device.") });
      await signOut();
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not revoke sessions"), description: error.message }),
  });

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("Settings")}
        description={t("Make Aurora yours, tune how often it syncs, and review the connection to this server.")}
        meta={
          <>
            <Badge tone="neutral">Unraid {snapshot.system.version}</Badge>
            <Badge tone="success" dot>{t("Authenticated")}</Badge>
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RotateCcw className="size-3.5" />} onClick={reset} disabled={!dirty}>
              {t("Reset")}
            </Button>
            <Button
              variant="primary"
              icon={saved ? <Check className="size-3.5" /> : <Save className="size-3.5" />}
              loading={saving}
              disabled={!dirty || saving}
              onClick={() => void save()}
            >
              {saved ? t("Saved") : t("Save changes")}
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
                  {t(item.label)}
                </button>
              );
            })}
          </nav>
          <div className="mt-3 hidden rounded-xl border border-line px-3 py-3 lg:block">
            <p className="text-[9px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Connected to")}</p>
            <p className="mt-1.5 text-[12px] font-medium text-ink">{settings.serverName || snapshot.system.name}</p>
            <p className="font-mono text-[9.5px] text-subtle">{snapshot.system.hostname} · {snapshot.system.version}</p>
            <Button size="sm" variant="ghost" className="mt-2 h-7 px-2 text-[10px]" onClick={refresh}>
              {t("Re-sync now")}
            </Button>
          </div>
        </aside>

        <div className="space-y-4">
          {section === "appearance" ? (
            <>
              <Card className="card-pad">
                <CardTitle title={t("Theme")} description={t("Aurora adapts every surface, chart and control to your preference.")} />
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
                          <span className="text-[12px] font-medium text-ink">{t(option.label)}</span>
                          {active ? <Check className="ml-auto size-3.5 text-[var(--accent)]" /> : null}
                        </div>
                        <p className="mt-0.5 text-[10px] text-subtle">{t(option.hint)}</p>
                      </button>
                    );
                  })}
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title={t("Accent")} description={t("One colour drives highlights, charts and focus states.")} />
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
                      <span className="text-[11.5px] font-medium text-soft">{t(accent.label)}</span>
                      {settings.accent === accent.value ? <Check className="size-3 text-[var(--accent)]" /> : null}
                    </button>
                  ))}
                </div>

                <div className="my-5 h-px bg-line" />

                <SettingsRow label={t("Density")} description={t("Compact reduces padding and row height across the interface.")}>
                  <div data-glass-control="segmented" className="flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
                    {(["comfortable", "compact"] as const).map((value) => (
                      <button
                        key={value}
                        type="button"
                        aria-pressed={settings.density === value}
                        onClick={() => {
                          update("density", value);
                          theme.setDensity(value);
                        }}
                        className={cn(
                          "rounded-md px-3 py-1 text-[11px] font-medium capitalize transition",
                          settings.density === value ? "bg-[var(--surface)] text-ink shadow-sm" : "text-muted hover:text-ink",
                        )}
                      >
                        {t(value === "comfortable" ? "Comfortable" : "Compact")}
                      </button>
                    ))}
                  </div>
                </SettingsRow>

                <SettingsRow
                  label={t("Surfaces")}
                  description={t("Glass gives cards, panels and popovers a frosted, translucent material. Stored on this device.")}
                >
                  <div data-glass-control="segmented" className="flex rounded-lg border border-line bg-[var(--surface-2)] p-0.5">
                    {([false, true] as const).map((value) => (
                      <button
                        key={String(value)}
                        type="button"
                        aria-pressed={theme.glass === value}
                        onClick={() => theme.setGlass(value)}
                        className={cn(
                          "rounded-md px-3 py-1 text-[11px] font-medium transition",
                          theme.glass === value ? "bg-[var(--surface)] text-ink shadow-sm" : "text-muted hover:text-ink",
                        )}
                      >
                        {t(value ? "Glass" : "Standard")}
                      </button>
                    ))}
                  </div>
                </SettingsRow>

                <SettingsRow
                  label={t("Wallpaper")}
                  description={t("Image displayed behind cards, darkened by 50%. Stored on this device.")}
                >
                  <div className="flex items-center gap-2">
                    {theme.wallpaper ? (
                      <img
                        src={theme.wallpaper}
                        alt=""
                        className="h-10 w-16 rounded-md border border-line object-cover"
                      />
                    ) : null}
                    <label
                      data-glass-control="button"
                      className={cn(
                        "inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] font-medium text-soft transition hover:border-line-strong hover:text-ink",
                        wallpaperBusy && "pointer-events-none opacity-60",
                      )}
                    >
                      <Upload className="size-3.5" />
                      {wallpaperBusy ? t("Processing…") : t("Choose image")}
                      <input
                        type="file"
                        accept="image/png,image/jpeg,image/webp"
                        className="hidden"
                        onChange={(event) => {
                          const file = event.target.files?.[0];
                          event.target.value = "";
                          if (file) void applyWallpaper(file);
                        }}
                      />
                    </label>
                    {theme.wallpaper ? (
                      <IconButton label={t("Remove wallpaper")} size="sm" onClick={() => theme.setWallpaper("")}>
                        <Trash2 className="size-3.5" />
                      </IconButton>
                    ) : null}
                  </div>
                </SettingsRow>

                <SettingsRow label={t("Temperature unit")} description={t("Used for disks, CPU and sensors.")}>
                  <Select
                    ariaLabel={t("Temperature unit")}
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
              <CardTitle title={t("General")} description={t("Server identity, refresh cadence and language.")} />
              <div className="mt-2 divide-y divide-line">
                <SettingsRow label={t("Server display name")} description={t("Defaults to the hostname. Shown in the sidebar and page titles.")}>
                  <input
                    value={settings.serverName}
                    placeholder={snapshot.system.hostname}
                    onChange={(event) => update("serverName", event.target.value)}
                    className="h-9 w-52 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[12px] text-ink outline-none transition focus:border-[var(--accent-line)]"
                  />
                </SettingsRow>
                <SettingsRow label={t("Refresh interval")} description={t("How often widgets re-sync with the server.")}>
                  <Select
                    ariaLabel={t("Refresh interval")}
                    value={String(settings.refreshInterval)}
                    onChange={(value) => update("refreshInterval", Number(value))}
                    options={[2, 5, 10, 30, 60].map((value) => ({ value: String(value), label: t("{count} seconds", { count: value }) }))}
                  />
                </SettingsRow>
                <SettingsRow label={t("Language")} description={t("Interface language for Aurora.")}>
                  <Select
                    ariaLabel={t("Language")}
                    value={settings.language}
                    onChange={(value) => {
                      update("language", value);
                      setLocale(value === "fr" ? "fr" : "en");
                    }}
                    options={[
                      { value: "en", label: t("English") },
                      { value: "fr", label: t("Français") },
                    ]}
                  />
                </SettingsRow>
                <SettingsRow label={t("Automatic updates")} description={t("Allow Aurora to surface and apply plugin updates.")}>
                  <Switch checked={settings.autoUpdate} onChange={(value) => update("autoUpdate", value)} label={t("Automatic updates")} />
                </SettingsRow>
                <SettingsRow label={t("Anonymous telemetry")} description={t("Share crash reports and feature usage. Off by default.")}>
                  <Switch checked={settings.telemetry} onChange={(value) => update("telemetry", value)} label={t("Anonymous telemetry")} />
                </SettingsRow>
              </div>
            </Card>
          ) : null}

          {section === "notifications" ? (
            <Card className="card-pad">
              <CardTitle title={t("Notifications")} description={t("Choose which channels receive alerts from your server.")} />
              <div className="mt-2 divide-y divide-line">
                <SettingsRow label={t("Minimum importance")} description={t("Ignore Unraid notifications below this level.")}>
                  <select
                    value={settings.notifyMinImportance || ""}
                    onChange={(event) => update("notifyMinImportance", event.target.value)}
                    aria-label={t("Minimum importance")}
                    className="h-9 w-40 rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)]"
                  >
                    <option value="">{t("All")}</option>
                    <option value="info">{t("Info")}</option>
                    <option value="normal">{t("Normal")}</option>
                    <option value="warning">{t("Warning")}</option>
                    <option value="alert">{t("Alert")}</option>
                    <option value="critical">{t("Critical")}</option>
                  </select>
                </SettingsRow>
                <SettingsRow label={t("Muted sources")} description={t("Comma-separated Unraid notification sources to ignore, for example docker, vm.")}>
                  <MutedSourcesField
                    value={settings.notifyMutedSources ?? []}
                    onChange={(value) => update("notifyMutedSources", value)}
                  />
                </SettingsRow>
                <SettingsRow label={t("Email alerts")} description={t("Send warnings and alerts to the address configured in Unraid.")}>
                  <Switch checked={settings.emailAlerts} onChange={(value) => update("emailAlerts", value)} label={t("Email alerts")} />
                </SettingsRow>
                <SettingsRow label={t("Push notifications")} description={t("Browser and mobile push through Unraid Connect agents.")}>
                  <Switch checked={settings.pushAlerts} onChange={(value) => update("pushAlerts", value)} label={t("Push notifications")} />
                </SettingsRow>
              </div>
              <div className="mt-2 divide-y divide-line">
                <SettingsRow label={t("Discord webhook")} description={t("Paste a Discord channel webhook URL.")}>
                  <input
                    value={settings.discordWebhook}
                    onChange={(event) => update("discordWebhook", event.target.value)}
                    placeholder="https://discord.com/api/webhooks/…"
                    aria-label={t("Discord webhook")}
                    className="h-9 w-72 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
                  />
                </SettingsRow>
                <SettingsRow label={t("ntfy URL")} description={t("Full ntfy topic URL, for example https://ntfy.sh/aurora.")}>
                  <input
                    value={settings.ntfyUrl}
                    onChange={(event) => update("ntfyUrl", event.target.value)}
                    placeholder="https://ntfy.sh/aurora"
                    aria-label={t("ntfy URL")}
                    className="h-9 w-72 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
                  />
                </SettingsRow>
                <div className="py-3.5">
                  <p className="text-[11.5px] font-medium text-soft">{t("Generic webhook")}</p>
                  <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">{t("Send a custom request to any HTTP endpoint.")}</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <select
                      value={settings.webhookMethod || "POST"}
                      onChange={(event) => update("webhookMethod", event.target.value)}
                      aria-label={t("Webhook method")}
                      className="h-9 w-24 rounded-[7px] border border-line bg-[var(--surface-2)] px-2 font-mono text-[10px] text-soft outline-none focus:border-[var(--accent-line)]"
                    >
                      {["POST", "GET", "PUT", "DELETE"].map((method) => <option key={method} value={method}>{method}</option>)}
                    </select>
                    <input
                      value={settings.webhookUrl}
                      onChange={(event) => update("webhookUrl", event.target.value)}
                      placeholder="https://example.com/aurora"
                      aria-label={t("Generic webhook URL")}
                      className="h-9 min-w-0 flex-1 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
                    />
                  </div>
                  <div className="mt-3">
                    <div className="flex items-center justify-between">
                      <p className="text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Headers")}</p>
                      <Button size="sm" variant="ghost" icon={<Plus className="size-3" />} onClick={() => update("webhookHeaders", { ...(settings.webhookHeaders ?? {}), "": "" })}>
                        {t("Add header")}
                      </Button>
                    </div>
                    <div className="mt-1 space-y-1.5">
                      {Object.entries(settings.webhookHeaders ?? {}).map(([key, value], index) => (
                        <div key={index} className="flex flex-col gap-1.5 sm:flex-row sm:gap-2">
                          <input
                            value={key}
                            onChange={(event) => {
                              const next = Object.entries(settings.webhookHeaders ?? {});
                              next[index] = [event.target.value, value];
                              update("webhookHeaders", Object.fromEntries(next.filter(([name]) => name)));
                            }}
                            placeholder="Authorization"
                            aria-label={t("Header name")}
                            className="h-8 w-full rounded-md border border-line bg-[var(--surface-2)] px-2 font-mono text-[9.5px] text-ink outline-none focus:border-[var(--accent-line)] sm:w-40"
                          />
                          <input
                            value={value}
                            onChange={(event) => {
                              const next = Object.entries(settings.webhookHeaders ?? {});
                              next[index] = [key, event.target.value];
                              update("webhookHeaders", Object.fromEntries(next.filter(([name]) => name)));
                            }}
                            placeholder="Bearer …"
                            aria-label={t("Header value")}
                            className="h-8 w-full min-w-0 rounded-md border border-line bg-[var(--surface-2)] px-2 font-mono text-[9.5px] text-ink outline-none focus:border-[var(--accent-line)] sm:flex-1"
                          />
                          <IconButton label={t("Remove header")} size="sm" onClick={() => {
                            const next = Object.entries(settings.webhookHeaders ?? {}).filter((_, item) => item !== index);
                            update("webhookHeaders", Object.fromEntries(next));
                          }}>
                            <Trash2 className="size-3.5" />
                          </IconButton>
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="mt-3">
                    <p className="text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Body template")}</p>
                    <textarea
                      value={settings.webhookBody}
                      onChange={(event) => update("webhookBody", event.target.value)}
                      placeholder={'{"title":"{title}","body":"{description}"}'}
                      aria-label={t("Body template")}
                      className="mt-1 h-24 w-full resize-y rounded-lg border border-line bg-[var(--surface-2)] p-2.5 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
                    />
                    <p className="mt-1 text-[9px] text-subtle">
                      {t("Placeholders: {title}, {subject}, {description}, {importance}, {source}, {timestamp}")}
                    </p>
                  </div>
                </div>
              </div>
              <div className="mt-4 border-t border-line pt-4">
                <p className="text-[11.5px] font-medium text-soft">{t("Alert events")}</p>
                <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">
                  {t("Choose which Unraid event categories can reach the configured channels.")}
                </p>
                <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
                  {([
                    ["array", "Array & parity"],
                    ["disk", "Disks & SMART"],
                    ["docker", "Docker"],
                    ["vm", "Virtual machines"],
                    ["mover", "Mover"],
                    ["plugin", "Plugins & updates"],
                    ["ups", "UPS & power"],
                    ["system", "System"],
                  ] as const).map(([event, label]) => (
                    <label key={event} className="flex items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)]/45 px-2.5 py-2 text-[10px] text-muted">
                      <Switch
                        checked={settings.notifyEvents?.[event] !== false}
                        onChange={(value) => update("notifyEvents", { ...(settings.notifyEvents ?? {}), [event]: value })}
                        label={t(label)}
                      />
                      {t(label)}
                    </label>
                  ))}
                </div>
              </div>
              <div className="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
                <span className="flex min-w-0 items-start gap-2">
                  <Bell className="mt-0.5 size-3.5 shrink-0 text-subtle" />
                  {t("Aurora forwards new Unraid notifications to every configured channel. Save the settings before relying on them in production.")}
                </span>
                <Button size="sm" variant="secondary" loading={testingNotification} onClick={() => void testNotification()}>
                  {t("Send test")}
                </Button>
              </div>
            </Card>
          ) : null}

          {section === "trivy" ? (
            <TrivyPanel
              enabled={settings.trivyEnabled ?? false}
              version={settings.trivyVersion ?? ""}
              hour={settings.trivyDbUpdateHour ?? 3}
              onEnabledChange={(value) => update("trivyEnabled", value)}
              onVersionChange={(value) => update("trivyVersion", value)}
              onHourChange={(value) => update("trivyDbUpdateHour", value)}
            />
          ) : null}

          {section === "ups" ? <UPSView /> : null}

          {section === "os" ? <OSUpdatePanel /> : null}

          {section === "unraid-api" ? <UnraidAPIPanel /> : null}

          {section === "docker" ? <DockerSettingsPanel /> : null}

          {section === "vm" ? <VMSettingsPanel /> : null}

          {section === "nvidia" ? <NvidiaDriverPanel /> : null}

          {section === "proxy" ? (
            <ProxySettingsPanel
              enabled={settings.proxyEnabled ?? false}
              proxies={settings.proxies ?? []}
              activeId={settings.proxyActiveId ?? ""}
              onEnabledChange={(value) => update("proxyEnabled", value)}
              onProxiesChange={(value) => update("proxies", value)}
              onActiveChange={(value) => update("proxyActiveId", value)}
            />
          ) : null}

          {section === "mcp" ? (
            <Card className="card-pad">
              <CardTitle title={t("MCP server")} description={t("Expose the Model Context Protocol endpoint for AI clients.")} />
              <div className="mt-2 divide-y divide-line">
                <SettingsRow
                  label={t("Enable MCP server")}
                  description={t("Serves /api/v1/mcp for bearer-token clients. Disabled by default.")}
                >
                  <Switch
                    checked={settings.mcpEnabled ?? false}
                    onChange={(value) => update("mcpEnabled", value)}
                    label={t("Enable MCP server")}
                  />
                </SettingsRow>
              </div>
              <p className="mt-2 text-[9.5px] leading-relaxed text-subtle">
                {t("The local stdio mode (aurora mcp) is not affected. Save changes to apply.")}
              </p>
            </Card>
          ) : null}

          {section === "api" ? (
            <Card className="card-pad">
              <CardTitle title={t("API tokens")} description={t("Create scoped bearer tokens for automation and Prometheus scraping.")} />
              {newToken ? (
                <div className="mt-4 rounded-xl border border-[var(--success-soft)] bg-[var(--success-soft)] p-3">
                  <p className="text-[11px] font-medium text-[var(--success)]">{t("Copy this token now. It will not be shown again.")}</p>
                  <div className="mt-2 flex gap-2">
                    <code className="min-w-0 flex-1 overflow-x-auto rounded-lg border border-line bg-[var(--surface)] px-2 py-1.5 font-mono text-[10px] text-soft">{newToken}</code>
                    <Button size="sm" variant="secondary" onClick={() => { void navigator.clipboard.writeText(newToken); toast({ kind: "info", title: t("Token copied") }); }}>{t("Copy")}</Button>
                    <Button size="sm" variant="ghost" onClick={() => setNewToken("")}>{t("Dismiss")}</Button>
                  </div>
                </div>
              ) : null}
              <div className="mt-4 flex flex-wrap items-end gap-2 border-b border-line pb-4">
                <label className="min-w-[220px] flex-1">
                  <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Token name")}</span>
                  <input value={tokenName} onChange={(event) => setTokenName(event.target.value)} placeholder="home-assistant" className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11px] text-ink outline-none focus:border-[var(--accent-line)]" />
                </label>
                <label className="min-w-[150px]">
                  <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Role")}</span>
                  <select
                    value={tokenRole}
                    onChange={(event) => setTokenRole(event.target.value as "viewer" | "operator" | "admin")}
                    aria-label={t("Role")}
                    className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)]"
                  >
                    <option value="viewer">{t("Viewer")}</option>
                    <option value="operator">{t("Operator")}</option>
                    <option value="admin">{t("Admin")}</option>
                  </select>
                </label>
                <label className="min-w-[130px]">
                  <span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Expiry")}</span>
                  <select
                    value={tokenExpiry}
                    onChange={(event) => setTokenExpiry(Number(event.target.value))}
                    aria-label={t("Expiry")}
                    className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none focus:border-[var(--accent-line)]"
                  >
                    <option value={30}>{t("30 days")}</option>
                    <option value={90}>{t("90 days")}</option>
                    <option value={365}>{t("1 year")}</option>
                    <option value={0}>{t("Never")}</option>
                  </select>
                </label>
                <Button variant="primary" loading={createToken.isPending} disabled={!tokenName.trim()} onClick={() => createToken.mutate()}>{t("Create token")}</Button>
              </div>
              <div className="mt-4 rounded-xl border border-line bg-[var(--surface-2)]/35 p-3">
                <p className="text-[10px] font-medium text-soft">{t("Scopes")}</p>
                <p className="mt-0.5 text-[9px] text-subtle">{t("Leave all unchecked to allow every resource for this role.")}</p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {([
                    ["docker", "Docker"],
                    ["vms", "Virtual machines"],
                    ["storage", "Storage"],
                    ["network", "Network"],
                    ["system", "System"],
                    ["ups", "UPS"],
                    ["apps", "Applications"],
                    ["automation", "Automation"],
                    ["access", "Access"],
                  ] as const).map(([scope, label]) => (
                    <label key={scope} className="flex items-center gap-1.5 rounded-lg border border-line px-2 py-1 text-[9.5px] text-muted">
                      <input
                        type="checkbox"
                        checked={tokenScopes.includes(scope)}
                        onChange={(event) => setTokenScopes(event.target.checked ? [...tokenScopes, scope] : tokenScopes.filter((item) => item !== scope))}
                        className="size-3.5 accent-[var(--accent)]"
                      />
                      {t(label)}
                    </label>
                  ))}
                </div>
              </div>
              <div className="mt-3 space-y-2">
                {tokens.isLoading ? <p className="text-[10.5px] text-muted">{t("Loading API tokens…")}</p> : tokens.isError ? <p className="text-[10.5px] text-[var(--danger)]">{(tokens.error as Error).message}</p> : tokens.data?.length ? tokens.data.map((token) => (
                  <div key={token.id} className="flex flex-wrap items-center gap-2 rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2">
                    <span className="min-w-0 flex-1 truncate text-[11px] font-medium text-soft">{token.name}</span>
                    <Badge tone={token.role === "viewer" ? "info" : token.role === "operator" ? "warning" : "danger"}>
                      {t(token.role === "viewer" ? "Viewer" : token.role === "operator" ? "Operator" : "Admin")}
                    </Badge>
                    {token.scopes?.length ? <Badge tone="neutral">{t("{count} scopes", { count: token.scopes.length })}</Badge> : null}
                    <span className="text-[9px] text-subtle">
                      {token.expiresAt && new Date(token.expiresAt).getTime() <= Date.now()
                        ? t("Expired")
                        : token.expiresAt
                          ? t("Expires {date}", { date: new Date(token.expiresAt).toLocaleDateString() })
                          : t("Never expires")}
                      {" · "}
                      {token.lastUsed ? new Date(token.lastUsed).toLocaleString() : t("Never used")}
                    </span>
                    <IconButton label={t("Revoke token")} size="sm" onClick={() => { if (window.confirm(t("Revoke API token {name}?", { name: token.name }))) revokeToken.mutate(token.id); }}><Trash2 className="size-3.5" /></IconButton>
                  </div>
                )) : <p className="text-[10.5px] text-subtle">{t("No API tokens yet.")}</p>}
              </div>
              <p className="mt-3 text-[9.5px] leading-relaxed text-subtle">{t("Use a token as Authorization: Bearer <token>. Viewers can only read, operators can manage containers, VMs, array and mover actions, and admins control everything. Token management itself requires a signed-in session.")}</p>
              <p className="mt-2 text-[9.5px] leading-relaxed text-subtle">
                {t("Browse every endpoint and role in the")}{" "}
                <a href="/api/docs" target="_blank" rel="noreferrer" className="text-[var(--accent)] underline decoration-dotted underline-offset-2">
                  {t("API reference")}
                </a>
                .
              </p>
            </Card>
          ) : null}

          {section === "security" ? (
            <>
              <Card className="card-pad">
                <CardTitle title={t("Session")} description={t("How this browser is authenticated against Aurora.")} />
                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <InfoBlock label={t("Account")} value={session?.user ?? "admin"} icon={<KeyRound className="size-3.5" />} />
                  <InfoBlock label={t("Authentication")} value="Native · /etc/shadow" tone="success" />
                  <InfoBlock label={t("Session store")} value="HMAC-SHA256 · HttpOnly" icon={<ShieldCheck className="size-3.5" />} />
                  <InfoBlock label={t("CSRF protection")} value={t("Custom header + origin check")} />
                </div>
                <div className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
                  <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
                  {t("Only the host")}{" "}
                  <code className="code-chip">root</code>{" "}
                  {t("account can sign in, matching the Unraid webGUI. Credentials are validated against the local shadow database and are never stored by Aurora.")}
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title={t("Access")} description={t("End the current session on this device.")} />
                <div className="mt-3 flex items-center justify-between gap-4">
                  <p className="text-[11px] text-muted">{t("You will be returned to the sign-in screen.")}</p>
                  <Button variant="danger" icon={<LogOut className="size-3.5" />} onClick={() => void signOut()}>
                    {t("Sign out")}
                  </Button>
                </div>
                <div className="mt-4 flex flex-wrap items-center justify-between gap-4 border-t border-line pt-4">
                  <div>
                    <p className="text-[11px] font-medium text-soft">{t("Sign out everywhere")}</p>
                    <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">
                      {t("Invalidate every Aurora session, including this browser. Useful after a token or password leak.")}
                    </p>
                  </div>
                  <Button
                    variant="secondary"
                    loading={revokeAll.isPending}
                    onClick={() => {
                      if (window.confirm(t("Sign out every session?"))) revokeAll.mutate();
                    }}
                  >
                    {t("Revoke all sessions")}
                  </Button>
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title={t("Active sessions")} description={t("Every browser currently signed in to Aurora. Sessions end when Aurora restarts.")} />
                {authSessions.isLoading ? (
                  <p className="mt-3 text-[10.5px] text-muted">{t("Loading sessions…")}</p>
                ) : authSessions.isError ? (
                  <p className="mt-3 text-[10.5px] text-[var(--danger)]">{(authSessions.error as Error).message}</p>
                ) : (
                  <div className="mt-3 divide-y divide-line">
                    {(authSessions.data ?? []).map((item) => (
                      <div key={item.nonce} className="flex flex-wrap items-center gap-3 py-2.5">
                        <div className="min-w-0 flex-1">
                          <p className="flex items-center gap-2 text-[11px] font-medium text-soft">
                            {item.user}
                            {item.current ? <Badge tone="success">{t("This device")}</Badge> : null}
                          </p>
                          <p className="mt-0.5 text-[9.5px] text-subtle">
                            {item.ip || "—"} · {t("Last seen {date}", { date: formatDateTime(item.lastSeen) })}
                          </p>
                        </div>
                        <span className="text-[9px] text-subtle">{t("Expires {date}", { date: formatDateTime(item.expires) })}</span>
                        <Button
                          size="sm"
                          variant="secondary"
                          loading={revokeSession.isPending && revokeSession.variables === item.nonce}
                          onClick={() => {
                            if (window.confirm(t("Revoke this session?"))) revokeSession.mutate(item.nonce);
                          }}
                        >
                          {t("Revoke")}
                        </Button>
                      </div>
                    ))}
                    {(authSessions.data ?? []).length === 0 ? <p className="py-2 text-[10.5px] text-subtle">{t("No active sessions.")}</p> : null}
                  </div>
                )}
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
                      <Badge tone="success" dot>{t("live host")}</Badge>
                    </div>
                    <p className="mt-1 max-w-xl text-[11.5px] leading-relaxed text-muted">
                      {t("An independent, opinionated control plane for Unraid. Built in Go with a React front end — designed to make homelab infrastructure feel calm and precise.")}
                    </p>
                  </div>
                </div>
                <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-3">
                  <InfoBlock label="Unraid" value={snapshot.system.version} />
                  <InfoBlock label={t("Kernel")} value={snapshot.system.kernel} />
                  <InfoBlock label={t("Uptime")} value={formatUptime(snapshot.system.uptime)} />
                  <InfoBlock label={t("Board")} value={snapshot.system.board} />
                  <InfoBlock label={t("Memory")} value={formatMemory(snapshot.system.memory.total)} icon={<Cpu className="size-3.5" />} />
                  <InfoBlock label={t("License")} value={snapshot.system.license || t("Unknown")} />
                </div>
              </Card>

              <Card className="card-pad">
                <CardTitle title={t("Credits")} description={t("Aurora stands on the shoulders of the Unraid community.")} />
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
                        <span className="block text-[11.5px] font-medium text-soft">{t(link.label)}</span>
                        <span className="block truncate text-[9.5px] text-subtle">{t(link.description)}</span>
                      </span>
                      <span className="text-[10px] text-subtle">↗</span>
                    </a>
                  ))}
                </div>
                <p className="mt-4 border-t border-line pt-3 text-[9.5px] leading-relaxed text-subtle">
                  {t("Aurora is not affiliated with Lime Technology. Unraid is a trademark of Lime Technology, Inc.")}
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
      className="h-9 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-soft outline-none transition focus:border-[var(--accent-line)]"
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
}

function MutedSourcesField({ value, onChange }: { value: string[]; onChange: (value: string[]) => void }) {
  const [text, setText] = useState(value.join(", "));
  return (
    <input
      value={text}
      onChange={(event) => {
        setText(event.target.value);
        onChange(event.target.value.split(",").map((item) => item.trim()).filter(Boolean));
      }}
      placeholder="docker, vm"
      aria-label="Muted sources"
      className="h-9 w-72 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
    />
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
