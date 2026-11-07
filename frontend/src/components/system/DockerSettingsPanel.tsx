import { useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Container, RefreshCw, Save } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { Badge, Button, Card, Switch, cn } from "../ui/primitives";
import type { DockerSettings } from "../../lib/types";

const LOG_SIZES = ["10m", "20m", "50m", "100m", "500m", "1g"];

function Row({
  label,
  description,
  children,
}: {
  label: string;
  description: string;
  children: ReactNode;
}) {
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

function Field({ value, onChange, placeholder, className }: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}) {
  return (
    <input
      value={value}
      onChange={(event) => onChange(event.target.value)}
      placeholder={placeholder}
      className={cn("h-9 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 text-[11.5px] text-ink outline-none transition focus:border-[var(--accent-line)]", className)}
    />
  );
}

function Choice({ value, onChange, options }: {
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className="h-9 rounded-[7px] border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
    >
      {options.map((option) => (
        <option key={option.value} value={option.value}>{option.label}</option>
      ))}
    </select>
  );
}

export function DockerSettingsPanel() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<DockerSettings | null>(null);

  const query = useQuery({
    queryKey: ["docker-settings"],
    queryFn: api.dockerSettings,
    staleTime: 0,
  });

  useEffect(() => {
    if (query.data) setDraft(query.data);
  }, [query.data]);

  const save = useMutation({
    mutationFn: (value: DockerSettings) => api.saveDockerSettings(value),
    onSuccess: (saved) => {
      setDraft(saved);
      queryClient.setQueryData(["docker-settings"], saved);
      toast({ kind: "success", title: t("Docker settings saved"), description: t("Restart Docker or the array to apply storage and service changes.") });
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Docker settings could not be saved"), description: error.message });
    },
  });

  if (query.isError) {
    return <Card className="card-pad"><p className="text-[10.5px] text-[var(--danger)]">{(query.error as Error).message}</p></Card>;
  }
  if (query.isLoading || !draft) {
    return <Card className="card-pad"><p className="py-8 text-center text-[10.5px] text-muted">{t("Loading Docker settings…")}</p></Card>;
  }

  const update = (patch: Partial<DockerSettings>) => setDraft((current) => (current ? { ...current, ...patch } : current));
  const imageMode = draft.imageType === "folder" ? "folder" : "image";

  return (
    <div className="space-y-4">
      <Card className="card-pad">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-xl border border-line bg-[var(--surface-2)] text-muted">
              <Container className="size-4" />
            </span>
            <div className="min-w-0">
              <h2 className="text-[13px] font-semibold text-ink">{t("Docker")}</h2>
              <p className="mt-0.5 text-[10.5px] leading-relaxed text-subtle">
                {t("Unraid Docker daemon settings from /boot/config/docker.cfg. Storage and service changes apply after Docker or the array restarts.")}
              </p>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={draft.running ? "success" : "neutral"} dot>{draft.running ? t("Running") : t("Stopped")}</Badge>
            {draft.version ? <Badge tone="neutral">{t("Docker {version}", { version: draft.version })}</Badge> : null}
            {draft.storageDriver ? <Badge tone="neutral">{draft.storageDriver}</Badge> : null}
            <Button size="sm" variant="secondary" icon={<RefreshCw className={cn("size-3.5", query.isFetching && "animate-spin")} />} onClick={() => void query.refetch()}>
              {t("Refresh")}
            </Button>
            <Button size="sm" variant="primary" icon={<Save className="size-3.5" />} loading={save.isPending} onClick={() => save.mutate(draft)}>
              {t("Save Docker settings")}
            </Button>
          </div>
        </div>
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Service")}</h2>
        <div className="mt-2 divide-y divide-line">
          <Row label={t("Enable Docker")} description={t("Start the Docker service with the array.")}>
            <Switch checked={draft.enabled} onChange={(enabled) => update({ enabled })} label={t("Enable Docker")} />
          </Row>
          <Row label={t("Docker stop timeout")} description={t("Seconds Docker waits before killing a container.")}>
            <input type="number" min={1} max={3600} value={draft.stopTimeout} onChange={(event) => update({ stopTimeout: Number(event.target.value) })} className="h-9 w-28 rounded-[7px] border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[11px] text-ink outline-none focus:border-[var(--accent-line)]" />
          </Row>
          <Row label={t("Docker PID limit")} description={t("Maximum number of processes per container. Empty means unlimited.")}>
            <input type="number" min={0} value={draft.pidLimit || ""} placeholder="2048" onChange={(event) => update({ pidLimit: Math.max(0, Number(event.target.value)) })} className="h-9 w-28 rounded-[7px] border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[11px] text-ink outline-none focus:border-[var(--accent-line)]" />
          </Row>
          <Row label={t("Docker version")} description={t("Version reported by the running Docker daemon.")}>
            <span className="font-mono text-[11px] text-soft">{draft.version || "—"}</span>
          </Row>
        </div>
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Storage")}</h2>
        <div className="mt-2 divide-y divide-line">
          <Row label={t("Docker data-root")} description={t("btrfs vDisk, xfs vDisk or a directory.")}>
            <Choice value={draft.imageType} onChange={(imageType) => update({ imageType })} options={[
              { value: "", label: t("btrfs vDisk") },
              { value: "xfs", label: t("xfs vDisk") },
              { value: "folder", label: t("Directory") },
            ]} />
          </Row>
          {imageMode === "image" ? (
            <Row label={t("Docker vDisk size")} description={t("Size of the Docker image in GB.")}>
              <input type="number" min={1} value={draft.imageSize} onChange={(event) => update({ imageSize: Number(event.target.value) })} className="h-9 w-28 rounded-[7px] border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[11px] text-ink outline-none focus:border-[var(--accent-line)]" />
            </Row>
          ) : null}
          <Row label={imageMode === "folder" ? t("Docker directory") : t("Docker vDisk location")} description={imageMode === "folder" ? t("Absolute /mnt path ending with /.") : t("Absolute /mnt path ending with docker.img or docker-xfs.img.")}>
            <Field value={draft.imageFile} onChange={(imageFile) => update({ imageFile })} placeholder={imageMode === "folder" ? "/mnt/user/system/docker/" : "/mnt/user/system/docker/docker.img"} className="w-80 max-w-full font-mono text-[10.5px]" />
          </Row>
          <Row label={t("Default appdata storage location")} description={t("Default host path for container configuration.")}>
            <Field value={draft.appConfigPath} onChange={(appConfigPath) => update({ appConfigPath })} placeholder="/mnt/user/appdata/" className="w-80 max-w-full font-mono text-[10.5px]" />
          </Row>
          <Row label={t("/unraid share path")} description={t("Optional /unraid mount exposed to containers.")}>
            <Field value={draft.appUnraidPath} onChange={(appUnraidPath) => update({ appUnraidPath })} placeholder="/mnt/user/" className="w-80 max-w-full font-mono text-[10.5px]" />
          </Row>
          {imageMode === "folder" ? (
            <Row label={t("Docker storage driver")} description={t("Storage driver for the directory data-root.")}>
              <Choice value={draft.backingFs} onChange={(backingFs) => update({ backingFs })} options={[
                { value: "overlay2", label: "overlay2" },
                { value: "native", label: "native" },
              ]} />
            </Row>
          ) : null}
        </div>
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Log rotation")}</h2>
        <div className="mt-2 divide-y divide-line">
          <Row label={t("Enable log rotation")} description={t("Limit the size and number of Docker log files.")}>
            <Switch checked={draft.logRotation} onChange={(logRotation) => update({ logRotation })} label={t("Enable log rotation")} />
          </Row>
          <Row label={t("Docker LOG maximum file size")} description={t("Maximum size of each container log file.")}>
            <Choice value={draft.logSize} onChange={(logSize) => update({ logSize })} options={LOG_SIZES.map((size) => ({ value: size, label: size }))} />
          </Row>
          <Row label={t("Docker LOG number of files")} description={t("Number of rotated log files kept per container.")}>
            <Choice value={String(draft.logFiles)} onChange={(value) => update({ logFiles: Number(value) })} options={[1, 2, 3].map((count) => ({ value: String(count), label: String(count) }))} />
          </Row>
        </div>
      </Card>

      <Card className="card-pad">
        <h2 className="text-[13px] font-semibold text-ink">{t("Advanced")}</h2>
        <div className="mt-2 divide-y divide-line">
          <Row label={t("Enable container table readmore")} description={t("Show the expandable container details in the official webGUI.")}>
            <Switch checked={draft.readMore} onChange={(readMore) => update({ readMore })} label={t("Enable container table readmore")} />
          </Row>
          <Row label={t("Template authoring mode")} description={t("Keep Docker templates editable for authoring.")}>
            <Switch checked={draft.authoringMode} onChange={(authoringMode) => update({ authoringMode })} label={t("Template authoring mode")} />
          </Row>
          <Row label={t("Docker custom network type")} description={t("macvlan or ipvlan for custom Docker networks.")}>
            <Choice value={draft.networkType} onChange={(networkType) => update({ networkType })} options={[
              { value: "", label: "macvlan" },
              { value: "1", label: "ipvlan" },
            ]} />
          </Row>
          <Row label={t("Host access to custom networks")} description={t("Allow the host to reach containers on custom networks.")}>
            <Switch checked={draft.allowAccess} onChange={(allowAccess) => update({ allowAccess })} label={t("Host access to custom networks")} />
          </Row>
          <Row label={t("Preserve user defined networks")} description={t("Keep Docker networks created outside Unraid.")}>
            <Switch checked={draft.preserveUserNetworks} onChange={(preserveUserNetworks) => update({ preserveUserNetworks })} label={t("Preserve user defined networks")} />
          </Row>
        </div>
      </Card>
    </div>
  );
}
