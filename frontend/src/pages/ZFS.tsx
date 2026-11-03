import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Camera, Database, Play, Plus, RefreshCw, Trash2 } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Button, Card, EmptyState, IconButton, Progress, cn } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { useToast } from "../app/ToastProvider";
import { api } from "../lib/api";
import { formatBytes, formatDateTime } from "../lib/format";

export function ZFSPage() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [dataset, setDataset] = useState<string | null>(null);
  const [snapshotName, setSnapshotName] = useState("");
  const pools = useQuery({ queryKey: ["zfs-pools"], queryFn: api.zfsPools, refetchInterval: 30_000 });
  const snapshots = useQuery({
    queryKey: ["zfs-snapshots", dataset],
    queryFn: () => api.zfsSnapshots(dataset ?? ""),
    enabled: Boolean(dataset),
  });

  const scrub = useMutation({
    mutationFn: (pool: string) => api.scrubZFSPool(pool),
    onSuccess: () => toast({ kind: "success", title: t("Scrub started") }),
    onError: (error: Error) => toast({ kind: "error", title: t("Could not start the scrub"), description: error.message }),
  });
  const create = useMutation({
    mutationFn: () => api.createZFSSnapshot(dataset ?? "", snapshotName.trim()),
    onSuccess: () => {
      setSnapshotName("");
      void queryClient.invalidateQueries({ queryKey: ["zfs-snapshots", dataset] });
      toast({ kind: "success", title: t("Snapshot created") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not create the snapshot"), description: error.message }),
  });
  const remove = useMutation({
    mutationFn: (name: string) => api.deleteZFSSnapshot(dataset ?? "", name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["zfs-snapshots", dataset] });
      toast({ kind: "success", title: t("Snapshot deleted") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not delete the snapshot"), description: error.message }),
  });

  return (
    <>
      <PageHeader
        eyebrow={t("Infrastructure")}
        title={t("ZFS")}
        description={t("Pools, datasets and snapshots on this Unraid host.")}
        meta={<Badge tone="neutral" dot>{t("{count} pools", { count: pools.data?.length ?? 0 })}</Badge>}
        actions={<Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={() => void pools.refetch()}>{t("Refresh")}</Button>}
      />

      {pools.isLoading ? (
        <p className="text-[11px] text-muted">{t("Loading ZFS data…")}</p>
      ) : pools.isError ? (
        <Card><EmptyState icon={<Database className="size-5" />} title={t("ZFS is unavailable")} description={(pools.error as Error).message} /></Card>
      ) : !pools.data?.length ? (
        <Card><EmptyState icon={<Database className="size-5" />} title={t("No ZFS pools found")} description={t("ZFS pools and datasets appear here when the host exposes the zpool and zfs commands.")} /></Card>
      ) : (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
          <div className="space-y-4">
            {pools.data.map((pool) => {
              const usage = pool.size ? (pool.allocated / pool.size) * 100 : 0;
              return (
                <Card key={pool.name} className="card-pad">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <h2 className="flex items-center gap-2 text-[14px] font-semibold text-ink">
                        <Database className="size-4 text-[var(--accent)]" /> {pool.name}
                      </h2>
                      <p className="mt-1 text-[10px] text-subtle">{formatBytes(pool.allocated)} / {formatBytes(pool.size)} · {t("free")} {formatBytes(pool.free)}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge tone={pool.health === "ONLINE" ? "success" : "warning"}>{pool.health}</Badge>
                      <Button size="sm" variant="secondary" icon={<Play className="size-3" />} loading={scrub.isPending} onClick={() => scrub.mutate(pool.name)}>{t("Scrub")}</Button>
                    </div>
                  </div>
                  <Progress value={usage} height={5} className="mt-3" />
                  <div className="mt-3 divide-y divide-line/60 overflow-hidden rounded-xl border border-line">
                    {pool.datasets.map((item) => (
                      <button
                        key={item.name}
                        type="button"
                        onClick={() => { setDataset(item.name); setSnapshotName(""); }}
                        className={cn("flex w-full items-center gap-3 px-3 py-2 text-left text-[10.5px] transition hover:bg-[var(--surface-2)]", dataset === item.name && "bg-[var(--accent-soft)]")}
                      >
                        <span className="min-w-0 flex-1 truncate font-mono text-soft">{item.name}</span>
                        <span className="num text-[9.5px] text-subtle">{formatBytes(item.used)}</span>
                        <span className="hidden truncate font-mono text-[9px] text-subtle sm:block">{item.mountpoint}</span>
                      </button>
                    ))}
                  </div>
                </Card>
              );
            })}
          </div>

          <Card className="card-pad">
            <div className="flex items-center justify-between gap-2">
              <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink"><Camera className="size-4 text-[var(--violet)]" /> {t("Snapshots")}</h2>
              {dataset ? <span className="truncate font-mono text-[9px] text-subtle">{dataset}</span> : null}
            </div>
            {!dataset ? (
              <p className="py-12 text-center text-[10.5px] text-muted">{t("Select a dataset to manage its snapshots.")}</p>
            ) : (
              <>
                <div className="mt-3 flex gap-2">
                  <input
                    value={snapshotName}
                    onChange={(event) => setSnapshotName(event.target.value)}
                    placeholder={t("Snapshot name")}
                    aria-label={t("Snapshot name")}
                    className="h-9 min-w-0 flex-1 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none focus:border-[var(--accent-line)]"
                  />
                  <Button size="sm" variant="secondary" icon={<Plus className="size-3.5" />} loading={create.isPending} disabled={!snapshotName.trim()} onClick={() => create.mutate()}>{t("Create")}</Button>
                </div>
                {snapshots.isLoading ? (
                  <p className="mt-3 text-[10px] text-muted">{t("Loading snapshots…")}</p>
                ) : snapshots.isError ? (
                  <p className="mt-3 text-[10px] text-[var(--danger)]">{(snapshots.error as Error).message}</p>
                ) : snapshots.data?.length ? (
                  <div className="mt-3 overflow-hidden rounded-xl border border-line">
                    {snapshots.data.map((snapshot) => (
                      <div key={snapshot.name} className="flex items-center gap-2 border-b border-line/60 px-3 py-2 last:border-0">
                        <span className="min-w-0 flex-1">
                          <span className="block truncate font-mono text-[10.5px] text-soft">{snapshot.name}</span>
                          <span className="block text-[9px] text-subtle">{snapshot.created ? formatDateTime(snapshot.created * 1000) : "—"} · {formatBytes(snapshot.used)}</span>
                        </span>
                        <IconButton
                          label={t("Delete snapshot")}
                          size="sm"
                          onClick={() => {
                            if (window.confirm(t("Delete snapshot {name}?", { name: snapshot.name }))) remove.mutate(snapshot.name);
                          }}
                        >
                          <Trash2 className="size-3.5" />
                        </IconButton>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="mt-3 text-[10px] text-subtle">{t("No snapshots for this dataset.")}</p>
                )}
              </>
            )}
          </Card>
        </div>
      )}
    </>
  );
}
