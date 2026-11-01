import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Boxes,
  CheckCircle2,
  Download,
  Plus,
  Puzzle,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Tag,
  Trash2,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Drawer, Modal } from "../components/ui/overlays";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  InputBase,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { api } from "../lib/api";
import { useToast } from "../app/ToastProvider";
import { hexToRgba } from "../lib/format";
import type { Plugin } from "../lib/types";

export function PluginsPage() {
  const { snapshot } = useApp();
  const { toast } = useToast();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("All");
  const [selected, setSelected] = useState<Plugin | null>(null);
  const [updateRun, setUpdateRun] = useState<{ title: string; lines: string[]; running: boolean; error: string } | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  const [installURL, setInstallURL] = useState("");
  const [removeTarget, setRemoveTarget] = useState<Plugin | null>(null);
  const queryClient = useQueryClient();

  const check = useMutation({
    mutationFn: api.checkPluginUpdates,
    onSuccess: (plugins) => {
      const available = plugins.filter((plugin) => plugin.updateAvailable).length;
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({
        kind: available ? "warning" : "success",
        title: available ? `${available} plugin update${available > 1 ? "s" : ""} available` : "All plugins are up to date",
        description: `${plugins.length} plugin${plugins.length > 1 ? "s" : ""} checked`,
      });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: "Plugin update check failed", description: error.message });
    },
  });

  const streamPluginTask = async (title: string, task: (onEvent: (event: import("../lib/types").PluginEvent) => void) => Promise<void>, success: string) => {
    setUpdateRun({ title, lines: [], running: true, error: "" });
    try {
      await task((event) => {
        setUpdateRun((current) => {
          if (!current) return current;
          const lines = event.line ? [...current.lines.slice(-400), event.line] : current.lines;
          return { ...current, lines, error: event.error ? event.line : current.error };
        });
      });
      setUpdateRun((current) => (current ? { ...current, running: false } : current));
      await queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      toast({ kind: "success", title: success });
    } catch (error) {
      setUpdateRun((current) => (current ? { ...current, running: false, error: (error as Error).message } : current));
      toast({ kind: "error", title: "Plugin operation failed", description: (error as Error).message });
    }
  };

  const runUpdate = (plugin: Plugin) =>
    streamPluginTask(`Updating ${plugin.name}`, (onEvent) => api.updatePlugin(plugin.name, onEvent), `${plugin.name} updated`);

  const runInstall = () => {
    const target = installURL.trim();
    if (!/^https?:\/\//i.test(target)) {
      toast({ kind: "error", title: "Invalid plugin URL" });
      return;
    }
    setInstallOpen(false);
    setInstallURL("");
    void streamPluginTask("Installing plugin", (onEvent) => api.installPlugin(target, onEvent), "Plugin installed");
  };

  const runRemove = () => {
    if (!removeTarget) return;
    const plugin = removeTarget;
    setRemoveTarget(null);
    setSelected(null);
    void streamPluginTask(`Removing ${plugin.name}`, (onEvent) => api.removePlugin(plugin.name, onEvent), `${plugin.name} removed`);
  };

  if (!snapshot) return null;
  const plugins = snapshot.plugins;
  const updates = plugins.filter((plugin) => plugin.updateAvailable);
  const categories = ["All", ...Array.from(new Set(plugins.map((plugin) => plugin.category)))];

  const filtered = useMemo(
    () =>
      plugins
        .filter((plugin) => category === "All" || plugin.category === category)
        .filter((plugin) => {
          if (!query.trim()) return true;
          return `${plugin.name} ${plugin.author} ${plugin.description}`.toLowerCase().includes(query.toLowerCase());
        }),
    [plugins, category, query],
  );

  return (
    <>
      <PageHeader
        eyebrow="System"
        title="Plugins"
        description="Extensions that add capabilities to Unraid — from applications and automation to hardware support."
        meta={
          <>
            <Badge tone="accent" dot>{plugins.length} installed</Badge>
            {updates.length > 0 ? <Badge tone="warning">{updates.length} updates available</Badge> : <Badge tone="success">Up to date</Badge>}
          </>
        }
        actions={
          <>
            <Button
              variant="secondary"
              icon={<RefreshCw className="size-3.5" />}
              loading={check.isPending}
              onClick={() => check.mutate()}
            >
              Check updates
            </Button>
            <Button variant="primary" icon={<Plus className="size-3.5" />} onClick={() => setInstallOpen(true)}>
              Install plugin
            </Button>
            <Button
              variant="secondary"
              icon={<Sparkles className="size-3.5" />}
              onClick={() => window.open("https://ca.unraid.net", "_blank", "noopener")}
            >
              Community Apps
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Stat icon={<Puzzle className="size-4" />} label="Installed" value={String(plugins.length)} hint="active extensions" />
        <Stat icon={<Download className="size-4" />} label="Updates" value={String(updates.length)} hint={updates.length ? "ready to install" : "everything current"} tone={updates.length ? "warning" : "success"} />
        <Stat icon={<Tag className="size-4" />} label="Categories" value={String(categories.length - 1)} hint="across your plugins" />
        <Stat
          icon={<ShieldCheck className="size-4" />}
          label="Auto update"
          value={snapshot.settings.autoUpdate ? "On" : "Off"}
          hint={snapshot.settings.autoUpdate ? "notify and install" : "manual approval"}
        />
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          {categories.map((item) => (
            <button
              key={item}
              type="button"
              onClick={() => setCategory(item)}
              className={cn(
                "h-8 rounded-lg border px-3 text-[11.5px] font-medium transition",
                category === item
                  ? "border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)]"
                  : "border-line bg-[var(--surface-2)] text-muted hover:border-line-strong hover:text-ink",
              )}
            >
              {item}
            </button>
          ))}
        </div>
        <InputBase
          value={query}
          onChange={setQuery}
          placeholder="Search plugins…"
          icon={<Search className="size-3.5" />}
          className="ml-auto w-52"
        />
      </div>

      {filtered.length === 0 ? (
        <Card>
          <EmptyState icon={<Puzzle className="size-5" />} title="No plugins match this filter" description="Try another category or search term." />
        </Card>
      ) : (
        <div className="stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map((plugin) => (
            <Card key={plugin.name} hover className="card-pad flex flex-col">
              <div className="flex items-start justify-between gap-3">
                <button type="button" onClick={() => setSelected(plugin)} className="flex min-w-0 items-center gap-3 text-left">
                  <span
                    className="grid size-10 shrink-0 place-items-center rounded-xl border text-[12px] font-bold uppercase"
                    style={{ color: plugin.color, backgroundColor: hexToRgba(plugin.color, 0.12), borderColor: hexToRgba(plugin.color, 0.24) }}
                  >
                    {plugin.name.slice(0, 2)}
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-[13px] font-semibold text-ink">{plugin.name}</span>
                    <span className="mt-0.5 block truncate text-[10px] text-subtle">{plugin.author}</span>
                  </span>
                </button>
                {plugin.updateAvailable ? <Badge tone="warning">update</Badge> : <Badge tone="success" dot>current</Badge>}
              </div>

              <p className="mt-3 line-clamp-2 min-h-[32px] text-[11px] leading-relaxed text-muted">{plugin.description}</p>

              <div className="mt-3 flex flex-wrap items-center gap-1.5">
                <Badge tone="neutral">{plugin.category}</Badge>
                <span className="chip font-mono text-[9.5px]">{plugin.version}</span>
                {plugin.updateAvailable ? <span className="chip font-mono text-[9.5px] text-[var(--warning)]">→ {plugin.latest}</span> : null}
              </div>

              <div className="mt-4 flex items-center gap-2 border-t border-line pt-3">
                <span className="flex items-center gap-1.5 text-[9.5px] text-subtle">
                  <span className={cn("status-dot", plugin.enabled ? "live" : "muted")} />
                  {plugin.enabled ? "Enabled" : "Disabled"}
                </span>
                {plugin.updateAvailable ? (
                  <Button
                    size="sm"
                    variant="soft"
                    className="ml-auto px-2 text-[11px]"
                    icon={<Download className="size-3" />}
                    onClick={() => void runUpdate(plugin)}
                  >
                    Update
                  </Button>
                ) : null}
                <Button
                  size="sm"
                  variant="ghost"
                  className={`${plugin.updateAvailable ? "" : "ml-auto"} px-2 text-[11px]`}
                  onClick={() => setSelected(plugin)}
                >
                  Details
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Drawer
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        title={selected?.name ?? ""}
        subtitle={selected ? `${selected.author} · ${selected.category}` : undefined}
      >
        {selected ? (
          <div className="space-y-5">
            <div className="flex items-center gap-4 rounded-xl border border-line bg-[var(--surface-2)]/60 p-4">
              <span
                className="grid size-12 place-items-center rounded-xl border text-[14px] font-bold uppercase"
                style={{ color: selected.color, backgroundColor: hexToRgba(selected.color, 0.12), borderColor: hexToRgba(selected.color, 0.24) }}
              >
                {selected.name.slice(0, 2)}
              </span>
              <div>
                <div className="flex items-center gap-2">
                  <p className="text-[14px] font-semibold text-ink">{selected.name}</p>
                  {selected.updateAvailable ? <Badge tone="warning">update available</Badge> : <Badge tone="success">up to date</Badge>}
                </div>
                <p className="mt-1 font-mono text-[10px] text-subtle">v{selected.version}</p>
              </div>
            </div>

            <p className="text-[11.5px] leading-relaxed text-muted">{selected.description}</p>

            <div className="grid grid-cols-2 gap-3">
              <Info label="Installed" value={selected.version} />
              <Info label="Latest" value={selected.latest} />
              <Info label="Author" value={selected.author} />
              <Info label="Category" value={selected.category} />
            </div>

            {selected.updateAvailable ? (
              <div className="rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3">
                <p className="flex items-center gap-2 text-[11px] font-semibold text-[var(--warning)]">
                  <Download className="size-3.5" /> Version {selected.latest} is available
                </p>
                <div className="mt-2">
                  <Button
                    size="sm"
                    variant="soft"
                    icon={<Download className="size-3" />}
                    onClick={() => void runUpdate(selected)}
                  >
                    Update plugin
                  </Button>
                </div>
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-xl border border-line p-3 text-[10.5px] text-[var(--success)]">
                <CheckCircle2 className="size-3.5" /> This plugin is running the latest available version.
              </div>
            )}

            <div className="flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3 text-[10.5px] leading-relaxed text-muted">
              <Boxes className="mt-0.5 size-3.5 shrink-0" />
              Plugins extend Unraid at the system level. Only install plugins you trust, and keep a current
              flash backup before updating.
            </div>

            <Button
              variant="danger"
              icon={<Trash2 className="size-3.5" />}
              onClick={() => setRemoveTarget(selected)}
            >
              Remove plugin
            </Button>
          </div>
        ) : null}
      </Drawer>

      <Modal
        open={Boolean(updateRun)}
        onClose={() => {
          if (!updateRun?.running) setUpdateRun(null);
        }}
        title={updateRun?.title ?? ""}
        description="Output from the official Unraid plugin manager."
        size="xl"
        footer={
          <Button variant="secondary" onClick={() => setUpdateRun(null)} disabled={updateRun?.running}>
            Close
          </Button>
        }
      >
        <div className="space-y-3">
          <pre className="max-h-[55vh] overflow-auto rounded-xl border border-[#1f2230] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed text-[#c7cbd6]">
            {updateRun?.lines.length ? updateRun.lines.join("\n") : "Starting plugin manager…"}
          </pre>
          {updateRun?.error ? (
            <p className="rounded-lg border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2 text-[10.5px] text-[var(--danger)]">
              {updateRun.error}
            </p>
          ) : null}
          {updateRun?.running ? <p className="text-[10px] text-subtle">Update in progress…</p> : null}
        </div>
      </Modal>

      <Modal
        open={installOpen}
        onClose={() => setInstallOpen(false)}
        title="Install a plugin"
        description="Downloads a .plg definition and runs it through the official Unraid plugin manager."
        footer={
          <>
            <Button variant="secondary" onClick={() => setInstallOpen(false)}>
              Cancel
            </Button>
            <Button variant="primary" icon={<Download className="size-3.5" />} onClick={runInstall}>
              Install
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <label className="block space-y-1">
            <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">Plugin URL</span>
            <input
              value={installURL}
              onChange={(event) => setInstallURL(event.target.value)}
              placeholder="https://example.com/my-plugin.plg"
              className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]"
            />
          </label>
          <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
            <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
            Plugins run as root. Only install definitions from sources you trust.
          </div>
        </div>
      </Modal>

      <Modal
        open={Boolean(removeTarget)}
        onClose={() => setRemoveTarget(null)}
        title={`Remove ${removeTarget?.name ?? "plugin"}?`}
        description="The official plugin manager removes the plugin files and its installed marker."
        footer={
          <>
            <Button variant="secondary" onClick={() => setRemoveTarget(null)}>
              Cancel
            </Button>
            <Button variant="danger" icon={<Trash2 className="size-3.5" />} onClick={runRemove}>
              Remove plugin
            </Button>
          </>
        }
      >
        <p className="text-[10.5px] leading-relaxed text-muted">
          Settings and data stored outside the plugin directory are preserved.
        </p>
      </Modal>
    </>
  );
}

function Stat({
  icon,
  label,
  value,
  hint,
  tone,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint: string;
  tone?: "warning" | "success";
}) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className={cn("num mt-3 text-[21px] font-semibold leading-none", tone === "warning" ? "text-[var(--warning)]" : tone === "success" ? "text-[var(--success)]" : "text-ink")}>
        {value}
      </p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2">
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-1 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}

