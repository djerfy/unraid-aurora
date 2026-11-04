import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Camera, Database, Play, Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { Modal } from "../components/ui/overlays";
import { Badge, Button, Card, EmptyState, IconButton, Progress, Switch, cn } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { useToast } from "../app/ToastProvider";
import { api } from "../lib/api";
import { formatBytes, formatDateTime } from "../lib/format";

export function ZFSView() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [dataset, setDataset] = useState<string | null>(null);
  const [snapshotName, setSnapshotName] = useState("");
  const [creating, setCreating] = useState(false);
  const [datasetName, setDatasetName] = useState("");
  const [compression, setCompression] = useState("lz4");
  const [recordsize, setRecordsize] = useState("128K");
  const [quotaGB, setQuotaGB] = useState(0);
  const [readOnly, setReadOnly] = useState(false);
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

  const createDataset = useMutation({
    mutationFn: () => api.createZFSDataset({
      name: datasetName.trim(),
      compression,
      quotaBytes: Math.max(0, quotaGB) * 1024 * 1024 * 1024,
      recordsize,
      readOnly,
    }),
    onSuccess: () => {
      setCreating(false);
      setDatasetName("");
      void queryClient.invalidateQueries({ queryKey: ["zfs-pools"] });
      toast({ kind: "success", title: t("Dataset created") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not create the dataset"), description: error.message }),
  });
  const updateDataset = useMutation({
    mutationFn: () => api.updateZFSDataset({
      name: dataset ?? "",
      compression,
      quotaBytes: Math.max(0, quotaGB) * 1024 * 1024 * 1024,
      recordsize,
      readOnly,
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["zfs-pools"] });
      toast({ kind: "success", title: t("Dataset updated") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not update the dataset"), description: error.message }),
  });
  const removeDataset = useMutation({
    mutationFn: () => api.deleteZFSDataset(dataset ?? "", true),
    onSuccess: () => {
      setDataset(null);
      void queryClient.invalidateQueries({ queryKey: ["zfs-pools"] });
      toast({ kind: "success", title: t("Dataset deleted") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not delete the dataset"), description: error.message }),
  });
  const rollback = useMutation({
    mutationFn: (name: string) => api.rollbackZFSSnapshot(dataset ?? "", name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["zfs-snapshots", dataset] });
      toast({ kind: "success", title: t("Snapshot rolled back") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not roll back the snapshot"), description: error.message }),
  });
  const clone = useMutation({
    mutationFn: ({ name, target }: { name: string; target: string }) => api.cloneZFSSnapshot(dataset ?? "", name, target),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["zfs-pools"] });
      toast({ kind: "success", title: t("Snapshot cloned") });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Could not clone the snapshot"), description: error.message }),
  });

  const selectDataset = (name: string) => {
    setDataset(name);
    setSnapshotName("");
    const item = pools.data?.flatMap((pool) => pool.datasets).find((entry) => entry.name === name);
    setCompression(item?.compression || "lz4");
    setRecordsize(item?.recordsize || "128K");
    setQuotaGB(item?.quota ? item.quota / 1024 / 1024 / 1024 : 0);
    setReadOnly(Boolean(item?.readOnly));
  };

  return (
    <>
      <div className="mb-4 flex items-center justify-between gap-2">
        <p className="text-[10.5px] text-subtle">{t("Pools, datasets and snapshots on this Unraid host.")}</p>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" icon={<Plus className="size-3.5" />} onClick={() => setCreating(true)}>{t("New dataset")}</Button>
          <Button variant="secondary" size="sm" icon={<RefreshCw className="size-3.5" />} onClick={() => void pools.refetch()}>{t("Refresh")}</Button>
        </div>
      </div>

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
                        onClick={() => selectDataset(item.name)}
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
                <div className="mt-3 grid grid-cols-2 gap-2">
                  <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Compression")}</span>
                    <select value={compression} onChange={(event) => setCompression(event.target.value)} className="h-8 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[10px] text-soft outline-none">
                      {["off", "lz4", "zstd", "zle", "gzip"].map((value) => <option key={value} value={value}>{value}</option>)}
                    </select>
                  </label>
                  <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Recordsize")}</span>
                    <select value={recordsize} onChange={(event) => setRecordsize(event.target.value)} className="h-8 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[10px] text-soft outline-none">
                      {["4K", "8K", "16K", "32K", "64K", "128K", "256K", "512K", "1M"].map((value) => <option key={value} value={value}>{value}</option>)}
                    </select>
                  </label>
                  <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Quota (GiB, 0 = none)")}</span>
                    <input type="number" min={0} value={quotaGB} onChange={(event) => setQuotaGB(Number(event.target.value))} className="h-8 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2 font-mono text-[10px] text-ink outline-none" />
                  </label>
                  <label className="flex items-end gap-2 pb-1 text-[9.5px] text-muted"><Switch checked={readOnly} onChange={setReadOnly} label={t("Read-only dataset")} />{t("Read-only")}</label>
                </div>
                <div className="mt-2 flex gap-2">
                  <Button size="sm" variant="secondary" icon={<Save className="size-3.5" />} loading={updateDataset.isPending} onClick={() => updateDataset.mutate()}>{t("Save properties")}</Button>
                  <Button size="sm" variant="danger" icon={<Trash2 className="size-3.5" />} loading={removeDataset.isPending} onClick={() => { if (window.confirm(t("Delete dataset {name} and its snapshots?", { name: dataset }))) removeDataset.mutate(); }}>{t("Delete dataset")}</Button>
                </div>
                <div className="mt-4 flex gap-2">
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
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => {
                            if (window.confirm(t("Roll back to snapshot {name}? Newer snapshots and data will be lost.", { name: snapshot.name }))) rollback.mutate(snapshot.name);
                          }}
                        >
                          {t("Rollback")}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => {
                            const target = window.prompt(t("Clone {name} into a new dataset:", { name: snapshot.name }), `${dataset}-clone`);
                            if (target) clone.mutate({ name: snapshot.name, target });
                          }}
                        >
                          {t("Clone")}
                        </Button>
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

      <Modal
        open={creating}
        onClose={() => setCreating(false)}
        title={t("New ZFS dataset")}
        description={t("Create a dataset inside an existing pool. The parent pool must already exist.")}
        footer={
          <>
            <Button variant="secondary" onClick={() => setCreating(false)} disabled={createDataset.isPending}>{t("Cancel")}</Button>
            <Button variant="primary" loading={createDataset.isPending} disabled={!datasetName.trim()} onClick={() => createDataset.mutate()}>{t("Create dataset")}</Button>
          </>
        }
      >
        <div className="space-y-3">
          <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Dataset name")}</span>
            <input value={datasetName} onChange={(event) => setDatasetName(event.target.value)} placeholder="tank/media" className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none" />
          </label>
          <div className="grid grid-cols-2 gap-3">
            <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Compression")}</span>
              <select value={compression} onChange={(event) => setCompression(event.target.value)} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none">
                {["off", "lz4", "zstd", "zle", "gzip"].map((value) => <option key={value} value={value}>{value}</option>)}
              </select>
            </label>
            <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Recordsize")}</span>
              <select value={recordsize} onChange={(event) => setRecordsize(event.target.value)} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[10.5px] text-soft outline-none">
                {["4K", "8K", "16K", "32K", "64K", "128K", "256K", "512K", "1M"].map((value) => <option key={value} value={value}>{value}</option>)}
              </select>
            </label>
          </div>
          <label className="block"><span className="mb-1 block text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">{t("Quota (GiB, 0 = none)")}</span>
            <input type="number" min={0} value={quotaGB} onChange={(event) => setQuotaGB(Number(event.target.value))} className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none" />
          </label>
          <label className="flex items-center gap-2 text-[10px] text-muted"><Switch checked={readOnly} onChange={setReadOnly} label={t("Read-only dataset")} />{t("Read-only dataset")}</label>
        </div>
      </Modal>
    </>
  );
}
