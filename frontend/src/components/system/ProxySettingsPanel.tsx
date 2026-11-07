import { Globe, Plus, Trash2 } from "lucide-react";
import { Badge, Button, Card, IconButton, Switch } from "../ui/primitives";
import { useI18n } from "../../i18n";
import type { ProxyConfig } from "../../lib/types";

const MAX_PROXIES = 4;

function newProxyID(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `proxy-${crypto.randomUUID()}`;
  }
  return `proxy-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
  optional,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: "text" | "password";
  optional?: boolean;
}) {
  const { t } = useI18n();
  return (
    <label className="block min-w-0">
      <span className="mb-1 flex items-baseline gap-1.5 text-[10px] font-medium uppercase tracking-wide text-subtle">
        {label}
        {optional ? <span className="normal-case tracking-normal text-muted">{t("optional")}</span> : null}
      </span>
      <input
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        autoComplete="off"
        spellCheck={false}
        className="h-9 w-full rounded-[7px] border border-line bg-[var(--surface-2)] px-3 font-mono text-[11px] text-ink outline-none transition focus:border-[var(--accent-line)]"
      />
    </label>
  );
}

function ProxyCard({
  proxy,
  active,
  onSelect,
  onChange,
  onRemove,
}: {
  proxy: ProxyConfig;
  active: boolean;
  onSelect: () => void;
  onChange: (patch: Partial<ProxyConfig>) => void;
  onRemove: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className={`rounded-xl border p-3.5 transition ${active ? "border-[var(--accent-line)] bg-[color-mix(in_srgb,var(--accent)_5%,transparent)]" : "border-line"}`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <label className="flex cursor-pointer items-center gap-2">
          <input
            type="radio"
            name="proxy-active"
            checked={active}
            onChange={onSelect}
            className="size-3.5 accent-[var(--accent)]"
          />
          <span className="text-[11.5px] font-medium text-ink">{proxy.name || t("New proxy")}</span>
          {active ? <Badge tone="success" dot>{t("Active")}</Badge> : null}
        </label>
        <IconButton label={t("Remove proxy")} size="sm" onClick={onRemove}>
          <Trash2 className="size-3.5" />
        </IconButton>
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <Field
          label={t("Name")}
          value={proxy.name}
          onChange={(name) => onChange({ name })}
          placeholder={t("Home proxy")}
        />
        <Field
          label={t("URL")}
          value={proxy.url}
          onChange={(url) => onChange({ url })}
          placeholder="http://192.168.1.10:3128"
        />
        <Field
          label={t("Username")}
          value={proxy.username}
          onChange={(username) => onChange({ username })}
          placeholder={t("Optional username")}
          optional
        />
        <Field
          label={t("Password")}
          value={proxy.password}
          onChange={(password) => onChange({ password })}
          placeholder={t("Optional password")}
          type="password"
          optional
        />
      </div>
    </div>
  );
}

export function ProxySettingsPanel({
  enabled,
  proxies,
  activeId,
  onEnabledChange,
  onProxiesChange,
  onActiveChange,
}: {
  enabled: boolean;
  proxies: ProxyConfig[];
  activeId: string;
  onEnabledChange: (value: boolean) => void;
  onProxiesChange: (value: ProxyConfig[]) => void;
  onActiveChange: (value: string) => void;
}) {
  const { t } = useI18n();
  const list = proxies ?? [];
  const canAdd = list.length < MAX_PROXIES;

  const addProxy = () => {
    if (!canAdd) return;
    const proxy: ProxyConfig = { id: newProxyID(), name: "", url: "", username: "", password: "" };
    onProxiesChange([...list, proxy]);
    if (!activeId) onActiveChange(proxy.id);
  };

  const updateProxy = (id: string, patch: Partial<ProxyConfig>) => {
    onProxiesChange(list.map((proxy) => (proxy.id === id ? { ...proxy, ...patch } : proxy)));
  };

  const removeProxy = (id: string) => {
    const next = list.filter((proxy) => proxy.id !== id);
    onProxiesChange(next);
    if (activeId === id) onActiveChange(next[0]?.id ?? "");
    if (next.length === 0) onEnabledChange(false);
  };

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <Globe className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("Outbound proxy")}</h2>
              <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                {t("Route Aurora's internet traffic through one of up to four proxy configurations.")}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={enabled ? "success" : "neutral"} dot>{enabled ? t("Enabled") : t("Disabled")}</Badge>
            <Badge tone="neutral">{t("{count}/{max} configurations", { count: list.length, max: MAX_PROXIES })}</Badge>
          </div>
        </div>
        <div className="mt-4 flex items-center justify-between gap-3 border-t border-line pt-3.5">
          <div className="min-w-0 max-w-lg">
            <p className="text-[11.5px] font-medium text-soft">{t("Enable outbound proxy")}</p>
            <p className="mt-0.5 text-[10px] leading-relaxed text-subtle">
              {t("Applies to registry checks, the app catalog, plugin and Unraid updates, Trivy downloads and notifications.")}
            </p>
          </div>
          <Switch checked={enabled} onChange={onEnabledChange} label={t("Enable outbound proxy")} />
        </div>
      </Card>

      <Card className="card-pad">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-[13px] font-semibold text-ink">{t("Proxy configurations")}</h2>
          <Button
            size="sm"
            variant="secondary"
            icon={<Plus className="size-3.5" />}
            onClick={addProxy}
            disabled={!canAdd}
          >
            {t("Add proxy")}
          </Button>
        </div>
        {list.length === 0 ? (
          <p className="mt-3 rounded-lg border border-dashed border-line px-3 py-6 text-center text-[10.5px] text-muted">
            {t("No proxy configuration yet. Add up to four and select the active one.")}
          </p>
        ) : (
          <div className="mt-3 space-y-3">
            {list.map((proxy) => (
              <ProxyCard
                key={proxy.id}
                proxy={proxy}
                active={proxy.id === activeId}
                onSelect={() => onActiveChange(proxy.id)}
                onChange={(patch) => updateProxy(proxy.id, patch)}
                onRemove={() => removeProxy(proxy.id)}
              />
            ))}
          </div>
        )}
        {!enabled && list.length > 0 ? (
          <p className="mt-3 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 py-2 text-[10px] leading-relaxed text-subtle">
            {t("The proxy is saved but disabled. Enable it above to route Aurora's outbound requests.")}
          </p>
        ) : null}
        <p className="mt-3 text-[10px] leading-relaxed text-muted">
          {t("Credentials are stored in Aurora's root-owned settings file. Docker image pulls are performed by the Docker daemon and use its own network settings.")}
        </p>
      </Card>
    </div>
  );
}
