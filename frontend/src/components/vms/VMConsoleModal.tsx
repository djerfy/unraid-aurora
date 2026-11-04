import { useRef, useState } from "react";
import { ClipboardPaste, ExternalLink, Keyboard, RefreshCw } from "lucide-react";
import { Modal } from "../ui/overlays";
import { Badge, Button } from "../ui/primitives";
import { VMConsolePanel, type VMConsoleHandle, type ConsoleStatus } from "./VMConsolePanel";
import { useI18n } from "../../i18n";
import { useToast } from "../../app/ToastProvider";
import type { VM } from "../../lib/types";

const TONE: Record<ConsoleStatus, "success" | "warning" | "danger"> = {
  connecting: "warning",
  connected: "success",
  disconnected: "danger",
  error: "danger",
};

export function VMConsoleModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const [status, setStatus] = useState<ConsoleStatus>("connecting");
  const [token, setToken] = useState(0);
  const panelRef = useRef<VMConsoleHandle>(null);

  if (!vm) return null;

  const pasteClipboard = async () => {
    try {
      const text = await navigator.clipboard.readText();
      if (!text) return;
      if (!panelRef.current?.clipboardPaste(text)) {
        toast({ kind: "warning", title: t("Clipboard is not ready"), description: t("Reconnect the console and try again.") });
      }
    } catch (error) {
      toast({ kind: "error", title: t("Unable to read the clipboard"), description: (error as Error).message });
    }
  };

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
      title={t("Console · {name}", { name: vm.name })}
      description={`${(vm.consoleProtocol || "vnc").toUpperCase()} · ${vm.os}`}
      size="xl"
      headerExtra={
        <Badge tone={TONE[status]} dot pulse={status === "connecting"}>
          {t(status)}
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
            icon={<ClipboardPaste className="size-3.5" />}
            onClick={() => void pasteClipboard()}
          >
            {t("Paste clipboard")}
          </Button>
          <Button
            variant="secondary"
            icon={<RefreshCw className="size-3.5" />}
            onClick={() => setToken((value) => value + 1)}
          >
            {t("Reconnect")}
          </Button>
          <Button variant="secondary" icon={<ExternalLink className="size-3.5" />} onClick={openWindow}>
            {t("New window")}
          </Button>
          <Button variant="secondary" onClick={onClose}>
            {t("Close")}
          </Button>
        </>
      }
    >
      {vm.consoleProtocol && !vm.consoleKeyboard ? (
        <div className="mb-3 rounded-xl border border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] px-3 py-2.5 text-[10.5px] leading-relaxed text-[var(--warning)]">
          {t(
            "This virtual machine has no keyboard device defined. Add one (PS/2 or USB) in the VM settings, otherwise the guest will ignore every keystroke.",
          )}
        </div>
      ) : null}
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
