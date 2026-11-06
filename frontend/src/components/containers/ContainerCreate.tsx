import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Boxes, FolderPlus, Plus, RefreshCw, Rocket, Trash2 } from "lucide-react";
import { api } from "../../lib/api";
import type { ContainerCreateOptions } from "../../lib/types";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { Modal } from "../ui/overlays";
import { Button, IconButton, Switch } from "../ui/primitives";
import {
  EmptyLine,
  NumberField,
  SectionHeader,
  TextField,
  validateContainerFields,
} from "./container-form";
import { ContainerAdvancedFields, emptyContainerAdvanced, normalizeContainerAdvanced } from "./ContainerAdvancedFields";
import { FolderPicker } from "../folders/FolderPicker";
import { useVisualFolders } from "../folders/use-visual-folders";

const RESTART_POLICIES = ["unless-stopped", "always", "no", "on-failure"];
const PROTOCOLS = ["tcp", "udp", "sctp"];

const emptyOptions: ContainerCreateOptions = {
  name: "",
  image: "",
  networkMode: "bridge",
  restartPolicy: "unless-stopped",
  ports: [],
  volumes: [],
  env: [
    { key: "PUID", value: "99" },
    { key: "PGID", value: "100" },
    { key: "UMASK", value: "022" },
  ],
  icon: "",
  webUi: "",
  privileged: false,
  advanced: emptyContainerAdvanced,
};

function cloneOptions(options: ContainerCreateOptions): ContainerCreateOptions {
  return {
    ...options,
    ports: options.ports.map((port) => ({ ...port })),
    volumes: options.volumes.map((volume) => ({ ...volume })),
    env: options.env.map((entry) => ({ ...entry })),
    advanced: normalizeContainerAdvanced(options.advanced),
  };
}

export function ContainerCreateModal({ open, onClose, preset }: { open: boolean; onClose: () => void; preset?: ContainerCreateOptions | null }) {
  const queryClient = useQueryClient();
  const { t } = useI18n();
  const { toast } = useToast();
  const [draft, setDraft] = useState<ContainerCreateOptions>(emptyOptions);
  const [error, setError] = useState("");
  const [templateID, setTemplateID] = useState("");
  const [folder, setFolder] = useState("");
  const folders = useVisualFolders("containers");

  const templates = useQuery({
    queryKey: ["docker-templates"],
    queryFn: api.containerTemplates,
    enabled: open,
    staleTime: 30_000,
  });

  const networks = useQuery({
    queryKey: ["docker-networks"],
    queryFn: api.containerNetworks,
    enabled: open,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (!open) return;
    setDraft(cloneOptions(preset ?? emptyOptions));
    setTemplateID("");
    setError("");
  }, [open, preset]);

  const create = useMutation({
    mutationFn: (options: ContainerCreateOptions) => api.createContainer(options),
    onSuccess: () => {
      toast({
        kind: "success",
        title: t("Container created"),
        description: t("{name} was created and started.", { name: draft.name }),
      });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
      folders.setAssignment(draft.name.trim(), folder);
      onClose();
    },
    onError: (err: Error) => {
      setError(err.message);
      toast({ kind: "error", title: t("Creation failed"), description: err.message });
    },
  });

  const update = (patch: Partial<ContainerCreateOptions>) => {
    setDraft((current) => ({ ...current, ...patch }));
    setError("");
  };

  const applyTemplate = (id: string) => {
    setTemplateID(id);
    const template = templates.data?.find((entry) => entry.id === id);
    if (!template) return;
    setDraft({
      name: template.name,
      image: template.image,
      networkMode: template.network || "bridge",
      restartPolicy: template.restartPolicy || "unless-stopped",
      ports: template.ports ?? [],
      volumes: template.volumes ?? [],
      env: template.env?.length ? template.env : emptyOptions.env.map((entry) => ({ ...entry })),
      icon: template.icon,
      webUi: template.webUi,
      privileged: template.privileged,
      advanced: normalizeContainerAdvanced(template.advanced),
    });
    setError("");
  };

  const submit = () => {
    if (!draft.name.trim()) {
      setError(t("A container name is required."));
      return;
    }
    if (!/^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/.test(draft.name.trim())) {
      setError(t("The name can only contain letters, digits, '_', '.' and '-'."));
      return;
    }
    if (!draft.image.trim()) {
      setError(t("An image repository is required, for example linuxserver/plex:latest."));
      return;
    }
    const validation = validateContainerFields(draft, t);
    if (validation) {
      setError(validation);
      return;
    }
    create.mutate({
      ...draft,
      name: draft.name.trim(),
      image: draft.image.trim(),
      ports: draft.ports,
      volumes: draft.volumes,
      env: draft.env.filter((entry) => entry.key.trim()),
    });
  };

  const networkOptions = networks.data?.length ? networks.data : ["bridge", "host", "none"];

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t("New container")}
      description={t("Pull an image and create a container, mirroring the main options of the Unraid Add Container page.")}
      size="lg"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={create.isPending}>
            {t("Cancel")}
          </Button>
          <Button
            variant="primary"
            icon={<Rocket className="size-3.5" />}
            loading={create.isPending}
            onClick={submit}
          >
            {t("Create & start")}
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        <section>
          <FolderPicker
            folders={folders.folders}
            value={folder}
            onChange={setFolder}
            onCreate={folders.createFolder}
            label={t("Folder")}
            disabled={folders.loading}
          />
        </section>

        <section>
          <SectionHeader title={t("Template")} hint={t("Load an existing Unraid XML to recreate a container")} />
          <div className="flex items-center gap-2">
            <select
              value={templateID}
              aria-label={t("Container template")}
              onChange={(event) => applyTemplate(event.target.value)}
              className="h-9 min-w-0 flex-1 rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
            >
              <option value="">{t("Start from scratch")}</option>
              {(templates.data ?? []).map((template) => (
                <option key={template.id} value={template.id}>
                  {template.name} · {template.image}
                </option>
              ))}
            </select>
            <Button
              size="sm"
              variant="secondary"
              icon={<RefreshCw className="size-3.5" />}
              loading={templates.isFetching}
              onClick={() => void templates.refetch()}
            >
              {t("Reload")}
            </Button>
          </div>
          <p className="mt-1.5 text-[9.5px] text-subtle">
            {t("Templates are read from the flash drive; loading one prefills every field and can be used to relaunch a container after a failed creation.")}
          </p>
        </section>

        <section>
          <SectionHeader title={t("Container")} hint={t("Identity, image and network")} />
          <div className="grid gap-2 sm:grid-cols-2">
            <label className="space-y-1">
              <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Name")}</span>
              <TextField label={t("Container name")} value={draft.name} placeholder="plex" onChange={(value) => update({ name: value })} mono={false} />
            </label>
            <label className="space-y-1">
              <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Repository")}</span>
              <TextField
                label={t("Repository")}
                value={draft.image}
                placeholder="linuxserver/plex:latest"
                onChange={(value) => update({ image: value })}
              />
            </label>
            <label className="space-y-1">
              <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Network")}</span>
              <select
                value={draft.networkMode}
                aria-label={t("Network")}
                onChange={(event) => update({ networkMode: event.target.value })}
                className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
              >
                {networkOptions.map((network) => (
                  <option key={network} value={network}>
                    {network}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1">
              <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Restart policy")}</span>
              <select
                value={draft.restartPolicy}
                aria-label={t("Restart policy")}
                onChange={(event) => update({ restartPolicy: event.target.value })}
                className="h-9 w-full rounded-lg border border-line bg-[var(--surface-2)] px-2.5 text-[11px] text-soft outline-none focus:border-[var(--accent-line)]"
              >
                {RESTART_POLICIES.map((policy) => (
                  <option key={policy} value={policy}>
                    {policy}
                  </option>
                ))}
              </select>
            </label>
            <label className="space-y-1">
              <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Icon URL")}</span>
              <TextField
                label={t("Icon URL")}
                value={draft.icon}
                placeholder="https://…/icon.png"
                onChange={(value) => update({ icon: value })}
              />
            </label>
            <label className="space-y-1">
              <span className="block text-[9.5px] uppercase tracking-[0.08em] text-subtle">{t("Web UI")}</span>
              <TextField
                label={t("Web UI")}
                value={draft.webUi}
                placeholder="http://[IP]:[PORT:32400]/"
                onChange={(value) => update({ webUi: value })}
              />
            </label>
          </div>
          <label className="mt-2 flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/45 px-3 py-2.5">
            <span>
              <span className="block text-[11px] font-medium text-soft">{t("Privileged mode")}</span>
              <span className="block text-[9.5px] text-subtle">{t("Grant the container full host device access.")}</span>
            </span>
            <Switch checked={draft.privileged} onChange={(value) => update({ privileged: value })} label={t("Privileged")} />
          </label>
          {draft.image ? (
            <p className="mt-2 flex items-center gap-1.5 font-mono text-[9px] text-subtle">
              <Boxes className="size-3" /> docker pull {draft.image}
            </p>
          ) : null}
        </section>

        {/* Paths --------------------------------------------------------- */}
        <section>
          <SectionHeader
            title={t("Paths")}
            hint={t("Bind mounts, host path to container path")}
            onAdd={() => update({ volumes: [...draft.volumes, { source: "", destination: "", readOnly: false }] })}
            addLabel={t("Add path")}
          />
          {draft.volumes.length === 0 ? (
            <EmptyLine icon={<FolderPlus className="size-3.5" />} text={t("No bind mounts configured.")} />
          ) : (
            <div className="space-y-2">
              {draft.volumes.map((volume, index) => (
                <div key={index} className="grid grid-cols-[1fr_1fr_auto_auto] items-center gap-2">
                  <TextField
                    label={t("Host path")}
                    value={volume.source}
                    placeholder="/mnt/user/appdata/plex"
                    onChange={(value) =>
                      update({
                        volumes: draft.volumes.map((item, i) => (i === index ? { ...item, source: value } : item)),
                      })
                    }
                  />
                  <TextField
                    label={t("Container path")}
                    value={volume.destination}
                    placeholder="/config"
                    onChange={(value) =>
                      update({
                        volumes: draft.volumes.map((item, i) => (i === index ? { ...item, destination: value } : item)),
                      })
                    }
                  />
                  <div className="flex items-center gap-1.5 text-[10px] text-muted">
                    <Switch
                      checked={volume.readOnly}
                      onChange={(value) =>
                        update({
                          volumes: draft.volumes.map((item, i) => (i === index ? { ...item, readOnly: value } : item)),
                        })
                      }
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

        {/* Ports --------------------------------------------------------- */}
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

        {/* Environment --------------------------------------------------- */}
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
                  <TextField
                    label={t("Variable name")}
                    value={entry.key}
                    placeholder="PUID"
                    className="uppercase"
                    onChange={(value) =>
                      update({
                        env: draft.env.map((item, i) => (i === index ? { ...item, key: value.toUpperCase() } : item)),
                      })
                    }
                  />
                  <TextField
                    label={t("Variable value")}
                    value={entry.value}
                    placeholder="99"
                    onChange={(value) =>
                      update({
                        env: draft.env.map((item, i) => (i === index ? { ...item, value } : item)),
                      })
                    }
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

        <ContainerAdvancedFields
          value={draft.advanced ?? emptyContainerAdvanced}
          privileged={draft.privileged}
          onChange={(advanced) => update({ advanced })}
        />

        {error ? (
          <div className="rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[11px] leading-relaxed text-[var(--danger)]">
            {error}
          </div>
        ) : null}
      </div>
    </Modal>
  );
}
