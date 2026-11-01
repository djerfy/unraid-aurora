import { useEffect, useRef, useState } from "react";
import { Monitor, RefreshCw } from "lucide-react";
import { Modal } from "../ui/overlays";
import { Badge, Button } from "../ui/primitives";
import type { VM } from "../../lib/types";

type ConsoleStatus = "connecting" | "connected" | "disconnected" | "error";

export function VMConsoleModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState<ConsoleStatus>("connecting");
  const [message, setMessage] = useState("");
  const [generation, setGeneration] = useState(0);

  useEffect(() => {
    const host = hostRef.current;
    if (!vm || !host) return;

    let cancelled = false;
    let client: { disconnect: () => void; focus: () => void; scaleViewport: boolean; resizeSession: boolean } | null = null;
    setStatus("connecting");
    setMessage("");

    void (async () => {
      try {
        const { default: RFB } = await import("@novnc/novnc");
        if (cancelled) return;
        host.replaceChildren();
        const protocol = window.location.protocol === "https:" ? "wss" : "ws";
        const url = `${protocol}://${window.location.host}/api/v1/vms/${encodeURIComponent(vm.id)}/console`;
        const connection = new RFB(host, url, { shared: true });
        // Scale the framebuffer to the panel instead of resizing the guest.
        connection.scaleViewport = true;
        connection.resizeSession = false;
        connection.addEventListener("connect", () => {
          if (!cancelled) setStatus("connected");
        });
        connection.addEventListener("disconnect", (event: Event) => {
          if (cancelled) return;
          const detail = (event as CustomEvent<{ clean?: boolean }>).detail;
          setStatus("disconnected");
          setMessage(detail?.clean ? "Session closed." : "The console connection was lost.");
        });
        connection.addEventListener("securityfailure", () => {
          if (!cancelled) {
            setStatus("error");
            setMessage("The virtual machine rejected the console connection.");
          }
        });
        connection.focus();
        client = connection;
      } catch (error) {
        if (!cancelled) {
          setStatus("error");
          setMessage((error as Error).message);
        }
      }
    })();

    return () => {
      cancelled = true;
      client?.disconnect();
      host.replaceChildren();
    };
  }, [vm, generation]);

  if (!vm) return null;

  return (
    <Modal
      open
      onClose={onClose}
      title={`Console · ${vm.name}`}
      description={`${(vm.consoleProtocol || "vnc").toUpperCase()} · ${vm.os}`}
      size="xl"
      footer={
        <>
          <Button
            variant="secondary"
            icon={<RefreshCw className="size-3.5" />}
            onClick={() => setGeneration((value) => value + 1)}
          >
            Reconnect
          </Button>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="flex items-center gap-2 text-[10.5px] text-muted">
            <Monitor className="size-3.5" /> Live framebuffer over the authenticated session
          </span>
          <Badge
            tone={status === "connected" ? "success" : status === "connecting" ? "warning" : "danger"}
            dot
            pulse={status === "connecting"}
          >
            {status}
          </Badge>
        </div>
        <div
          ref={hostRef}
          className="grid h-[68vh] min-h-[420px] w-full place-items-center overflow-hidden rounded-xl border border-[#1f2230] bg-[#08090d]"
        />
        {message ? <p className="text-[10.5px] text-[var(--danger)]">{message}</p> : null}
      </div>
    </Modal>
  );
}
