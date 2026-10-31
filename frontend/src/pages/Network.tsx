import type { ReactNode } from "react";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Cable,
  ExternalLink,
  Globe2,
  Lock,
  Network as NetworkIcon,
  RefreshCw,
  Server,
  Shield,
  Signal,
  Waypoints,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { AreaChart } from "../components/ui/charts";
import { Badge, Button, Card, cn } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { formatBytes, formatSpeed, formatUptime, relativeTime } from "../lib/format";

export function NetworkPage() {
  const { snapshot, metrics, history, refresh } = useApp();
  if (!snapshot) return null;

  const { network, system } = snapshot;
  const points = history.slice(-180);
  const timestamps = points.map((point) => point.t);
  const rx = points.map((point) => point.netRx);
  const tx = points.map((point) => point.netTx);

  const primary = network.interfaces.find((iface) => iface.name === "br0") ?? network.interfaces[0];
  const vpnOnline = network.wireGuard.peers.filter((peer) => peer.online).length;
  const listening = network.services.filter((service) => service.status === "listening").length;

  return (
    <>
      <PageHeader
        eyebrow="Infrastructure"
        title="Network"
        description="Interfaces, bridges and encrypted tunnels — with a live view of everything leaving and entering the server."
        meta={
          <>
            <Badge tone="success" dot pulse>{network.interfaces.filter((iface) => iface.state === "up").length} interfaces up</Badge>
            <span className="chip">{network.hostname}.{network.domain}</span>
            {network.wireGuard.enabled ? <Badge tone="violet" dot>{vpnOnline} VPN peers</Badge> : null}
          </>
        }
        actions={
          <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
            Refresh
          </Button>
        }
      />

      <div className="mb-4 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Stat icon={<ArrowDownToLine className="size-4" />} label="Download" value={formatSpeed(metrics?.netRx ?? 0)} hint={`${formatBytes(primary?.rxTotal ?? 0)} received`} />
        <Stat icon={<ArrowUpFromLine className="size-4" />} label="Upload" value={formatSpeed(metrics?.netTx ?? 0)} hint={`${formatBytes(primary?.txTotal ?? 0)} sent`} />
        <Stat icon={<Signal className="size-4" />} label="Link speed" value={primary ? `${(primary.speedMbps / 1000).toFixed(1)} GbE` : "—"} hint={primary?.description ?? ""} />
        <Stat icon={<Server className="size-4" />} label="Services" value={String(listening)} hint={`${network.services.length} exposed ports`} />
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
        <Card className="card-pad xl:col-span-8">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 className="text-[13px] font-semibold text-ink">Traffic</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">Aggregated across all interfaces · last 6 minutes</p>
            </div>
            <div className="flex items-center gap-3 text-[10px]">
              <span className="flex items-center gap-1.5 text-muted"><span className="size-1.5 rounded-full bg-[var(--info)]" /> Download</span>
              <span className="flex items-center gap-1.5 text-muted"><span className="size-1.5 rounded-full bg-[var(--violet)]" /> Upload</span>
            </div>
          </div>
          <AreaChart
            series={[
              { values: rx, color: "var(--info)", label: "Download" },
              { values: tx, color: "var(--violet)", label: "Upload" },
            ]}
            timestamps={timestamps}
            height={230}
            showAxis
            formatValue={(value) => formatSpeed(value)}
            domain={[0, Math.max(1_000_000, ...rx, ...tx) * 1.2]}
            className="mt-4"
          />
          <div className="mt-4 grid grid-cols-2 divide-x divide-line border-t border-line pt-4 sm:grid-cols-4">
            <Mini label="Total received" value={formatBytes(primary?.rxTotal ?? 0)} />
            <Mini label="Total sent" value={formatBytes(primary?.txTotal ?? 0)} />
            <Mini label="Gateway" value={network.gateway} />
            <Mini label="DNS" value={network.dns[0] ?? "—"} />
          </div>
        </Card>

        <Card className="card-pad xl:col-span-4">
          <div className="flex items-center gap-2">
            <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">
              <Globe2 className="size-3.5" />
            </span>
            <h2 className="text-[13px] font-semibold text-ink">Connection</h2>
          </div>
          <div className="mt-4 space-y-3">
            <Row label="Hostname" value={`${network.hostname}.${network.domain}`} />
            <Row label="Gateway" value={network.gateway} />
            <Row label="Primary IP" value={primary?.ip ?? "—"} />
            <Row label="DNS" value={network.dns.join(", ")} />
            <Row label="Interface" value={primary?.name ?? "—"} />
            <Row label="MAC" value={primary?.mac ?? "—"} />
          </div>
          <div className="mt-4 rounded-xl border border-line bg-[var(--surface-2)]/50 p-3">
            <p className="flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">
              <Lock className="size-3" /> WireGuard
            </p>
            <div className="mt-2 flex items-center justify-between">
              <span className="text-[11px] text-soft">{network.wireGuard.enabled ? "Tunnel active" : "Disabled"}</span>
              <Badge tone={network.wireGuard.enabled ? "success" : "neutral"} dot>
                port {network.wireGuard.port}
              </Badge>
            </div>
            <p className="mt-1.5 text-[10px] text-subtle">{vpnOnline} of {network.wireGuard.peers.length} peers connected</p>
          </div>
        </Card>

        {network.interfaces.map((iface) => {
          const isLive = iface.name === "br0";
          const down = isLive ? (metrics?.netRx ?? iface.rxBps) : iface.rxBps;
          const up = isLive ? (metrics?.netTx ?? iface.txBps) : iface.txBps;
          return (
            <Card key={iface.name} className="card-pad xl:col-span-6">
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-center gap-2.5">
                  <span className="grid size-8 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-muted">
                    {iface.name.startsWith("wg") ? <Shield className="size-4" /> : iface.bridge ? <Waypoints className="size-4" /> : <Cable className="size-4" />}
                  </span>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-mono text-[12px] font-semibold text-ink">{iface.name}</h3>
                      <Badge tone={iface.state === "up" ? "success" : "neutral"} dot>{iface.state}</Badge>
                    </div>
                    <p className="mt-0.5 text-[10px] text-subtle">{iface.description}</p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="num text-[12px] text-soft">{iface.speedMbps ? `${(iface.speedMbps / 1000).toFixed(1)} GbE` : "—"}</p>
                  <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{iface.name.startsWith("wg") ? "tunnel" : iface.bridge ? "bridge" : iface.bond ? "bond" : "physical"}</p>
                </div>
              </div>

              <div className="mt-4 grid grid-cols-2 gap-4">
                <div>
                  <p className="flex items-center gap-1 text-[9px] uppercase tracking-[0.1em] text-subtle"><ArrowDownToLine className="size-2.5 text-[var(--info)]" /> Down</p>
                  <p className="num mt-1 text-[15px] font-semibold text-ink">{formatSpeed(down)}</p>
                  <p className="mt-0.5 text-[9px] text-subtle">{formatBytes(iface.rxTotal)} total</p>
                </div>
                <div>
                  <p className="flex items-center gap-1 text-[9px] uppercase tracking-[0.1em] text-subtle"><ArrowUpFromLine className="size-2.5 text-[var(--violet)]" /> Up</p>
                  <p className="num mt-1 text-[15px] font-semibold text-ink">{formatSpeed(up)}</p>
                  <p className="mt-0.5 text-[9px] text-subtle">{formatBytes(iface.txTotal)} total</p>
                </div>
              </div>

              <div className="mt-4 flex flex-wrap gap-1.5">
                {iface.ip ? <span className="chip font-mono text-[9.5px]">{iface.ip}</span> : null}
                {iface.mac ? <span className="chip font-mono text-[9.5px]">{iface.mac}</span> : null}
                {iface.bridge ? <span className="chip text-[9.5px]">bridge {iface.bridge}</span> : null}
                {iface.gateway ? <span className="chip text-[9.5px]">gw {iface.gateway}</span> : null}
              </div>
            </Card>
          );
        })}

        <Card className="xl:col-span-7">
          <div className="flex items-center justify-between border-b border-line px-5 py-3.5">
            <div>
              <h2 className="text-[13px] font-semibold text-ink">WireGuard peers</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">Encrypted tunnels to your devices and remote sites</p>
            </div>
            <Badge tone="violet" dot>{vpnOnline} online</Badge>
          </div>
          <div className="divide-y divide-line/60">
            {network.wireGuard.peers.map((peer) => (
              <div key={peer.name} className="flex flex-wrap items-center gap-3 px-5 py-3">
                <span className={cn("status-dot", peer.online ? "live" : "muted")} />
                <div className="min-w-0 flex-1">
                  <p className="text-[11.5px] font-medium text-soft">{peer.name}</p>
                  <p className="truncate font-mono text-[9.5px] text-subtle">{peer.endpoint} · {peer.allowedIps.join(", ")}</p>
                </div>
                <div className="hidden text-right sm:block">
                  <p className="num text-[10px] text-muted">{formatBytes(peer.rx)} ↓ / {formatBytes(peer.tx)} ↑</p>
                  <p className="text-[9px] text-subtle">handshake {relativeTime(peer.latestHandshake)}</p>
                </div>
                <span className="font-mono text-[9px] text-subtle" title="Public key">{peer.publicKey}</span>
              </div>
            ))}
          </div>
        </Card>

        <Card className="xl:col-span-5">
          <div className="flex items-center justify-between border-b border-line px-5 py-3.5">
            <div>
              <h2 className="text-[13px] font-semibold text-ink">Network services</h2>
              <p className="mt-0.5 text-[10.5px] text-subtle">Listening ports on this server</p>
            </div>
            <Badge tone="success" dot>{listening}</Badge>
          </div>
          <div className="divide-y divide-line/60">
            {network.services.map((service) => (
              <div key={`${service.name}-${service.port}`} className="flex items-center gap-3 px-5 py-2.5">
                <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-[9px] text-subtle">
                  {service.protocol === "udp" ? "U" : "T"}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[11px] text-soft">{service.name}</p>
                  <p className="font-mono text-[9px] text-subtle">{service.protocol.toUpperCase()} · port {service.port}</p>
                </div>
                {service.url ? (
                  <button
                    type="button"
                    onClick={() => window.open(service.url, "_blank", "noopener")}
                    className="grid size-6 place-items-center rounded-md text-subtle hover:bg-[var(--surface-3)] hover:text-ink"
                    aria-label={`Open ${service.name}`}
                  >
                    <ExternalLink className="size-3" />
                  </button>
                ) : null}
                <span className="status-dot" />
              </div>
            ))}
          </div>
          <div className="border-t border-line px-5 py-3">
            <div className="flex items-center justify-between text-[10px] text-subtle">
              <span className="flex items-center gap-1"><NetworkIcon className="size-3" /> Host uptime</span>
              <span className="num text-soft">{formatUptime(system.uptime)}</span>
            </div>
            <p className="mt-1.5 font-mono text-[9px] text-subtle">{system.kernel}</p>
          </div>
        </Card>
      </div>
    </>
  );
}

function Stat({ icon, label, value, hint }: { icon: ReactNode; label: string; value: string; hint: string }) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className="num mt-3 text-[21px] font-semibold leading-none text-ink">{value}</p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-[11px]">
      <span className="text-muted">{label}</span>
      <span className="num max-w-[62%] truncate text-right text-soft">{value}</span>
    </div>
  );
}

function Mini({ label, value }: { label: string; value: string }) {
  return (
    <div className="px-3 first:pl-0 last:pr-0">
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="num mt-1 truncate text-[11px] text-soft">{value}</p>
    </div>
  );
}
