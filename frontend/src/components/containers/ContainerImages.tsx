import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Layers, RefreshCw, Trash2, TriangleAlert } from "lucide-react";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import { Badge, Button, Card, EmptyState, IconButton, cn } from "../ui/primitives";
import { formatBytes, relativeTime } from "../../lib/format";
import type { ContainerImage } from "../../lib/types";

function imageName(image: ContainerImage): string {
  if (image.repoTags.length > 0) return image.repoTags[0];
  return image.id;
}

function shortID(id: string): string {
  return id.length > 19 ? `${id.slice(0, 19)}…` : id;
}

export function ContainerImages() {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["container-images"],
    queryFn: api.containerImages,
    refetchInterval: 30_000,
  });
  const images = query.data ?? [];
  const unused = images.filter((image) => !image.inUse);
  const unusedSize = unused.reduce((sum, image) => sum + image.size, 0);
  const totalSize = images.reduce((sum, image) => sum + image.size, 0);

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ["container-images"] });
  };

  const remove = useMutation({
    mutationFn: (id: string) => api.removeContainerImage(id),
    onSuccess: () => {
      toast({ kind: "success", title: t("Image removed") });
      invalidate();
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Image removal failed"), description: error.message });
    },
  });

  const prune = useMutation({
    mutationFn: api.removeUnusedContainerImages,
    onSuccess: (result) => {
      toast({
        kind: result.failed.length ? "warning" : "success",
        title: t("Unused images removed"),
        description: t("{removed} removed · {skipped} still in use · {failed} failed", {
          removed: result.removed.length,
          skipped: result.skipped.length,
          failed: result.failed.length,
        }),
      });
      invalidate();
    },
    onError: (error: Error) => {
      toast({ kind: "error", title: t("Image removal failed"), description: error.message });
    },
  });

  const confirmRemove = (image: ContainerImage) => {
    if (image.inUse) return;
    if (window.confirm(t("Remove image {name}?", { name: imageName(image) }))) {
      remove.mutate(image.id);
    }
  };

  const confirmPrune = () => {
    if (!unused.length) return;
    if (window.confirm(t("Remove {count} unused images?", { count: unused.length }))) {
      prune.mutate();
    }
  };

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-line px-4 py-3.5">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink">
            <Layers className="size-4 text-[var(--accent)]" /> {t("Images")}
          </h2>
          <p className="mt-1 max-w-2xl text-[10.5px] leading-relaxed text-muted">
            {t("Docker images on the host. Images used by a container are protected from deletion.")}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="neutral">{t("{count} images", { count: images.length })}</Badge>
          <Badge tone={unused.length ? "warning" : "success"}>
            {t("{count} unused", { count: unused.length })}
          </Badge>
          <Button
            size="sm"
            variant="secondary"
            icon={<RefreshCw className={cn("size-3.5", query.isFetching && "animate-spin")} />}
            loading={query.isFetching}
            onClick={() => void query.refetch()}
          >
            {t("Refresh")}
          </Button>
          <Button
            size="sm"
            variant="danger"
            icon={<Trash2 className="size-3.5" />}
            disabled={unused.length === 0 || prune.isPending}
            loading={prune.isPending}
            onClick={confirmPrune}
          >
            {t("Remove unused")}
          </Button>
        </div>
      </div>

      {query.isLoading ? (
        <p className="py-12 text-center text-[10.5px] text-muted">{t("Loading images…")}</p>
      ) : query.isError ? (
        <div className="m-4 flex items-start gap-2 rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] text-[var(--danger)]">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
          <span>{query.error.message}</span>
        </div>
      ) : images.length === 0 ? (
        <EmptyState
          icon={<Layers className="size-5" />}
          title={t("No images.")}
          description={t("Aurora has not found any Docker image on this host.")}
        />
      ) : (
        <>
          <div className="grid grid-cols-3 divide-x divide-line border-b border-line bg-[var(--surface-2)]/35">
            <div className="px-4 py-3">
              <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{t("Total size")}</p>
              <p className="num mt-1 text-[13px] font-semibold text-ink">{formatBytes(totalSize)}</p>
            </div>
            <div className="px-4 py-3">
              <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{t("Reclaimable")}</p>
              <p className="num mt-1 text-[13px] font-semibold text-ink">{formatBytes(unusedSize)}</p>
            </div>
            <div className="px-4 py-3">
              <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{t("In use")}</p>
              <p className="num mt-1 text-[13px] font-semibold text-ink">
                {t("{count} images", { count: images.length - unused.length })}
              </p>
            </div>
          </div>

          <div className="hidden items-center gap-3 border-b border-line px-4 py-2 text-[8.5px] font-semibold uppercase tracking-[0.12em] text-subtle md:flex">
            <span className="min-w-0 flex-1">{t("Image")}</span>
            <span className="w-24 shrink-0">{t("Size")}</span>
            <span className="w-24 shrink-0">{t("Created")}</span>
            <span className="w-[240px] shrink-0">{t("Used by")}</span>
            <span className="w-10 shrink-0 text-right">{t("Actions")}</span>
          </div>

          <div className="divide-y divide-line/60">
            {images.map((image) => {
              const tags = image.repoTags.length ? image.repoTags : [];
              const busy = remove.isPending && remove.variables === image.id;
              return (
                <div key={image.id} className="flex flex-wrap items-center gap-3 px-4 py-3 transition hover:bg-[var(--surface-2)]/40">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-1.5">
                      {tags.length ? tags.slice(0, 3).map((tag) => (
                        <span key={tag} className="chip max-w-full truncate font-mono text-[9.5px]">{tag}</span>
                      )) : null}
                      {image.dangling ? <Badge tone="warning">{t("Dangling")}</Badge> : null}
                    </div>
                    <p className="mt-1 truncate font-mono text-[9px] text-subtle" title={image.id}>{shortID(image.id)}</p>
                  </div>
                  <span className="w-24 shrink-0 font-mono text-[10px] text-muted">{formatBytes(image.size)}</span>
                  <span className="w-24 shrink-0 text-[10px] text-subtle">{relativeTime(image.created)}</span>
                  <span className="flex w-[240px] shrink-0 flex-wrap items-center gap-1">
                    {image.inUse ? (
                      <>
                        {image.usedBy.slice(0, 3).map((name) => (
                          <span key={name} className="chip max-w-[110px] truncate text-[9px]">{name}</span>
                        ))}
                        {image.usedBy.length > 3 ? <span className="font-mono text-[9px] text-subtle">+{image.usedBy.length - 3}</span> : null}
                      </>
                    ) : (
                      <Badge tone="success">{t("Unused")}</Badge>
                    )}
                  </span>
                  <span className="w-10 shrink-0 text-right">
                    <IconButton
                      label={image.inUse ? t("This image is used by a container.") : t("Remove image")}
                      size="sm"
                      disabled={image.inUse || busy}
                      onClick={() => confirmRemove(image)}
                    >
                      <Trash2 className="size-3.5" />
                    </IconButton>
                  </span>
                </div>
              );
            })}
          </div>
        </>
      )}
    </Card>
  );
}
