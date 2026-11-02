import { useRef, useState } from "react";
import { useParams } from "react-router-dom";
import { Keyboard, Monitor, RefreshCw, X } from "lucide-react";
import { useApp } from "../app/AppDataProvider";
import { VMConsolePanel, type VMConsoleHandle, type ConsoleStatus } from "../components/vms/VMConsolePanel";
import { Badge, Button } from "../components/ui/primitives";
import { useI18n } from "../i18n";

const TONE: Record<ConsoleStatus, "success" | "warning" | "danger"> = {
  connecting: "warning",
  connected: "success",
  disconnected: "danger",
  error: "danger",
};

export function VMConsolePage() {
  const { t } = useI18n();
  const { id } = useParams();
  const { snapshot } = useApp();
  const [status, setStatus] = useState<ConsoleStatus>("connecting");
  const [token, setToken] = useState(0);
  const panelRef = useRef<VMConsoleHandle>(null);
  const vm = snapshot?.vms.find((entry) => entry.id === id) ?? null;

  return (
    <div className="flex h-dvh w-screen flex-col bg-[#08090d]">
      <header className="flex flex-wrap items-center gap-3 border-b border-[#1f2230] px-4 py-2.5">
        <Monitor className="size-4 text-[#ff8a6b]" />
        <span className="font-mono text-[11.5px] text-[#c7cbd6]">{vm?.name ?? t("Virtual machine")}</span>
        <Badge tone={TONE[status]} dot pulse={status === "connecting"}>
          {t(status)}
        </Badge>
        <div className="ml-auto flex items-center gap-2">
          <Button
            size="sm"
            variant="secondary"
            icon={<Keyboard className="size-3" />}
            onClick={() => panelRef.current?.sendCtrlAltDel()}
            className="border-[#2a2e3b] bg-[#11141c] text-[#c7cbd6] hover:border-[#3a4050] hover:bg-[#181c26]"
          >
            Ctrl+Alt+Del
          </Button>
          <Button
            size="sm"
            variant="secondary"
            icon={<RefreshCw className="size-3" />}
            onClick={() => setToken((value) => value + 1)}
            className="border-[#2a2e3b] bg-[#11141c] text-[#c7cbd6] hover:border-[#3a4050] hover:bg-[#181c26]"
          >
            {t("Reconnect")}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            icon={<X className="size-3" />}
            onClick={() => window.close()}
            className="border-[#2a2e3b] bg-[#11141c] text-[#c7cbd6] hover:border-[#3a4050] hover:bg-[#181c26]"
          >
            {t("Close window")}
          </Button>
        </div>
      </header>
      {vm && vm.consoleProtocol && !vm.consoleKeyboard ? (
        <div className="border-b border-[color-mix(in_srgb,var(--warning)_24%,transparent)] bg-[var(--warning-soft)] px-4 py-2 text-[10.5px] text-[var(--warning)]">
          {t("This virtual machine has no keyboard device defined. Add one in the VM settings.")}
        </div>
      ) : null}
      {vm ? (
        <VMConsolePanel
          ref={panelRef}
          vm={vm}
          reconnectToken={token}
          onStatus={(next) => setStatus(next)}
          className="min-h-0 flex-1 p-2"
        />
      ) : (
        <div className="grid flex-1 place-items-center px-6 text-center text-[12px] text-[#8b93a7]">
          {t("Virtual machine not found. Close this window and open the console from the VM page.")}
        </div>
      )}
    </div>
  );
}
