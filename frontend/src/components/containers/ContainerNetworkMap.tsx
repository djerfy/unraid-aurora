import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Boxes, Globe2, RefreshCw, Router, TriangleAlert, Waypoints } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { Badge, Card, IconButton, cn } from "../ui/primitives";
import { EntityIcon, containerIconURL } from "../ui/entity-icon";
import type { NetworkMapEdge, NetworkMapPeer } from "../../lib/types";

const NODE_HEIGHT = 52;
const ROW = 62;
const HEADER = 34;
const WIDTH = 980;
const COLUMN = 320;
const PEER_X = WIDTH - COLUMN;

const SCOPE_COLOR: Record<string, string> = {
  container: "var(--success)",
  lan: "var(--accent)",
  internet: "var(--warning)",
  other: "var(--muted)",
};

function countryFlag(code?: string): string {
  if (!code || !/^[A-Z]{2}$/.test(code)) return "";
  return String.fromCodePoint(...[...code].map((char) => 127397 + char.charCodeAt(0)));
}

function scopeLabel(scope: string, t: (key: string) => string): string {
  if (scope === "container") return t("Containers");
  if (scope === "lan") return t("LAN");
  if (scope === "internet") return t("Internet");
  return t("Other");
}

type MapLink = {
  key: string;
  containerId: string;
  containerName: string;
  peer: NetworkMapPeer;
  count: number;
  ports: number[];
  protocols: string[];
};

function buildLinks(edges: NetworkMapEdge[], peers: NetworkMapPeer[]): MapLink[] {
  const byAddress = new Map(peers.map((peer) => [peer.address, peer]));
  const links = new Map<string, MapLink>();
  for (const edge of edges) {
    const peer = byAddress.get(edge.peerAddress);
    if (!peer) continue;
    const key = `${edge.containerId}|${edge.peerAddress}`;
    let link = links.get(key);
    if (!link) {
      link = {
        key,
        containerId: edge.containerId,
        containerName: edge.containerName,
        peer,
        count: 0,
        ports: [],
        protocols: [],
      };
      links.set(key, link);
    }
    link.count += 1;
    if (edge.remotePort && !link.ports.includes(edge.remotePort)) link.ports.push(edge.remotePort);
    if (edge.protocol && !link.protocols.includes(edge.protocol)) link.protocols.push(edge.protocol);
  }
  return [...links.values()].sort((a, b) => {
    if (a.containerName !== b.containerName) return a.containerName.localeCompare(b.containerName);
    return a.peer.address.localeCompare(b.peer.address);
  });
}

export function ContainerNetworkMap() {
  const { t } = useI18n();
  const [scope, setScope] = useState("all");
  const [hovered, setHovered] = useState<string | null>(null);
  const [pinned, setPinned] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["container-network-map"],
    queryFn: api.containerNetworkMap,
    refetchInterval: (result) => (result.state.data?.available === false ? false : 15000),
  });
  const data = query.data;
  const allPeers = data?.peers ?? [];
  const scopeCounts = {
    container: allPeers.filter((peer) => peer.scope === "container").length,
    lan: allPeers.filter((peer) => peer.scope === "lan").length,
    internet: allPeers.filter((peer) => peer.scope === "internet").length,
  };
  const peers = scope === "all" ? allPeers : allPeers.filter((peer) => peer.scope === scope);
  const peerAddresses = new Set(peers.map((peer) => peer.address));
  const edges = (data?.edges ?? []).filter((edge) => peerAddresses.has(edge.peerAddress));
  const visibleContainerIDs = new Set(edges.map((edge) => edge.containerId));
  const mapContainers = (data?.containers ?? [])
    .filter((container) => scope === "all" || visibleContainerIDs.has(container.id))
    .sort((a, b) => a.name.localeCompare(b.name));
  const links = buildLinks(edges, peers);
  const activeCandidate = hovered ?? pinned;
  const active =
    activeCandidate &&
    (mapContainers.some((container) => `c:${container.id}` === activeCandidate) ||
      peers.some((peer) => `p:${peer.address}` === activeCandidate))
      ? activeCandidate
      : null;

  const containerIndex = new Map(mapContainers.map((container, index) => [container.id, index]));
  const peerIndex = new Map(peers.map((peer, index) => [peer.address, index]));
  const containerCounts = new Map<string, number>();
  for (const edge of edges) {
    containerCounts.set(edge.containerId, (containerCounts.get(edge.containerId) ?? 0) + 1);
  }
  const peerCounts = new Map<string, number>();
  for (const edge of edges) {
    peerCounts.set(edge.peerAddress, (peerCounts.get(edge.peerAddress) ?? 0) + 1);
  }
  const height = HEADER + Math.max(mapContainers.length, peers.length, 1) * ROW;
  const linkIsActive = (link: MapLink) =>
    !active || active === `c:${link.containerId}` || active === `p:${link.peer.address}`;
  const nodeIsDimmed = (key: string) => Boolean(active && active !== key);

  const body = (() => {
    if (query.isLoading) {
      return <p className="py-12 text-center text-[10.5px] text-muted">{t("Scanning container sockets…")}</p>;
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
    if (!data || data.edges.length === 0) {
      return (
        <div className="grid min-h-[220px] place-items-center rounded-xl border border-dashed border-line bg-[var(--surface-2)]/35 px-4 text-center">
          <div>
            <Waypoints className="mx-auto size-6 text-subtle" />
            <p className="mt-2 text-[11.5px] font-medium text-soft">{t("No remote connection detected right now.")}</p>
            <p className="mt-1 text-[10px] text-subtle">{t("Run a container with active sockets to populate the map.")}</p>
          </div>
        </div>
      );
    }
    if (peers.length === 0 || mapContainers.length === 0) {
      return (
        <div className="grid min-h-[180px] place-items-center rounded-xl border border-dashed border-line bg-[var(--surface-2)]/35 px-4 text-center">
          <p className="text-[11px] text-muted">{t("No peer matches this scope.")}</p>
        </div>
      );
    }
    return (
      <>
        <div className="overflow-x-auto rounded-xl border border-line bg-[var(--surface-2)]/20">
          <div className="relative" style={{ width: WIDTH, height }}>
            <span className="absolute top-2 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle" style={{ left: 4 }}>
              {t("Containers")}
            </span>
            <span className="absolute top-2 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle" style={{ left: PEER_X + 4 }}>
              {t("Peers")}
            </span>

            <svg className="pointer-events-none absolute inset-0" width={WIDTH} height={height} aria-hidden="true">
              {links.map((link) => {
                const from = containerIndex.get(link.containerId);
                const to = peerIndex.get(link.peer.address);
                if (from === undefined || to === undefined) return null;
                const y1 = HEADER + from * ROW + NODE_HEIGHT / 2;
                const y2 = HEADER + to * ROW + NODE_HEIGHT / 2;
                const mid = (COLUMN + PEER_X) / 2;
                return (
                  <path
                    key={link.key}
                    d={`M ${COLUMN + 4} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${PEER_X - 4} ${y2}`}
                    fill="none"
                    stroke={SCOPE_COLOR[link.peer.scope] ?? "var(--muted)"}
                    strokeWidth={1 + Math.min(3, link.count * 0.5)}
                    strokeOpacity={linkIsActive(link) ? 0.72 : 0.1}
                    className="transition-opacity"
                  />
                );
              })}
            </svg>

            {mapContainers.map((container, index) => {
              const key = `c:${container.id}`;
              const count = containerCounts.get(container.id) ?? 0;
              return (
                <button
                  key={key}
                  type="button"
                  onMouseEnter={() => setHovered(key)}
                  onMouseLeave={() => setHovered(null)}
                  onClick={() => setPinned((value) => (value === key ? null : key))}
                  className={cn(
                    "absolute z-10 flex items-center gap-2.5 rounded-lg border border-line bg-[var(--surface)] px-2.5 text-left shadow-sm transition hover:border-[var(--accent-line)]",
                    nodeIsDimmed(key) && "opacity-30",
                    pinned === key && "border-[var(--accent-line)]",
                  )}
                  style={{ left: 0, top: HEADER + index * ROW, width: COLUMN, height: NODE_HEIGHT }}
                >
                  <EntityIcon src={containerIconURL(container.id)} name={container.name} size={28} className="rounded-md" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[11px] font-semibold text-ink">{container.name}</span>
                    <span className="block truncate font-mono text-[9px] text-subtle">
                      {container.ip || t("host network")} · {container.network || "—"}
                    </span>
                  </span>
                  <span className="shrink-0 rounded border border-line bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-[9px] text-subtle">
                    {count}
                  </span>
                </button>
              );
            })}

            {peers.map((peer, index) => {
              const key = `p:${peer.address}`;
              const flag = countryFlag(peer.country);
              return (
                <button
                  key={key}
                  type="button"
                  onMouseEnter={() => setHovered(key)}
                  onMouseLeave={() => setHovered(null)}
                  onClick={() => setPinned((value) => (value === key ? null : key))}
                  className={cn(
                    "absolute z-10 flex items-center gap-2.5 rounded-lg border border-l-2 border-line bg-[var(--surface)] px-2.5 text-left shadow-sm transition hover:border-[var(--accent-line)]",
                    nodeIsDimmed(key) && "opacity-30",
                    pinned === key && "border-[var(--accent-line)]",
                  )}
                  style={{
                    left: PEER_X,
                    top: HEADER + index * ROW,
                    width: COLUMN,
                    height: NODE_HEIGHT,
                    borderLeftColor: SCOPE_COLOR[peer.scope] ?? "var(--muted)",
                  }}
                >
                  <span className="grid size-7 shrink-0 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-[13px] text-muted">
                    {peer.scope === "container" ? (
                      <Boxes className="size-3.5" />
                    ) : flag ? (
                      flag
                    ) : peer.scope === "internet" ? (
                      <Globe2 className="size-3.5" />
                    ) : (
                      <Router className="size-3.5" />
                    )}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[11px] font-semibold text-ink">
                      {peer.containerName || peer.address}
                    </span>
                    <span className="block truncate font-mono text-[9px] text-subtle">
                      {peer.containerName ? `${peer.address} · ` : ""}
                      {peer.protocols.join("/")}
                      {peer.ports.length ? ` :${peer.ports.slice(0, 3).join(",")}` : ""}
                      {peer.ports.length > 3 ? "…" : ""}
                    </span>
                  </span>
                  <span className="shrink-0 rounded border border-line bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-[9px] text-subtle">
                    {peerCounts.get(peer.address) ?? 0}
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        <div className="mt-5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-[12px] font-semibold text-ink">{t("Who talks to whom")}</h3>
            <span className="text-[9.5px] text-subtle">
              {t("Click a node to pin its edges. Hover the table to isolate a container.")}
            </span>
          </div>
          <div className="mt-2 overflow-x-auto rounded-xl border border-line">
            <div className="min-w-[760px]">
              <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.1fr)_96px_88px_100px_70px] gap-3 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[8.5px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <span>{t("Container")}</span>
                <span>{t("Peer")}</span>
                <span>{t("Scope")}</span>
                <span>{t("Protocol")}</span>
                <span>{t("Ports")}</span>
                <span className="text-right">{t("Sockets")}</span>
              </div>
              <div className="max-h-[360px] divide-y divide-line/60 overflow-y-auto">
                {links.map((link) => (
                  <div
                    key={link.key}
                    onMouseEnter={() => setHovered(`c:${link.containerId}`)}
                    onMouseLeave={() => setHovered(null)}
                    className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1.1fr)_96px_88px_100px_70px] items-center gap-3 px-3 py-2 text-[10px] transition hover:bg-[var(--surface-2)]/50"
                  >
                    <span className="min-w-0 truncate text-[10.5px] font-medium text-ink">{link.containerName}</span>
                    <span className="min-w-0 truncate font-mono text-[9.5px] text-muted" title={link.peer.containerName || link.peer.address}>
                      {link.peer.containerName ? `${link.peer.containerName} (${link.peer.address})` : link.peer.address}
                    </span>
                    <span><Badge tone="neutral">{scopeLabel(link.peer.scope, t)}</Badge></span>
                    <span className="font-mono text-[9.5px] uppercase text-muted">{link.protocols.join("/") || "—"}</span>
                    <span className="font-mono text-[9.5px] text-muted">{link.ports.slice(0, 4).join(", ") || "—"}</span>
                    <span className="text-right font-mono text-[9.5px] text-subtle">{link.count}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </>
    );
  })();

  return (
    <Card className="card-pad">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink">
            <Waypoints className="size-4 text-[var(--accent)]" /> {t("Network map")}
          </h2>
          <p className="mt-1 max-w-2xl text-[10.5px] leading-relaxed text-muted">
            {t("Live view of which container talks to which remote IP. Peers include other containers, LAN hosts and internet addresses.")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={scope}
            onChange={(event) => setScope(event.target.value)}
            aria-label={t("Scope")}
            className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
          >
            <option value="all">{t("All scopes")} ({allPeers.length})</option>
            <option value="container">{t("Containers")} ({scopeCounts.container})</option>
            <option value="lan">{t("LAN")} ({scopeCounts.lan})</option>
            <option value="internet">{t("Internet")} ({scopeCounts.internet})</option>
          </select>
          <IconButton label={t("Refresh")} size="md" onClick={() => void query.refetch()} disabled={query.isFetching}>
            <RefreshCw className={cn("size-4", query.isFetching && "animate-spin")} />
          </IconButton>
        </div>
      </div>

      {data ? (
        <div className="mt-3 flex flex-wrap items-center gap-2 text-[10px] text-subtle">
          <Badge tone="neutral">{t("{count} containers", { count: data.containers.length })}</Badge>
          <Badge tone="neutral">{t("{count} peers", { count: allPeers.length })}</Badge>
          <Badge tone="neutral">{t("{count} sockets", { count: data.edges.length })}</Badge>
          <span>{t("Updated {time} · auto-refresh every 15 seconds", { time: new Date(data.capturedAt).toLocaleTimeString() })}</span>
        </div>
      ) : null}

      <div className="mt-4">{body}</div>

      <p className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-line pt-2.5 text-[9px] text-subtle">
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: SCOPE_COLOR.container }} /> {t("Containers")}</span>
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: SCOPE_COLOR.lan }} /> {t("LAN")}</span>
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: SCOPE_COLOR.internet }} /> {t("Internet")}</span>
        <span>{t("Live sockets are inspected with ss inside each container namespace. Loopback sockets are hidden.")}</span>
      </p>
    </Card>
  );
}
