import { useRef, useState } from "react";
import { ExternalLink, Keyboard, RefreshCw } from "lucide-react";
import { Modal } from "../ui/overlays";
import { Badge, Button } from "../ui/primitives";
import { VMConsolePanel, type VMConsoleHandle, type ConsoleStatus } from "./VMConsolePanel";
import type { VM } from "../../lib/types";

const TONE: Record<ConsoleStatus, "success" | "warning" | "danger"> = {
  connecting: "warning",
  connected: "success",
  disconnected: "danger",
  error: "danger",
};

export function VMConsoleModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const [status, setStatus] = useState<ConsoleStatus>("connecting");
  const [token, setToken] = useState(0);
  const panelRef = useRef<VMConsoleHandle>(null);

  if (!vm) return null;

  const openWindow = () => {
    window.open(
      `${window.location.origin}/vms/console/${encodeURIComponent(vm.id)}`,
      `aurora-vm-console-${vm.id}`,
      "popup=yes,width=1280,height=900",
    );
  };

  return (
    <Modal
      open
      onClose={onClose}
      title={`Console · ${vm.name}`}
      description={`${(vm.consoleProtocol || "vnc").toUpperCase()} · ${vm.os}`}
      size="xl"
      headerExtra={
        <Badge tone={TONE[status]} dot pulse={status === "connecting"}>
          {status}
        </Badge>
      }
      footer={
        <>
          <Button
            variant="secondary"
            icon={<Keyboard className="size-3.5" />}
            onClick={() => panelRef.current?.sendCtrlAltDel()}
          >
            Ctrl+Alt+Del
          </Button>
          <Button
            variant="secondary"
            icon={<RefreshCw className="size-3.5" />}
            onClick={() => setToken((value) => value + 1)}
          >
            Reconnect
          </Button>
          <Button variant="secondary" icon={<ExternalLink className="size-3.5" />} onClick={openWindow}>
            New window
          </Button>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
        </>
      }
    >
      <VMConsolePanel
        ref={panelRef}
        vm={vm}
        reconnectToken={token}
        onStatus={(next) => setStatus(next)}
        className="h-[68vh] w-full"
      />
    </Modal>
  );
}
