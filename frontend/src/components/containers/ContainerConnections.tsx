import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Globe2, RefreshCw, TriangleAlert } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { Badge, Card, IconButton, cn } from "../ui/primitives";
import type { Container, ContainerConnection } from "../../lib/types";

function useConnections(containerId: string) {
  return useQuery({
    queryKey: ["container-connections", containerId],
    queryFn: () => api.containerConnections(containerId),
    enabled: Boolean(containerId),
    refetchInterval: (query) => (query.state.data?.available === false ? false : 5000),
  });
}

function countryFlag(code: string): string {
  if (!/^[A-Z]{2}$/.test(code)) return "";
  return String.fromCodePoint(...[...code].map((char) => 127397 + char.charCodeAt(0)));
}

function endpoint(address: string, port: number): string {
  return address.includes(":") ? `[${address}]:${port}` : `${address}:${port}`;
}

function locationLabel(connection: ContainerConnection): string {
  if (connection.country && connection.city) return `${connection.country} · ${connection.city}`;
  return connection.country || connection.city || "—";
}

export function ContainerConnections({
  containers,
  container,
  compact = false,
}: {
  containers?: Container[];
  container?: Container | null;
  compact?: boolean;
}) {
  const { t } = useI18n();
  const [target, setTarget] = useState("");
  const containerId = containers ? target : container?.id ?? "";
  const query = useConnections(containerId);
  const data = query.data;
  const connections = data?.connections ?? [];
  const uniqueHosts = new Set(connections.map((connection) => connection.remoteAddress)).size;
  const selected = containers?.find((item) => item.id === containerId) ?? container ?? null;

  const refresh = () => void query.refetch();

  const body = (() => {
    if (!containerId) {
      return (
        <div className="grid min-h-[180px] place-items-center rounded-xl border border-dashed border-line px-4 text-center">
          <div>
            <Globe2 className="mx-auto size-5 text-subtle" />
            <p className="mt-2 text-[11px] font-medium text-soft">{t("Select a container")}</p>
            <p className="mt-1 text-[10px] text-subtle">{t("Choose a running container to inspect its external network calls.")}</p>
          </div>
        </div>
      );
    }
    if (query.isLoading) {
      return <p className="py-10 text-center text-[10.5px] text-muted">{t("Inspecting connections…")}</p>;
    }
    if (query.isError) {
      return (
        <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] text-[var(--danger)]">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
          <span>{query.error.message}</span>
        </div>
      );
    }
    if (data && !data.available) {
      return (
        <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] px-3 py-2.5 text-[10.5px] text-[var(--warning)]">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
          <span>{t(data.reason || "ss is not available on this host")}</span>
        </div>
      );
    }
    if (data?.reason) {
      return <p className="py-8 text-center text-[10.5px] text-muted">{t(data.reason)}</p>;
    }
    if (connections.length === 0) {
      return (
        <div className="grid min-h-[160px] place-items-center rounded-xl border border-line bg-[var(--surface-2)]/35 px-4 text-center">
          <div>
            <p className="text-[11px] font-medium text-soft">{t("No external connection detected")}</p>
            <p className="mt-1 text-[10px] text-subtle">{t("The container is running but has no public remote socket right now.")}</p>
          </div>
        </div>
      );
    }
    return <ConnectionsTable connections={connections} />;
  })();

  if (compact) {
    return (
      <section>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
            <Globe2 className="size-3.5" /> {t("External network calls")}
          </h3>
          <div className="flex items-center gap-1">
            {connections.length > 0 ? <Badge tone="neutral">{t("{count} connections", { count: connections.length })}</Badge> : null}
            <IconButton label={t("Refresh")} size="sm" onClick={refresh} disabled={!containerId || query.isFetching}>
              <RefreshCw className={cn("size-3.5", query.isFetching && "animate-spin")} />
            </IconButton>
          </div>
        </div>
        <p className="mt-1 text-[9.5px] text-subtle">
          {t("Observed with ss inside the container network namespace. Country and city are optional via GeoIP.")}
        </p>
        <div className="mt-3">{body}</div>
      </section>
    );
  }

  return (
    <Card className="card-pad">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink">
            <Globe2 className="size-4 text-[var(--accent)]" /> {t("External network calls")}
          </h2>
          <p className="mt-1 max-w-2xl text-[10.5px] leading-relaxed text-muted">
            {t("Outbound connections observed with ss inside the selected container. Empty until a container is selected; refresh every 5 seconds.")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            aria-label={t("Container")}
            className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
          >
            <option value="">{t("Select a container…")}</option>
            {(containers ?? []).map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <IconButton label={t("Refresh")} size="md" onClick={refresh} disabled={!containerId || query.isFetching}>
            <RefreshCw className={cn("size-4", query.isFetching && "animate-spin")} />
          </IconButton>
        </div>
      </div>

      {selected ? (
        <div className="mt-3 flex flex-wrap items-center gap-2 text-[10px] text-subtle">
          <Badge tone="neutral">{selected.image}</Badge>
          <span className="font-mono">{selected.ip || "—"}</span>
          {connections.length > 0 ? (
            <span>
              {t("{connections} connections · {hosts} remote hosts", { connections: connections.length, hosts: uniqueHosts })}
            </span>
          ) : null}
        </div>
      ) : null}

      <div className="mt-4">{body}</div>

      <p className="mt-3 flex items-center gap-1.5 border-t border-line pt-2.5 text-[9px] text-subtle">
        <Globe2 className="size-3" />
        {t("Only public remote addresses are shown. Country and city are optional via GeoIP.")}
      </p>
    </Card>
  );
}

function ConnectionsTable({ connections }: { connections: ContainerConnection[] }) {
  const { t } = useI18n();
  return (
    <div className="overflow-x-auto rounded-xl border border-line">
      <div className="min-w-[720px]">
        <div className="grid grid-cols-[minmax(0,1.35fr)_minmax(0,0.9fr)_minmax(0,1fr)_70px_80px_minmax(0,1fr)] gap-3 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[8.5px] font-semibold uppercase tracking-[0.1em] text-subtle">
          <span>{t("Remote")}</span>
          <span>{t("Location")}</span>
          <span>{t("Process")}</span>
          <span>{t("Protocol")}</span>
          <span>{t("State")}</span>
          <span>{t("Local")}</span>
        </div>
        <div className="divide-y divide-line/60">
          {connections.map((connection) => (
            <div
              key={`${connection.protocol}-${connection.localAddress}-${connection.localPort}-${connection.remoteAddress}-${connection.remotePort}-${connection.pid}`}
              className="grid grid-cols-[minmax(0,1.35fr)_minmax(0,0.9fr)_minmax(0,1fr)_70px_80px_minmax(0,1fr)] items-center gap-3 px-3 py-2 text-[10px]"
            >
              <span className="min-w-0">
                <span className="flex items-center gap-2">
                  {connection.country ? <span className="text-[13px] leading-none">{countryFlag(connection.country)}</span> : null}
                  <span className="truncate font-mono text-[10.5px] text-ink">{endpoint(connection.remoteAddress, connection.remotePort)}</span>
                </span>
              </span>
              <span className="truncate text-[10px] text-muted" title={locationLabel(connection)}>{locationLabel(connection)}</span>
              <span className="min-w-0 truncate text-[10px] text-soft" title={connection.process ? `${connection.process} (${connection.pid})` : "—"}>
                {connection.process || "—"}
                {connection.pid ? <span className="ml-1 font-mono text-[9px] text-subtle">({connection.pid})</span> : null}
              </span>
              <span><Badge tone="neutral">{connection.protocol.toUpperCase()}</Badge></span>
              <span><Badge tone={connection.state === "ESTAB" ? "success" : "neutral"}>{connection.state}</Badge></span>
              <span className="truncate font-mono text-[9.5px] text-subtle" title={endpoint(connection.localAddress, connection.localPort)}>
                {endpoint(connection.localAddress, connection.localPort)}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
