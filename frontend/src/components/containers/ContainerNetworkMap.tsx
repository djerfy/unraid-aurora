import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDownLeft,
  ArrowUpRight,
  Boxes,
  Globe2,
  RefreshCw,
  Router,
  TriangleAlert,
  Waypoints,
} from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { Badge, Card, IconButton, cn } from "../ui/primitives";
import { EntityIcon, containerIconURL } from "../ui/entity-icon";
import type { Container, NetworkMapEdge, NetworkMapPeer } from "../../lib/types";

const NODE_HEIGHT = 52;
const ROW = 62;
const HEADER = 34;

const SCOPE_COLOR: Record<string, string> = {
  container: "var(--success)",
  lan: "var(--accent)",
  internet: "var(--warning)",
  other: "var(--muted)",
};

const DIRECTIONS = ["inbound", "outbound"] as const;
type Direction = (typeof DIRECTIONS)[number];

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

function scopeRank(scope: string): number {
  if (scope === "container") return 0;
  if (scope === "lan") return 1;
  if (scope === "internet") return 2;
  return 3;
}

type PeerNode = NetworkMapPeer & {
  direction: Direction;
  edgeCount: number;
  ports: number[];
  protocols: string[];
};

type MapLink = {
  key: string;
  containerId: string;
  containerName: string;
  peerKey: string;
  peer: PeerNode;
  direction: Direction;
  count: number;
  ports: number[];
  protocols: string[];
};

function buildGraph(edges: NetworkMapEdge[], peers: NetworkMapPeer[]) {
  const byAddress = new Map(peers.map((peer) => [peer.address, peer]));
  const inbound = new Map<string, PeerNode>();
  const outbound = new Map<string, PeerNode>();
  const links = new Map<string, MapLink>();

  for (const edge of edges) {
    const peer = byAddress.get(edge.peerAddress);
    if (!peer) continue;
    const direction: Direction = edge.direction === "inbound" ? "inbound" : "outbound";
    const bucket = direction === "inbound" ? inbound : outbound;
    let node = bucket.get(peer.address);
    if (!node) {
      node = { ...peer, direction, edgeCount: 0, ports: [], protocols: [] };
      bucket.set(peer.address, node);
    }
    node.edgeCount += 1;
    const port = direction === "inbound" ? edge.localPort : edge.remotePort;
    if (port && !node.ports.includes(port)) node.ports.push(port);
    if (edge.protocol && !node.protocols.includes(edge.protocol)) node.protocols.push(edge.protocol);

    const key = `${direction}|${edge.containerId}|${peer.address}`;
    let link = links.get(key);
    if (!link) {
      link = {
        key,
        containerId: edge.containerId,
        containerName: edge.containerName,
        peerKey: `${direction === "inbound" ? "i" : "o"}:${peer.address}`,
        peer: node,
        direction,
        count: 0,
        ports: [],
        protocols: [],
      };
      links.set(key, link);
    }
    link.count += 1;
    if (port && !link.ports.includes(port)) link.ports.push(port);
    if (edge.protocol && !link.protocols.includes(edge.protocol)) link.protocols.push(edge.protocol);
  }

  const sortPeers = (a: PeerNode, b: PeerNode) => {
    if (scopeRank(a.scope) !== scopeRank(b.scope)) return scopeRank(a.scope) - scopeRank(b.scope);
    if (a.edgeCount !== b.edgeCount) return b.edgeCount - a.edgeCount;
    return a.address.localeCompare(b.address);
  };

  return {
    inbound: [...inbound.values()].sort(sortPeers),
    outbound: [...outbound.values()].sort(sortPeers),
    links: [...links.values()].sort((a, b) => {
      if (a.containerName !== b.containerName) return a.containerName.localeCompare(b.containerName);
      if (a.direction !== b.direction) return a.direction === "inbound" ? -1 : 1;
      return a.peer.address.localeCompare(b.peer.address);
    }),
  };
}

export function ContainerNetworkMap({ containers }: { containers: Container[] }) {
  const { t } = useI18n();
  const [target, setTarget] = useState("");
  const [scope, setScope] = useState("all");
  const [hovered, setHovered] = useState<string | null>(null);
  const [hoveredLinkKey, setHoveredLinkKey] = useState<string | null>(null);
  const [pinned, setPinned] = useState<string | null>(null);
  const graphRef = useRef<HTMLDivElement>(null);
  const [graphWidth, setGraphWidth] = useState(0);

  useEffect(() => {
    const element = graphRef.current;
    if (!element) return;
    const measure = () => setGraphWidth(element.clientWidth);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const query = useQuery({
    queryKey: ["container-network-map", target],
    queryFn: () => api.containerNetworkMap(target === "all" ? undefined : target),
    enabled: target !== "",
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
  const { inbound, outbound, links } = buildGraph(edges, peers);

  const activeCandidate = hovered ?? pinned;
  const active =
    activeCandidate &&
    (mapContainers.some((container) => `c:${container.id}` === activeCandidate) ||
      inbound.some((peer) => `i:${peer.address}` === activeCandidate) ||
      outbound.some((peer) => `o:${peer.address}` === activeCandidate))
      ? activeCandidate
      : null;
  const emphasizedLink = hoveredLinkKey ? links.find((link) => link.key === hoveredLinkKey) ?? null : null;

  const containerIndex = new Map(mapContainers.map((container, index) => [container.id, index]));
  const inboundIndex = new Map(inbound.map((peer, index) => [peer.address, index]));
  const outboundIndex = new Map(outbound.map((peer, index) => [peer.address, index]));
  const containerCounts = new Map<string, { inbound: number; outbound: number }>();
  for (const edge of edges) {
    const counts = containerCounts.get(edge.containerId) ?? { inbound: 0, outbound: 0 };
    if (edge.direction === "inbound") counts.inbound += 1;
    else counts.outbound += 1;
    containerCounts.set(edge.containerId, counts);
  }

  const height = HEADER + Math.max(inbound.length, mapContainers.length, outbound.length, 1) * ROW;
  const viewportWidth = graphWidth > 0 ? Math.max(320, Math.floor(graphWidth * 0.9) - 4) : 900;
  const gutter = Math.min(80, Math.max(24, Math.round(viewportWidth * 0.05)));
  const columnWidth = Math.max(0, Math.floor((viewportWidth - 2 * gutter) / 3));
  const usedWidth = columnWidth * 3 + 2 * gutter;
  const startX = Math.max(0, Math.floor((viewportWidth - usedWidth) / 2));
  const inboundX = startX;
  const containerX = startX + columnWidth + gutter;
  const outboundX = startX + 2 * (columnWidth + gutter);

  const linkIsActive = (link: MapLink) =>
    emphasizedLink
      ? link.key === emphasizedLink.key
      : !active || active === `c:${link.containerId}` || active === link.peerKey;
  const nodeIsDimmed = (key: string) => {
    if (emphasizedLink) {
      return key !== `c:${emphasizedLink.containerId}` && key !== emphasizedLink.peerKey;
    }
    return Boolean(active && active !== key);
  };

  const peerNode = (peer: PeerNode, index: number, left: number) => {
    const key = `${peer.direction === "inbound" ? "i" : "o"}:${peer.address}`;
    const flag = countryFlag(peer.country);
    const Arrow = peer.direction === "inbound" ? ArrowDownLeft : ArrowUpRight;
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
          left,
          top: HEADER + index * ROW,
          width: columnWidth,
          height: NODE_HEIGHT,
          borderLeftColor: SCOPE_COLOR[peer.scope] ?? "var(--muted)",
        }}
      >
        <span className="grid size-7 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-[var(--surface-2)] text-[13px] text-muted">
          {peer.scope === "container" && peer.containerId ? (
            <EntityIcon
              src={containerIconURL(peer.containerId)}
              name={peer.containerName || peer.address}
              size={24}
              className="rounded-md"
            />
          ) : peer.scope === "container" ? (
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
        <span className="flex shrink-0 items-center gap-0.5 rounded border border-line bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-[9px] text-subtle">
          <Arrow className="size-2.5" />
          {peer.edgeCount}
        </span>
      </button>
    );
  };

  const body = (() => {
    if (!target) {
      return (
        <div className="grid min-h-[220px] place-items-center rounded-xl border border-dashed border-line bg-[var(--surface-2)]/35 px-4 text-center">
          <div>
            <Waypoints className="mx-auto size-6 text-subtle" />
            <p className="mt-2 text-[11.5px] font-medium text-soft">{t("Select a container")}</p>
            <p className="mt-1 text-[10px] text-subtle">{t("Choose a container or All containers to build the network map.")}</p>
          </div>
        </div>
      );
    }
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
    if (inbound.length === 0 && outbound.length === 0) {
      return (
        <div className="grid min-h-[180px] place-items-center rounded-xl border border-dashed border-line bg-[var(--surface-2)]/35 px-4 text-center">
          <p className="text-[11px] text-muted">{t("No peer matches this scope.")}</p>
        </div>
      );
    }
    return (
      <>
        <div className="my-8 overflow-x-auto rounded-xl border border-line bg-[var(--surface-2)]/20 py-6">
          <div className="relative mx-auto" style={{ width: "90%", height }}>
            <span className="absolute top-2 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle" style={{ left: inboundX + 4 }}>
              {t("Incoming")}
            </span>
            <span className="absolute top-2 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle" style={{ left: containerX + 4 }}>
              {t("Containers")}
            </span>
            <span className="absolute top-2 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle" style={{ left: outboundX + 4 }}>
              {t("Outgoing")}
            </span>

            <svg className="pointer-events-none absolute inset-0" width="100%" height={height} aria-hidden="true">
              {links.map((link) => {
                const containerPosition = containerIndex.get(link.containerId);
                if (containerPosition === undefined) return null;
                const yContainer = HEADER + containerPosition * ROW + NODE_HEIGHT / 2;
                const inboundPosition = inboundIndex.get(link.peer.address);
                const outboundPosition = outboundIndex.get(link.peer.address);
                const yPeer =
                  link.direction === "inbound"
                    ? inboundPosition !== undefined
                      ? HEADER + inboundPosition * ROW + NODE_HEIGHT / 2
                      : undefined
                    : outboundPosition !== undefined
                      ? HEADER + outboundPosition * ROW + NODE_HEIGHT / 2
                      : undefined;
                if (yPeer === undefined) return null;
                const fromX = link.direction === "inbound" ? inboundX + columnWidth + 4 : containerX + columnWidth + 4;
                const toX = link.direction === "inbound" ? containerX - 4 : outboundX - 4;
                const fromY = link.direction === "inbound" ? yPeer : yContainer;
                const toY = link.direction === "inbound" ? yContainer : yPeer;
                const mid = (fromX + toX) / 2;
                const path = `M ${fromX} ${fromY} C ${mid} ${fromY}, ${mid} ${toY}, ${toX} ${toY}`;
                return (
                  <g key={link.key}>
                    <path
                      d={path}
                      fill="none"
                      stroke={SCOPE_COLOR[link.peer.scope] ?? "var(--muted)"}
                      strokeWidth={1 + Math.min(3, link.count * 0.5)}
                      strokeOpacity={linkIsActive(link) ? 0.72 : 0.1}
                      className="transition-opacity"
                    />
                    <path
                      d={path}
                      fill="none"
                      stroke="transparent"
                      strokeWidth={14}
                      style={{ pointerEvents: "stroke", cursor: "pointer" }}
                      onMouseEnter={() => setHoveredLinkKey(link.key)}
                      onMouseLeave={() => setHoveredLinkKey(null)}
                    />
                  </g>
                );
              })}
            </svg>

            {inbound.map((peer, index) => peerNode(peer, index, inboundX))}

            {mapContainers.map((container, index) => {
              const key = `c:${container.id}`;
              const counts = containerCounts.get(container.id) ?? { inbound: 0, outbound: 0 };
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
                  style={{ left: containerX, top: HEADER + index * ROW, width: columnWidth, height: NODE_HEIGHT }}
                >
                  <EntityIcon src={containerIconURL(container.id)} name={container.name} size={28} className="rounded-md" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[11px] font-semibold text-ink">{container.name}</span>
                    <span className="block truncate font-mono text-[9px] text-subtle">
                      {container.ip || t("host network")} · {container.network || "—"}
                    </span>
                  </span>
                  <span className="flex shrink-0 items-center gap-1 rounded border border-line bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-[9px] text-subtle">
                    <ArrowDownLeft className="size-2.5" />
                    {counts.inbound}
                    <ArrowUpRight className="ml-0.5 size-2.5" />
                    {counts.outbound}
                  </span>
                </button>
              );
            })}

            {outbound.map((peer, index) => peerNode(peer, index, outboundX))}
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
            <div className="min-w-0">
              <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_88px_74px_78px_84px_58px] gap-3 border-b border-line bg-[var(--surface-2)]/50 px-3 py-2 text-[8.5px] font-semibold uppercase tracking-[0.1em] text-subtle">
                <span>{t("Container")}</span>
                <span>{t("Peer")}</span>
                <span>{t("Direction")}</span>
                <span>{t("Scope")}</span>
                <span>{t("Protocol")}</span>
                <span>{t("Ports")}</span>
                <span className="text-right">{t("Sockets")}</span>
              </div>
              <div className="max-h-[360px] divide-y divide-line/60 overflow-y-auto">
                {links.map((link) => (
                  <div
                    key={link.key}
                    onMouseEnter={() => setHoveredLinkKey(link.key)}
                    onMouseLeave={() => setHoveredLinkKey(null)}
                    className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_88px_74px_78px_84px_58px] items-center gap-3 px-3 py-2 text-[10px] transition hover:bg-[var(--surface-2)]/50"
                  >
                    <span className="min-w-0 truncate text-[10.5px] font-medium text-ink">{link.containerName}</span>
                    <span className="min-w-0 truncate font-mono text-[9.5px] text-muted" title={link.peer.containerName || link.peer.address}>
                      {link.peer.containerName ? `${link.peer.containerName} (${link.peer.address})` : link.peer.address}
                    </span>
                    <span>
                      <Badge tone={link.direction === "inbound" ? "success" : "neutral"}>
                        {link.direction === "inbound" ? t("Incoming") : t("Outgoing")}
                      </Badge>
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
            {t("Incoming peers are on the left, containers in the middle and outgoing peers on the right.")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            aria-label={t("Container")}
            className="h-9 min-w-[180px] rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
          >
            <option value="" hidden>{t("Select a container")}</option>
            <option value="all">{t("All containers")}</option>
            {containers.map((container) => (
              <option key={container.id} value={container.id}>{container.name}</option>
            ))}
          </select>
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
          <IconButton label={t("Refresh")} size="md" onClick={() => void query.refetch()} disabled={!target || query.isFetching}>
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

      <div ref={graphRef} className="mt-4">{body}</div>

      <p className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-line pt-2.5 text-[9px] text-subtle">
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: SCOPE_COLOR.container }} /> {t("Containers")}</span>
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: SCOPE_COLOR.lan }} /> {t("LAN")}</span>
        <span className="flex items-center gap-1.5"><span className="size-2 rounded-full" style={{ background: SCOPE_COLOR.internet }} /> {t("Internet")}</span>
        <span>{t("Live sockets are inspected with ss inside each container namespace. Loopback sockets are hidden.")}</span>
      </p>
    </Card>
  );
}
