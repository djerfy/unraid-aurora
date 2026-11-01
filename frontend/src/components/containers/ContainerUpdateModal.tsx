import { Check, Loader2, RefreshCw, TriangleAlert, X } from "lucide-react";
import type { ContainerUpdateEvent } from "../../lib/types";
import { Modal } from "../ui/overlays";
import { Badge, Button, Progress } from "../ui/primitives";

export type ContainerUpdateRun = {
  names: string[];
  index: number;
  events: ContainerUpdateEvent[];
  running: boolean;
  error: string;
  done: boolean;
};

const STEP_LABELS: Record<string, string> = {
  pull: "Pull image",
  stop: "Stop container",
  create: "Recreate container",
  start: "Start container",
  cleanup: "Remove previous container",
  done: "Complete",
};

export function ContainerUpdateModal({
  run,
  onClose,
}: {
  run: ContainerUpdateRun | null;
  onClose: () => void;
}) {
  if (!run) return null;
  const target = run.names[run.index] ?? "container";
  const last = run.events[run.events.length - 1];
  const steps = orderedSteps(run.events);
  const percent = progressOf(run.events);

  return (
    <Modal
      open
      onClose={run.running ? () => {} : onClose}
      title={run.done ? "Update complete" : `Updating ${target}`}
      description={
        run.names.length > 1
          ? `Container ${run.index + 1} of ${run.names.length} · pull and recreate, like the Unraid webGUI.`
          : "Pull the image and recreate the container with its current settings."
      }
      footer={
        <Button variant="secondary" onClick={onClose} disabled={run.running} icon={<X className="size-3.5" />}>
          Close
        </Button>
      }
    >
      <div className="space-y-4">
        <div className="flex items-center justify-between gap-3">
          <span className="min-w-0 truncate font-mono text-[11px] text-soft">{target}</span>
          <span className="flex items-center gap-2">
            {run.done ? <Badge tone="success" dot>success</Badge> : null}
            {run.error ? <Badge tone="danger" dot>failed</Badge> : null}
            {run.running ? (
              <Badge tone="accent" dot pulse>
                running
              </Badge>
            ) : null}
          </span>
        </div>

        <div>
          <Progress value={percent} height={6} tone={run.error ? "var(--danger)" : undefined} />
          <div className="mt-1.5 flex items-center justify-between text-[9.5px] text-subtle">
            <span className="min-w-0 truncate font-mono">{last?.message ?? "Starting…"}</span>
            <span className="num shrink-0">{Math.round(percent)}%</span>
          </div>
        </div>

        <div className="space-y-1.5">
          {steps.length === 0 ? (
            <div className="flex items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)]/45 px-3 py-2 text-[10.5px] text-muted">
              <Loader2 className="size-3.5 animate-spin" /> Preparing the update…
            </div>
          ) : (
            steps.map((step) => {
              const active = run.running && step.step === last?.step && !run.done;
              const failed = step.step === "error";
              return (
                <div
                  key={step.step}
                  className="flex items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)]/45 px-3 py-2 text-[10.5px]"
                >
                  {failed ? (
                    <TriangleAlert className="size-3.5 shrink-0 text-[var(--danger)]" />
                  ) : active ? (
                    <Loader2 className="size-3.5 shrink-0 animate-spin text-[var(--accent)]" />
                  ) : (
                    <Check className="size-3.5 shrink-0 text-[var(--success)]" />
                  )}
                  <span className={failed ? "text-[var(--danger)]" : "text-soft"}>
                    {STEP_LABELS[step.step] ?? step.step}
                  </span>
                  {step.step === "pull" && step.progress > 0 ? (
                    <span className="ml-auto num text-[9.5px] text-subtle">{Math.round(step.progress)}%</span>
                  ) : null}
                </div>
              );
            })
          )}
        </div>

        {run.error ? (
          <div className="rounded-xl border border-[color-mix(in_srgb,var(--danger)_24%,transparent)] bg-[var(--danger-soft)] px-3 py-2.5 text-[10.5px] leading-relaxed text-[var(--danger)]">
            {run.error}
          </div>
        ) : null}

        {run.running ? (
          <p className="flex items-center gap-1.5 text-[9.5px] text-subtle">
            <RefreshCw className="size-3" /> Keep this window open; the server streams each stage.
          </p>
        ) : null}
      </div>
    </Modal>
  );
}

function orderedSteps(events: ContainerUpdateEvent[]) {
  const byStep = new Map<string, ContainerUpdateEvent>();
  for (const event of events) {
    if (event.error) {
      byStep.set("error", event);
      continue;
    }
    if (!byStep.has(event.step)) byStep.set(event.step, event);
    else byStep.set(event.step, { ...byStep.get(event.step)!, progress: Math.max(byStep.get(event.step)!.progress, event.progress) });
  }
  return Array.from(byStep.values());
}

function progressOf(events: ContainerUpdateEvent[]): number {
  const last = events[events.length - 1];
  if (!last) return 0;
  if (last.error) return 100;
  switch (last.step) {
    case "pull":
      return Math.min(30, last.progress * 0.3);
    case "stop":
      return 45;
    case "create":
      return 68;
    case "start":
      return 86;
    case "cleanup":
      return 96;
    case "done":
      return 100;
    default:
      return Math.max(0, Math.min(100, last.progress));
  }
}
