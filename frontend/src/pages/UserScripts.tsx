import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { FileCode2, Play, RefreshCw, Terminal } from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Badge, Button, Card, EmptyState, IconButton, cn } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { api } from "../lib/api";
import { useToast } from "../app/ToastProvider";

export function UserScriptsPage() {
  const { t } = useI18n();
  const { toast } = useToast();
  const [selected, setSelected] = useState<string | null>(null);
  const [output, setOutput] = useState("");
  const scripts = useQuery({ queryKey: ["user-scripts"], queryFn: api.userScripts, refetchInterval: 30_000 });
  const script = useQuery({
    queryKey: ["user-script", selected],
    queryFn: () => api.userScript(selected ?? ""),
    enabled: Boolean(selected),
  });

  useEffect(() => {
    if (!selected && scripts.data?.length) setSelected(scripts.data[0].id);
  }, [scripts.data, selected]);

  const run = useMutation({
    mutationFn: () => api.runUserScript(selected ?? ""),
    onSuccess: (result) => {
      setOutput(result.output || result.error || "");
      if (result.error) toast({ kind: "warning", title: t("Script finished with an error"), description: result.error });
      else toast({ kind: "success", title: t("Script completed") });
    },
    onError: (error: Error) => {
      setOutput(error.message);
      toast({ kind: "error", title: t("Could not run the script"), description: error.message });
    },
  });

  const active = scripts.data?.find((item) => item.id === selected) ?? null;

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("User scripts")}
        description={t("Run the scripts stored by the Unraid User Scripts plugin and inspect their output.")}
        meta={
          <>
            <Badge tone="neutral" dot>{t("{count} scripts", { count: scripts.data?.length ?? 0 })}</Badge>
            <span className="chip">{t("Root execution on the host")}</span>
          </>
        }
        actions={
          <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={() => void scripts.refetch()}>
            {t("Refresh")}
          </Button>
        }
      />

      {scripts.isLoading ? (
        <p className="text-[11px] text-muted">{t("Loading scripts…")}</p>
      ) : scripts.isError ? (
        <Card><EmptyState icon={<FileCode2 className="size-5" />} title={t("User scripts are unavailable")} description={(scripts.error as Error).message} /></Card>
      ) : !scripts.data?.length ? (
        <Card><EmptyState icon={<FileCode2 className="size-5" />} title={t("No user scripts")} description={t("Create scripts in the Unraid User Scripts plugin to see them here.")} /></Card>
      ) : (
        <div className="grid gap-4 xl:grid-cols-[300px_minmax(0,1fr)]">
          <Card className="card-pad">
            <h2 className="text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Scripts")}</h2>
            <div className="mt-2 space-y-1">
              {scripts.data.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => { setSelected(item.id); setOutput(""); }}
                  className={cn(
                    "w-full rounded-lg border px-2.5 py-2 text-left transition",
                    selected === item.id ? "border-[var(--accent-line)] bg-[var(--accent-soft)]" : "border-transparent hover:bg-[var(--surface-2)]",
                  )}
                >
                  <span className="block truncate text-[11.5px] font-medium text-ink">{item.name}</span>
                  <span className="mt-0.5 block truncate text-[9px] text-subtle">{item.schedule || t("Manual")}</span>
                </button>
              ))}
            </div>
          </Card>

          <Card className="card-pad">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <h2 className="truncate text-[13px] font-semibold text-ink">{active?.name ?? t("Script")}</h2>
                <p className="mt-1 text-[10.5px] text-subtle">{active?.description || t("No description.")}</p>
              </div>
              <Button
                size="sm"
                variant="primary"
                icon={<Play className="size-3.5" />}
                loading={run.isPending}
                disabled={!selected}
                onClick={() => run.mutate()}
              >
                {t("Run script")}
              </Button>
            </div>
            <div className="mt-4">
              <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
                <FileCode2 className="size-3.5" /> {t("Script source")}
              </h3>
              <pre className="mt-2 max-h-[38vh] overflow-auto rounded-xl border border-[#1f2230] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed text-[#c7cbd6]">
                {script.isLoading ? t("Loading…") : script.data?.content ?? "—"}
              </pre>
            </div>
            <div className="mt-4">
              <div className="flex items-center justify-between">
                <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
                  <Terminal className="size-3.5" /> {t("Output")}
                </h3>
                {output ? <IconButton label={t("Clear output")} size="sm" onClick={() => setOutput("")}><RefreshCw className="size-3.5" /></IconButton> : null}
              </div>
              <pre className="mt-2 h-48 overflow-auto rounded-xl border border-[#1f2230] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed text-[#c7cbd6]">
                {output || t("Run the script to see its output.")}
              </pre>
            </div>
          </Card>
        </div>
      )}
    </>
  );
}
