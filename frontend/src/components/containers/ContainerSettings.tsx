import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, FolderPlus, Plus, Save, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import type { Container, ContainerConfig, ContainerEnv, ContainerVolume } from "../../lib/types";
import { useI18n } from "../../i18n";
import { EntityIcon, containerIconURL } from "../ui/entity-icon";
import { Modal } from "../ui/overlays";
import { Badge, Button, IconButton, Switch } from "../ui/primitives";
import {
  EmptyLine,
  NumberField,
  SectionHeader,
  validateContainerFields,
} from "./container-form";
import { useToast } from "../../app/ToastProvider";

const RESTART_POLICIES = ["unless-stopped", "always", "no", "on-failure"];
const PROTOCOLS = ["tcp", "udp", "sctp"];

export function ContainerSettingsModal({
  container,
  onClose,
}: {
  container: Container | null;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const { toast } = useToast();
  const [draft, setDraft] = useState<ContainerConfig | null>(null);
  const [error, setError] = useState("");

  const configQuery = useQuery({
    queryKey: ["container-config", container?.id],
    queryFn: () => api.containerConfig(container?.id ?? ""),
    enabled: Boolean(container),
    staleTime: 0,
  });

  useEffect(() => {
    if (configQuery.data) {
      setDraft({
        ...configQuery.data,
        ports: configQuery.data.ports ?? [],
        volumes: configQuery.data.volumes ?? [],
        env: configQuery.data.env ?? [],
      });
      setError("");
    }
  }, [configQuery.data]);

  useEffect(() => {
    if (!container) {
      setDraft(null);
      setError("");
    }
  }, [container]);

  const save = useMutation({
    mutationFn: (cfg: ContainerConfig) => api.updateContainerConfig(container?.id ?? "", cfg),
    onSuccess: () => {
      toast({
        kind: "success",
        title: t("Container updated"),
        description: t("{name} was recreated with the new settings.", { name: container?.name ?? "" }),
      });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      void queryClient.invalidateQueries({ queryKey: ["container-config", container?.id] });
      onClose();
    },
    onError: (err: Error) => {
      setError(err.message);
      toast({ kind: "error", title: t("Update failed"), description: err.message });
    },
  });

  if (!container) return null;

  const update = (patch: Partial<ContainerConfig>) => {
    setDraft((current) => (current ? { ...current, ...patch } : current));
    setError("");
  };
  const updateVolume = (index: number, patch: Partial<ContainerVolume>) => {
    if (!draft) return;
    update({
      volumes: draft.volumes.map((volume, i) => (i === index ? { ...volume, ...patch } : volume)),
    });
  };
  const updateEnv = (index: number, patch: Partial<ContainerEnv>) => {
    if (!draft) return;
    update({ env: draft.env.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)) });
  };

  const submit = () => {
    if (!draft) return;
    const validation = validateContainerFields(draft, t);
    if (validation) {
      setError(validation);
      return;
    }
    save.mutate(draft);
  };

  return (
    <Modal
      open={Boolean(container)}
      onClose={onClose}
      title={t("Settings · {name}", { name: container.name })}
      description={t("Recreate this container with new paths, ports, environment variables and restart policy.")}
      size="xl"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={save.isPending}>
            {t("Cancel")}
          </Button>
          <Button
            variant="primary"
            icon={<Save className="size-3.5" />}
            loading={save.isPending}
            disabled={!draft}
            onClick={submit}
          >
            {t("Apply & recreate")}
          </Button>
        </>
      }
    >
      {configQuery.isLoading ? (
        <div className="space-y-3 py-2">
          <div className="skeleton h-16 rounded-xl" />
          <div className="skeleton h-32 rounded-xl" />
          <div className="skeleton h-24 rounded-xl" />
        </div>
      ) : configQuery.isError ? (
        <div className="rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] p-3 text-[11px] text-[var(--danger)]">
          {(configQuery.error as Error).message}
        </div>
      ) : draft ? (
        <div className="space-y-5">
          <div className="flex items-center gap-3 rounded-xl border border-line bg-[var(--surface-2)]/60 p-3">
            <EntityIcon
              src={containerIconURL(container.id)}
              name={container.name}
              size={38}
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-[12px] font-semibold text-ink">{container.name}</p>
              <p className="truncate font-mono text-[9.5px] text-subtle">{draft.image || container.image}</p>
            </div>
            <Badge tone="neutral">{draft.networkMode || "bridge"}</Badge>
          </div>

          <div className="flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] p-3 text-[10.5px] leading-relaxed text-muted">
            <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-[var(--warning)]" />
            <span>
              {t("Applying changes stops and recreates the container. Its current ID and uptime reset. Only the paths listed below are kept; other bind mounts are removed.")}
            </span>
          </div>

          {/* Paths ------------------------------------------------------- */}
          <section>
            <SectionHeader
              title={t("Paths")}
              hint={t("Bind mounts, host path to container path")}
              onAdd={() =>
                update({
                  volumes: [...draft.volumes, { source: "", destination: "", readOnly: false }],
                })
              }
              addLabel={t("Add path")}
            />
            {draft.volumes.length === 0 ? (
              <EmptyLine icon={<FolderPlus className="size-3.5" />} text={t("No bind mounts configured.")} />
            ) : (
              <div className="space-y-2">
                {draft.volumes.map((volume, index) => (
                  <div key={index} className="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-2">
                    <input
                      value={volume.source}
                      onChange={(event) => updateVolume(index, { source: event.target.value })}
                      placeholder="/mnt/user/appdata/plex"
                      aria-label={t("Host path")}
                      className="h-9 min-w-0 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
                    />
                    <input
                      value={volume.destination}
                      onChange={(event) => updateVolume(index, { destination: event.target.value })}
                      placeholder="/config"
                      aria-label={t("Container path")}
                      className="h-9 min-w-0 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
                    />
                    <div className="flex items-center gap-1.5 text-[10px] text-muted">
                      <Switch
                        checked={volume.readOnly}
                        onChange={(value) => updateVolume(index, { readOnly: value })}
                        label={t("Read only")}
                      />
                      <span className="hidden w-8 sm:inline">{volume.readOnly ? "ro" : "rw"}</span>
                    </div>
                    <IconButton
                      label={t("Remove path")}
                      size="sm"
                      onClick={() => update({ volumes: draft.volumes.filter((_, i) => i !== index) })}
                    >
                      <Trash2 className="size-3.5" />
                    </IconButton>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Ports ------------------------------------------------------- */}
          <section>
            <SectionHeader
              title={t("Ports")}
              hint={t("Published host ports")}
              onAdd={() => update({ ports: [...draft.ports, { host: 0, container: 0, protocol: "tcp" }] })}
              addLabel={t("Add port")}
            />
            {draft.ports.length === 0 ? (
              <EmptyLine icon={<Plus className="size-3.5" />} text={t("No published ports.")} />
            ) : (
              <div className="space-y-2">
                {draft.ports.map((port, index) => (
                  <div key={index} className="grid grid-cols-[1fr_1fr_110px_auto] items-center gap-2">
                    <NumberField
                      label={t("Host port")}
                      value={port.host}
                      onChange={(value) =>
                        update({
                          ports: draft.ports.map((item, i) => (i === index ? { ...item, host: value } : item)),
                        })
                      }
                    />
                    <NumberField
                      label={t("Container port")}
                      value={port.container}
                      onChange={(value) =>
                        update({
                          ports: draft.ports.map((item, i) => (i === index ? { ...item, container: value } : item)),
                        })
                      }
                    />
                    <select
                      value={port.protocol}
                      aria-label={t("Protocol")}
                      onChange={(event) =>
                        update({
                          ports: draft.ports.map((item, i) =>
                            i === index ? { ...item, protocol: event.target.value } : item,
                          ),
                        })
                      }
                      className="h-9 rounded-lg border border-line bg-[var(--surface-2)] px-2 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
                    >
                      {PROTOCOLS.map((protocol) => (
                        <option key={protocol} value={protocol}>
                          {protocol}
                        </option>
                      ))}
                    </select>
                    <IconButton
                      label={t("Remove port")}
                      size="sm"
                      onClick={() => update({ ports: draft.ports.filter((_, i) => i !== index) })}
                    >
                      <Trash2 className="size-3.5" />
                    </IconButton>
                  </div>
                ))}
              </div>
            )}
          </section>

          {/* Environment ------------------------------------------------- */}
          <section>
            <SectionHeader
              title={t("Environment")}
              hint={t("Variables passed to the container")}
              onAdd={() => update({ env: [...draft.env, { key: "", value: "" }] })}
              addLabel={t("Add variable")}
            />
            {draft.env.length === 0 ? (
              <EmptyLine icon={<Plus className="size-3.5" />} text={t("No environment variables.")} />
            ) : (
              <div className="space-y-2">
                {draft.env.map((entry, index) => (
                  <div key={index} className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)_auto] items-center gap-2">
                    <input
                      value={entry.key}
                      onChange={(event) => updateEnv(index, { key: event.target.value.toUpperCase() })}
                      placeholder="PUID"
                      aria-label={t("Variable name")}
                      className="h-9 min-w-0 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] uppercase text-ink outline-none transition focus:border-[var(--accent-line)]"
                    />
                    <input
                      value={entry.value}
                      onChange={(event) => updateEnv(index, { value: event.target.value })}
                      placeholder="99"
                      aria-label={t("Variable value")}
                      className="h-9 min-w-0 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 font-mono text-[10.5px] text-ink outline-none transition focus:border-[var(--accent-line)]"
                    />
                    <IconButton
                      label={t("Remove variable")}
                      size="sm"
                      onClick={() => update({ env: draft.env.filter((_, i) => i !== index) })}
                    >
                      <Trash2 className="size-3.5" />
                    </IconButton>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section>
            <SectionHeader title={t("Restart policy")} hint={t("Docker restart behaviour")} />
            <select
              value={draft.restartPolicy}
              aria-label={t("Restart policy")}
              onChange={(event) => update({ restartPolicy: event.target.value })}
              className="h-9 w-56 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11.5px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              {RESTART_POLICIES.map((policy) => (
                <option key={policy} value={policy}>
                  {policy}
                </option>
              ))}
            </select>
          </section>

          {error ? (
            <div className="rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[11px] leading-relaxed text-[var(--danger)]">
              {error}
            </div>
          ) : null}
        </div>
      ) : null}
    </Modal>
  );
}

